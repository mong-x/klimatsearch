-- +goose Up

ALTER TABLE resources ADD COLUMN details_json TEXT NOT NULL DEFAULT '{}';

UPDATE meta SET value = '4' WHERE key = 'schema_version';

-- +goose Down

UPDATE meta SET value = '3' WHERE key = 'schema_version';
