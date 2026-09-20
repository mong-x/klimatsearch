package ingest

import (
	"context"
	"encoding/json"
)

// EventSink is implemented by store.Store (no ingest import).
type EventSink interface {
	AppendIngestEvent(ctx context.Context, event, catalog, errText, body string) error
}

type sinkLog struct{ EventSink }

// LogTo persists Events through a Store.
func LogTo(s EventSink) EventLog {
	if s == nil {
		return nil
	}
	return sinkLog{s}
}

func (s sinkLog) AppendEvent(ctx context.Context, ev Event) error {
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return s.AppendIngestEvent(ctx, ev.Event, ev.Catalog, ev.Error, string(body))
}
