package broadcaster

import (
	"sync"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
)

// bufferSize bounds each subscriber queue. Slow subscribers are
// dropped on Publish rather than blocking the publisher.
const bufferSize = 256

// inMem is the in-process Broadcaster. It is also embedded into the
// Postgres impl as the local fan-out path, so it doubles as both a
// standalone backend (NewInMem) and a building block.
type inMem struct {
	mu        sync.Mutex
	subs      map[string]map[int]chan *nsv1.StreamEvent
	nextSubID int
	closed    bool
}

// NewInMem returns an in-process Broadcaster. Single-pod only — two
// instances do not share state. The Postgres backend (NewPostgres)
// embeds an inMem internally and adds LISTEN/NOTIFY for cross-pod
// fan-out.
func NewInMem() Broadcaster {
	return newInMem()
}

func newInMem() *inMem {
	return &inMem{subs: map[string]map[int]chan *nsv1.StreamEvent{}}
}

func (b *inMem) Subscribe(runID string) (<-chan *nsv1.StreamEvent, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[runID] == nil {
		b.subs[runID] = map[int]chan *nsv1.StreamEvent{}
	}
	id := b.nextSubID
	b.nextSubID++
	ch := make(chan *nsv1.StreamEvent, bufferSize)
	b.subs[runID][id] = ch
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		m, ok := b.subs[runID]
		if !ok {
			return
		}
		c, ok := m[id]
		if !ok {
			return
		}
		delete(m, id)
		close(c)
		if len(m) == 0 {
			delete(b.subs, runID)
		}
	}
}

func (b *inMem) Publish(runID string, ev *nsv1.StreamEvent) {
	b.mu.Lock()
	subs := make([]chan *nsv1.StreamEvent, 0, len(b.subs[runID]))
	for _, c := range b.subs[runID] {
		subs = append(subs, c)
	}
	b.mu.Unlock()
	for _, c := range subs {
		select {
		case c <- ev:
		default:
			// Slow subscriber; drop. The subscriber recovers via run
			// history (records.RecordStore), so dropping a live event
			// is acceptable.
		}
	}
}

func (b *inMem) CloseRun(runID string) {
	b.mu.Lock()
	m, ok := b.subs[runID]
	if !ok {
		b.mu.Unlock()
		return
	}
	delete(b.subs, runID)
	b.mu.Unlock()
	for _, c := range m {
		close(c)
	}
}

// Close drains every per-run channel still open. Idempotent.
func (b *inMem) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	all := b.subs
	b.subs = map[string]map[int]chan *nsv1.StreamEvent{}
	b.mu.Unlock()
	for _, m := range all {
		for _, c := range m {
			close(c)
		}
	}
	return nil
}
