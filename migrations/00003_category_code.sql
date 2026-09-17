-- +goose Up

ALTER TABLE resources ADD COLUMN category_code TEXT NOT NULL DEFAULT '';

UPDATE meta SET value = '3' WHERE key = 'schema_version';

-- +goose Down

-- SQLite cannot DROP COLUMN on all builds; leave the column on down.
UPDATE meta SET value = '2' WHERE key = 'schema_version';
