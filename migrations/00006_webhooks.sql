-- +goose Up

CREATE TABLE webhooks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  url TEXT NOT NULL,
  secret TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 1,
  events TEXT NOT NULL DEFAULT '',
  created TEXT NOT NULL
);

UPDATE meta SET value = '6' WHERE key = 'schema_version';

-- +goose Down

DROP TABLE webhooks;
UPDATE meta SET value = '5' WHERE key = 'schema_version';
