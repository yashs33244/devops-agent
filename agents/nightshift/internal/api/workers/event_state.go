package workers

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/records"
)

// eventKey produces the (collection, key) at which a StreamEvent for
// runID + index is persisted. The zero-padded index gives lexicographic
// ordering that matches index ordering.
func eventKey(runID string, index int64) string {
	return fmt.Sprintf("%s:%020d", runID, index)
}

func eventToRecord(runID string, ev *nsv1.StreamEvent) (records.Record, error) {
	data, err := proto.Marshal(ev)
	if err != nil {
		return records.Record{}, fmt.Errorf("marshal event: %w", err)
	}
	return records.Record{
		Collection:  eventsCollection,
		Key:         eventKey(runID, ev.GetIndex()),
		Data:        data,
		ContentType: recordContentType,
		Attributes:  map[string]string{attrRunID: runID},
	}, nil
}

func recordToEvent(rec records.Record) (*nsv1.StreamEvent, error) {
	ev := &nsv1.StreamEvent{}
	if err := proto.Unmarshal(rec.Data, ev); err != nil {
		return nil, fmt.Errorf("unmarshal event: %w", err)
	}
	return ev, nil
}
