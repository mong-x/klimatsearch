package store

import (
	"context"
	"fmt"
	"time"
)

const maxIngestEvents = 200

// IngestEvent is one persisted ingest webhook (success or failure).
type IngestEvent struct {
	ID      int64
	At      time.Time
	Event   string
	Catalog string
	Error   string
	Body    string
}

// AppendIngestEvent writes one row and drops the oldest past maxIngestEvents.
func (s *Store) AppendIngestEvent(ctx context.Context, event, catalog, errText, body string) error {
	if event == "" {
		return fmt.Errorf("event name is required")
	}
	at := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO ingest_events (at, event, catalog, error, body) VALUES (?, ?, ?, ?, ?)`,
		at, event, catalog, errText, body,
	); err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM ingest_events WHERE id NOT IN (SELECT id FROM ingest_events ORDER BY id DESC LIMIT ?)`,
		maxIngestEvents,
	); err != nil {
		return fmt.Errorf("prune events: %w", err)
	}
	return nil
}

// ListIngestEvents returns newest first.
func (s *Store) ListIngestEvents(ctx context.Context, limit int) ([]IngestEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > maxIngestEvents {
		limit = maxIngestEvents
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, at, event, catalog, error, body FROM ingest_events ORDER BY id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	var out []IngestEvent
	for rows.Next() {
		var e IngestEvent
		var at string
		if err := rows.Scan(&e.ID, &at, &e.Event, &e.Catalog, &e.Error, &e.Body); err != nil {
			return nil, err
		}
		e.At, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, e)
	}
	return out, rows.Err()
}
