-- +goose Up

CREATE TABLE resources_new (
    catalog_id TEXT NOT NULL DEFAULT 'boverket',
    id TEXT NOT NULL,
    name_sv TEXT NOT NULL DEFAULT '',
    name_en TEXT NOT NULL DEFAULT '',
    description_sv TEXT NOT NULL DEFAULT '',
    description_en TEXT NOT NULL DEFAULT '',
    applicability_sv TEXT NOT NULL DEFAULT '',
    applicability_en TEXT NOT NULL DEFAULT '',
    synonyms TEXT NOT NULL DEFAULT '',
    a1_a3 REAL NOT NULL DEFAULT 0,
    unit TEXT NOT NULL DEFAULT '',
    conversions_json TEXT NOT NULL DEFAULT '{}',
    category TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    raw_json TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (catalog_id, id)
);

INSERT INTO resources_new (
    catalog_id, id, name_sv, name_en, description_sv, description_en,
    applicability_sv, applicability_en, synonyms,
    a1_a3, unit, conversions_json, category, version,
    content_hash, raw_json, updated_at
)
SELECT
    'boverket', id, name_sv, name_en, description_sv, description_en,
    '', '', '',
    a1_a3, unit, conversions_json, category, version,
    content_hash, raw_json, updated_at
FROM resources;

DROP TRIGGER IF EXISTS resources_au;
DROP TRIGGER IF EXISTS resources_ad;
DROP TRIGGER IF EXISTS resources_ai;
DROP TABLE IF EXISTS resources_fts;
DROP TABLE IF EXISTS resources;

ALTER TABLE resources_new RENAME TO resources;

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

INSERT INTO resources_fts(rowid, name_sv, name_en, description_sv, description_en)
SELECT rowid, name_sv, name_en, description_sv, description_en FROM resources;

CREATE TABLE resource_vec_migrate AS
SELECT resource_id AS old_id, embedding FROM resource_vec;

DROP TABLE IF EXISTS resource_vec;

CREATE VIRTUAL TABLE resource_vec USING vec0(
    doc_id TEXT PRIMARY KEY,
    embedding float[320] distance_metric=cosine,
    catalog_id TEXT
);

INSERT INTO resource_vec(doc_id, embedding, catalog_id)
SELECT 'boverket:' || old_id, embedding, 'boverket' FROM resource_vec_migrate;

DROP TABLE IF EXISTS resource_vec_migrate;

UPDATE meta SET value = '2' WHERE key = 'schema_version';

-- +goose Down

DROP TRIGGER IF EXISTS resources_au;
DROP TRIGGER IF EXISTS resources_ad;
DROP TRIGGER IF EXISTS resources_ai;
DROP TABLE IF EXISTS resources_fts;

CREATE TABLE resources_old (
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

INSERT INTO resources_old (
    id, name_sv, name_en, description_sv, description_en,
    a1_a3, unit, conversions_json, category, version,
    content_hash, raw_json, updated_at
)
SELECT
    id, name_sv, name_en, description_sv, description_en,
    a1_a3, unit, conversions_json, category, version,
    content_hash, raw_json, updated_at
FROM resources
WHERE catalog_id = 'boverket';

DROP TABLE IF EXISTS resources;
ALTER TABLE resources_old RENAME TO resources;

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

INSERT INTO resources_fts(rowid, name_sv, name_en, description_sv, description_en)
SELECT rowid, name_sv, name_en, description_sv, description_en FROM resources;

CREATE TABLE resource_vec_migrate AS
SELECT doc_id, embedding FROM resource_vec;

DROP TABLE IF EXISTS resource_vec;

CREATE VIRTUAL TABLE resource_vec USING vec0(
    resource_id TEXT PRIMARY KEY,
    embedding float[320] distance_metric=cosine
);

INSERT INTO resource_vec(resource_id, embedding)
SELECT CASE
    WHEN instr(doc_id, ':') > 0 THEN substr(doc_id, instr(doc_id, ':') + 1)
    ELSE doc_id
END, embedding
FROM resource_vec_migrate;

DROP TABLE IF EXISTS resource_vec_migrate;

UPDATE meta SET value = '1' WHERE key = 'schema_version';
