package broadcaster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
)

// notifyChannel is the Postgres LISTEN/NOTIFY channel name. Global
// across the cluster — single channel for every run is fine because
// the JSON payload carries the runID and subscribers only fan out
// matches into their local subscriber map.
const notifyChannel = "ns_run_events"

// notifyPayload is what every Publish/CloseRun writes into the
// channel. Kept tiny so it stays well below Postgres' 8000-byte
// NOTIFY ceiling. Subscribers re-fetch the full StreamEvent via
// EventFetcher — the index field carries enough to do that.
type notifyPayload struct {
	Pod   string `json:"pod"`
	Run   string `json:"run"`
	Kind  string `json:"kind"` // "event" | "close"
	Index int64  `json:"index,omitempty"`
}

// Postgres is the LISTEN/NOTIFY-backed Broadcaster used when more
// than one nightshift-api replica shares a Postgres records store.
// Local subscribers see events at memory speed via the embedded inMem
// fan-out; cross-pod fan-out is layered on top via NOTIFY +
// pg_notify.
type Postgres struct {
	pool       *pgxpool.Pool
	listenConn *pgx.Conn
	podID      string
	fetcher    EventFetcher
	local      *inMem
	logger     *slog.Logger

	// Listener-loop lifecycle.
	listenCtx    context.Context
	cancelListen context.CancelFunc
	listenDone   chan struct{}

	closeOnce sync.Once
}

// NewPostgres opens a pgxpool against dsn, dedicates one conn to
// LISTEN ns_run_events, and starts a goroutine that fans out
// remote NOTIFYs into a per-process inMem broadcaster.
//
// podID identifies this instance for self-echo suppression — every
// outgoing NOTIFY carries it in the payload, and the listener skips
// notifications it originated.
//
// fetcher resolves NOTIFY references back to full StreamEvents
// (NOTIFY payloads carry only {pod, run, kind, index}; the full
// event is read from the records store on receive).
func NewPostgres(ctx context.Context, dsn, podID string, fetcher EventFetcher) (*Postgres, error) {
	if podID == "" {
		return nil, errors.New("broadcaster: podID required")
	}
	if fetcher == nil {
		return nil, errors.New("broadcaster: EventFetcher required")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("broadcaster: pgxpool: %w", err)
	}
	listenConn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("broadcaster: listen conn: %w", err)
	}
	if _, err := listenConn.Exec(ctx, "LISTEN "+notifyChannel); err != nil {
		_ = listenConn.Close(ctx)
		pool.Close()
		return nil, fmt.Errorf("broadcaster: LISTEN: %w", err)
	}

	listenCtx, cancel := context.WithCancel(context.Background())
	p := &Postgres{
		pool:         pool,
		listenConn:   listenConn,
		podID:        podID,
		fetcher:      fetcher,
		local:        newInMem(),
		logger:       slog.Default().With("component", "broadcaster.postgres", "pod", podID),
		listenCtx:    listenCtx,
		cancelListen: cancel,
		listenDone:   make(chan struct{}),
	}
	go p.listenLoop()
	return p, nil
}

// Subscribe delegates to the local inMem fan-out. Whether the events
// originated locally (Publish on this instance) or remotely (NOTIFY
// from another instance), the listener funnels them through the same
// inMem.Publish path — so subscribers don't need to care which side
// of the wire an event came from.
func (p *Postgres) Subscribe(runID string) (<-chan *nsv1.StreamEvent, func()) {
	return p.local.Subscribe(runID)
}

// Publish does local fan-out first (so this pod's subscribers see
// the event immediately) and then NOTIFYs the channel for cross-pod
// delivery. The local listener filters out the self-echo via podID.
func (p *Postgres) Publish(runID string, ev *nsv1.StreamEvent) {
	p.local.Publish(runID, ev)
	p.notify(notifyPayload{
		Pod:   p.podID,
		Run:   runID,
		Kind:  "event",
		Index: ev.GetIndex(),
	})
}

// CloseRun does local fan-out close first, then NOTIFYs other pods
// to do the same.
func (p *Postgres) CloseRun(runID string) {
	p.local.CloseRun(runID)
	p.notify(notifyPayload{
		Pod:  p.podID,
		Run:  runID,
		Kind: "close",
	})
}

// Close cancels the listener loop, drains the dedicated LISTEN conn,
// closes the pool, and closes any per-run channels still open.
// Idempotent.
func (p *Postgres) Close() error {
	var firstErr error
	p.closeOnce.Do(func() {
		p.cancelListen()
		// listenLoop exits when WaitForNotification observes ctx
		// cancellation; close the conn first to unblock it
		// immediately.
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := p.listenConn.Close(closeCtx); err != nil && firstErr == nil {
			firstErr = err
		}
		select {
		case <-p.listenDone:
		case <-time.After(2 * time.Second):
			// Listener didn't exit promptly; proceed anyway.
		}
		p.pool.Close()
		if err := p.local.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	})
	return firstErr
}

// notify writes payload into the LISTEN channel. Best-effort: a
// failure to NOTIFY does not surface to the caller — local
// subscribers already saw the event, and remote subscribers will
// recover via run-history reads against records.
func (p *Postgres) notify(payload notifyPayload) {
	body, err := json.Marshal(payload)
	if err != nil {
		p.logger.Warn("notify marshal", "err", err, "payload", payload)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := p.pool.Exec(ctx, "SELECT pg_notify($1, $2)", notifyChannel, string(body)); err != nil {
		p.logger.Warn("notify exec", "err", err, "payload", payload)
	}
}

// listenLoop blocks on the dedicated conn waiting for notifications
// and feeds them into local fan-out. Self-echoes (pod == p.podID)
// are skipped — those events were already delivered locally during
// Publish/CloseRun.
//
// On a transient conn error (e.g. Postgres restart), the loop tries
// to reconnect with bounded backoff and re-LISTEN. The compliance
// suite asserts only the happy path; reconnect is verified manually
// in kind end-to-end (Step 12 in the plan).
func (p *Postgres) listenLoop() {
	defer close(p.listenDone)

	for {
		select {
		case <-p.listenCtx.Done():
			return
		default:
		}

		n, err := p.listenConn.WaitForNotification(p.listenCtx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			// During Close() we shut listenConn down before
			// listenCtx is observed; pgx returns
			// "use of closed network connection". Don't log noise.
			if p.listenCtx.Err() != nil {
				return
			}
			p.logger.Warn("listen wait", "err", err)
			if !p.reconnect() {
				return
			}
			continue
		}

		var pl notifyPayload
		if err := json.Unmarshal([]byte(n.Payload), &pl); err != nil {
			p.logger.Warn("notify decode", "err", err, "raw", n.Payload)
			continue
		}
		if pl.Pod == p.podID {
			// Self-echo — already delivered locally.
			continue
		}

		switch pl.Kind {
		case "event":
			ev, err := p.fetcher.Fetch(p.listenCtx, pl.Run, pl.Index)
			if err != nil {
				if errors.Is(err, ErrUnknownRun) {
					p.logger.Debug("fetch miss", "run", pl.Run, "index", pl.Index)
				} else {
					p.logger.Warn("fetch event", "err", err, "run", pl.Run, "index", pl.Index)
				}
				continue
			}
			p.local.Publish(pl.Run, ev)
		case "close":
			p.local.CloseRun(pl.Run)
		default:
			p.logger.Warn("unknown notify kind", "kind", pl.Kind)
		}
	}
}

// reconnect re-opens the dedicated listen conn and re-issues LISTEN
// after a transient failure. Returns false if the listener context
// is cancelled (shutdown) or the pool is gone — caller exits the
// loop in that case.
func (p *Postgres) reconnect() bool {
	const maxBackoff = 5 * time.Second
	backoff := 100 * time.Millisecond
	for {
		select {
		case <-p.listenCtx.Done():
			return false
		default:
		}

		// Acquire a fresh listen conn from the pool's underlying
		// config. We can't hijack one from the pool (that conn would
		// then be unavailable for queries forever), so we open a new
		// dedicated conn using the same config.
		conf := p.pool.Config().ConnConfig.Copy()
		conn, err := pgx.ConnectConfig(p.listenCtx, conf)
		if err != nil {
			p.logger.Warn("reconnect dial", "err", err)
			select {
			case <-p.listenCtx.Done():
				return false
			case <-time.After(backoff):
			}
			if backoff < maxBackoff {
				backoff *= 2
			}
			continue
		}
		if _, err := conn.Exec(p.listenCtx, "LISTEN "+notifyChannel); err != nil {
			_ = conn.Close(p.listenCtx)
			p.logger.Warn("reconnect LISTEN", "err", err)
			select {
			case <-p.listenCtx.Done():
				return false
			case <-time.After(backoff):
			}
			if backoff < maxBackoff {
				backoff *= 2
			}
			continue
		}
		p.listenConn = conn
		p.logger.Info("listen reconnected")
		return true
	}
}
