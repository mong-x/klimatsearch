-- +goose Up

CREATE TABLE resources (
    id TEXT PRIMARY KEY,
    name_sv TEXT NOT NULL DEFAULT '',
    name_en TEXT NOT NULL DEFAULT '',
    description_sv TEXT NOT NULL DEFAULT '',
    description_en TEXT NOT NULL DEFAULT '',
    a1_a3 REAL NOT NULL DEFAULT 0,
    unit TEXT NOT NULL DEFAULT '',
    conversions_json TEXT NOT NULL DEFAULT '{}',
    category TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    raw_json TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE VIRTUAL TABLE resources_fts USING fts5(
    name_sv,
    name_en,
    description_sv,
    description_en,
    content='resources',
    content_rowid='rowid'
);

-- +goose StatementBegin
CREATE TRIGGER resources_ai AFTER INSERT ON resources BEGIN
    INSERT INTO resources_fts(rowid, name_sv, name_en, description_sv, description_en)
    VALUES (new.rowid, new.name_sv, new.name_en, new.description_sv, new.description_en);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER resources_ad AFTER DELETE ON resources BEGIN
    INSERT INTO resources_fts(resources_fts, rowid, name_sv, name_en, description_sv, description_en)
    VALUES ('delete', old.rowid, old.name_sv, old.name_en, old.description_sv, old.description_en);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER resources_au AFTER UPDATE ON resources BEGIN
    INSERT INTO resources_fts(resources_fts, rowid, name_sv, name_en, description_sv, description_en)
    VALUES ('delete', old.rowid, old.name_sv, old.name_en, old.description_sv, old.description_en);
    INSERT INTO resources_fts(rowid, name_sv, name_en, description_sv, description_en)
    VALUES (new.rowid, new.name_sv, new.name_en, new.description_sv, new.description_en);
END;
-- +goose StatementEnd

CREATE VIRTUAL TABLE resource_vec USING vec0(
    resource_id TEXT PRIMARY KEY,
    embedding float[320] distance_metric=cosine
);

CREATE TABLE meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO meta(key, value) VALUES
    ('schema_version', '1'),
    ('embedding_dim', '320'),
    ('dataset_version', '');

-- +goose Down

DROP TABLE IF EXISTS resource_vec;
DROP TRIGGER IF EXISTS resources_au;
DROP TRIGGER IF EXISTS resources_ad;
DROP TRIGGER IF EXISTS resources_ai;
DROP TABLE IF EXISTS resources_fts;
DROP TABLE IF EXISTS resources;
DROP TABLE IF EXISTS meta;
