-- +goose Up

CREATE TABLE ingest_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  at TEXT NOT NULL,
  event TEXT NOT NULL,
  catalog TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX ingest_events_id ON ingest_events(id DESC);

UPDATE meta SET value = '5' WHERE key = 'schema_version';

-- +goose Down

DROP TABLE ingest_events;
UPDATE meta SET value = '4' WHERE key = 'schema_version';
