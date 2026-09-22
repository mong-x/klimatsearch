package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"

	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/migrations"
)

const DefaultDim = 320

var (
	vecOnce      sync.Once
	migrateMu    sync.Mutex
	ErrNotFound  = errors.New("resource not found")
	ErrAmbiguous = errors.New("resource id is ambiguous across catalogs")
)

// Store is the SQLite + FTS5 + sqlite-vec persistence layer.
type Store struct {
	db               *sql.DB
	embeddingDim     int
	vectorOK         bool
	vectorSkipReason string
	log              *slog.Logger
}

// Open creates/opens a SQLite file, enables WAL, runs goose migrations, and
// records embedding_dim. Call sqlite_vec.Auto before sql.Open.
func Open(path string) (*Store, error) {
	return OpenFS(path, migrations.FS, ".")
}

// OpenFS is Open with a custom migration filesystem (tests).
func OpenFS(path string, fsys fs.FS, dir string) (*Store, error) {
	vecOnce.Do(func() { sqlite_vec.Auto() })

	if path != ":memory:" && !strings.HasPrefix(path, "file::memory:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir db dir: %w", err)
		}
	}
	dsn := path
	if !strings.Contains(dsn, "?") {
		dsn = dsn + "?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on"
	}
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	migrateMu.Lock()
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		migrateMu.Unlock()
		_ = db.Close()
		return nil, fmt.Errorf("goose dialect: %w", err)
	}
	goose.SetBaseFS(fsys)
	err = goose.Up(db, dir)
	migrateMu.Unlock()
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	s := &Store{db: db, vectorOK: true, log: slog.Default()}
	dim, err := s.MetaInt(context.Background(), "embedding_dim")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("read embedding_dim: %w", err)
	}
	if dim <= 0 {
		dim = DefaultDim
		if err := s.SetMeta(context.Background(), "embedding_dim", strconv.Itoa(dim)); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	s.embeddingDim = dim
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) EmbeddingDim() int { return s.embeddingDim }

func (s *Store) VectorEnabled() bool { return s.vectorOK }

func (s *Store) VectorSkipReason() string { return s.vectorSkipReason }

// ConfigureVector disables KNN when the live embedder dim does not match meta.
func (s *Store) ConfigureVector(modelDim int) {
	if modelDim != s.embeddingDim {
		s.vectorOK = false
		s.vectorSkipReason = fmt.Sprintf("stored embedding_dim=%d != model dim=%d; refusing vector search (re-embed required)", s.embeddingDim, modelDim)
		s.log.Error("vector search disabled", "reason", s.vectorSkipReason)
		return
	}
	s.vectorOK = true
	s.vectorSkipReason = ""
}

func (s *Store) Meta(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("meta %s: %w", key, err)
	}
	return v, nil
}

func (s *Store) MetaInt(ctx context.Context, key string) (int, error) {
	v, err := s.Meta(ctx, key)
	if err != nil {
		return 0, err
	}
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("meta %s atoi: %w", key, err)
	}
	return n, nil
}

func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("set meta %s: %w", key, err)
	}
	return nil
}

func (s *Store) Hash(ctx context.Context, catalogID, id string) (string, error) {
	if catalogID == "" {
		catalogID = model.CatalogBoverket
	}
	var h string
	err := s.db.QueryRowContext(ctx, `SELECT content_hash FROM resources WHERE catalog_id = ? AND id = ?`, catalogID, id).Scan(&h)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("hash %s: %w", model.DocID(catalogID, id), err)
	}
	return h, nil
}

func (s *Store) Upsert(ctx context.Context, r model.Resource, embedding []float32) error {
	if r.CatalogID == "" {
		r.CatalogID = model.CatalogBoverket
	}
	if r.Conversions == nil {
		r.Conversions = map[string]float64{}
	}
	conv, err := json.Marshal(r.Conversions)
	if err != nil {
		return fmt.Errorf("marshal conversions: %w", err)
	}
	details, err := json.Marshal(r.Details)
	if err != nil {
		return fmt.Errorf("marshal details: %w", err)
	}
	// One transaction: the resources row (FTS rides its triggers) and the
	// vector are a single logical write. Committing the row without its
	// vector would let the ContentHash skip permanently hide the Resource
	// from vector search.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin upsert %s: %w", r.DocID(), err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
INSERT INTO resources (
    catalog_id, id, name_sv, name_en, description_sv, description_en,
    applicability_sv, applicability_en, synonyms,
    a1_a3, unit, conversions_json, category, category_code, version,
    content_hash, raw_json, details_json, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
ON CONFLICT(catalog_id, id) DO UPDATE SET
    name_sv=excluded.name_sv,
    name_en=excluded.name_en,
    description_sv=excluded.description_sv,
    description_en=excluded.description_en,
    applicability_sv=excluded.applicability_sv,
    applicability_en=excluded.applicability_en,
    synonyms=excluded.synonyms,
    a1_a3=excluded.a1_a3,
    unit=excluded.unit,
    conversions_json=excluded.conversions_json,
    category=excluded.category,
    category_code=excluded.category_code,
    version=excluded.version,
    content_hash=excluded.content_hash,
    raw_json=excluded.raw_json,
    details_json=excluded.details_json,
    updated_at=excluded.updated_at
`, r.CatalogID, r.ResourceID, r.NameSV, r.NameEN, r.DescriptionSV, r.DescriptionEN,
		r.ApplicabilitySV, r.ApplicabilityEN, r.Synonyms,
		r.A1A3, r.Unit, string(conv), r.Category, r.CategoryCode, r.Version, r.Hash, r.RawJSON, string(details))
	if err != nil {
		return fmt.Errorf("upsert resource %s: %w", r.DocID(), err)
	}
	if len(embedding) > 0 {
		if err := upsertVec(ctx, tx, r.CatalogID, r.ResourceID, embedding); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit upsert %s: %w", r.DocID(), err)
	}
	return nil
}

// execer is the statement surface *sql.DB and *sql.Tx share.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func upsertVec(ctx context.Context, q execer, catalogID, id string, embedding []float32) error {
	blob, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return fmt.Errorf("serialize embedding: %w", err)
	}
	docID := model.DocID(catalogID, id)
	if _, err := q.ExecContext(ctx, `DELETE FROM resource_vec WHERE doc_id = ?`, docID); err != nil {
		return fmt.Errorf("delete vec %s: %w", docID, err)
	}
	if _, err := q.ExecContext(ctx, `INSERT INTO resource_vec(doc_id, embedding, catalog_id) VALUES (?, ?, ?)`, docID, blob, catalogID); err != nil {
		return fmt.Errorf("insert vec %s: %w", docID, err)
	}
	return nil
}

// PutVector writes one Resource's vector without touching the row or its
// ContentHash (the content did not change) - the backfill path.
func (s *Store) PutVector(ctx context.Context, catalogID, id string, embedding []float32) error {
	return upsertVec(ctx, s.db, catalogID, id, embedding)
}

// MissingVectors lists Resources whose vector row is absent: embedder-less
// ingest, or a torn write from before the Upsert transaction. (A dim change
// additionally needs resource_vec recreated at the new float[N] before a
// backfill can run.) Keys are drained before the per-row Get because the
// pool holds one connection.
func (s *Store) MissingVectors(ctx context.Context) ([]model.Resource, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT catalog_id, id FROM resources
WHERE NOT EXISTS (SELECT 1 FROM resource_vec WHERE doc_id = catalog_id || ':' || id)`)
	if err != nil {
		return nil, fmt.Errorf("missing vectors: %w", err)
	}
	type key struct{ catalog, id string }
	var keys []key
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.catalog, &k.id); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	out := make([]model.Resource, 0, len(keys))
	for _, k := range keys {
		r, err := s.Get(ctx, model.DocID(k.catalog, k.id))
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, nil
}

func (s *Store) Get(ctx context.Context, id string) (*model.Resource, error) {
	catalogID, resourceID := model.SplitDocID(id)
	if catalogID != "" {
		return s.getExact(ctx, catalogID, resourceID)
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT `+resourceCols+`
FROM resources r WHERE r.id = ?`, resourceID)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", id, err)
	}
	defer rows.Close()
	var found []model.Resource
	for rows.Next() {
		r, err := scanResource(rows)
		if err != nil {
			return nil, fmt.Errorf("get %s: %w", id, err)
		}
		found = append(found, *r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get %s: %w", id, err)
	}
	switch len(found) {
	case 0:
		return nil, ErrNotFound
	case 1:
		return &found[0], nil
	default:
		cats := make([]string, 0, len(found))
		for _, r := range found {
			cats = append(cats, r.CatalogID)
		}
		return nil, &AmbiguousError{ID: resourceID, Catalogs: cats}
	}
}

func (s *Store) getExact(ctx context.Context, catalogID, id string) (*model.Resource, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT `+resourceCols+`
FROM resources r WHERE r.catalog_id = ? AND r.id = ?`, catalogID, id)
	r, err := scanResource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", model.DocID(catalogID, id), err)
	}
	return r, nil
}

func (s *Store) List(ctx context.Context, catalogs []string) ([]model.Resource, error) {
	q := `
SELECT ` + resourceCols + `
FROM resources r`
	args := []any{}
	if clause, cargs := catalogWhere("r", catalogs); clause != "" {
		q += " WHERE " + strings.TrimPrefix(clause, " AND ")
		args = cargs
	}
	q += " ORDER BY r.catalog_id, r.id"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()
	var out []model.Resource
	for rows.Next() {
		r, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM resources`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	return n, nil
}

const resourceCols = `r.catalog_id, r.id, r.name_sv, r.name_en, r.description_sv, r.description_en,
       r.applicability_sv, r.applicability_en, r.synonyms, r.a1_a3, r.unit,
       r.conversions_json, r.category, r.category_code, r.version, r.content_hash, r.raw_json,
       r.details_json`

// SearchFTS runs BM25 over language-specific columns. Rank is 1-based in this list.
func (s *Store) SearchFTS(ctx context.Context, query, lang string, limit int, catalogs []string) ([]model.Ranked, error) {
	if limit <= 0 {
		limit = 20
	}
	match := columnFilter(lang) + quoteFTS(query)
	q := `
SELECT ` + resourceCols + `
FROM resources_fts
JOIN resources r ON r.rowid = resources_fts.rowid
WHERE resources_fts MATCH ?`
	args := []any{match}
	if clause, cargs := catalogWhere("r", catalogs); clause != "" {
		q += clause
		args = append(args, cargs...)
	}
	q += `
ORDER BY bm25(resources_fts)
LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("fts: %w", err)
	}
	defer rows.Close()
	return scanRanked(rows)
}

// SearchVector runs sqlite-vec cosine KNN. Rank is 1-based in this list.
func (s *Store) SearchVector(ctx context.Context, embedding []float32, lang string, limit int, catalogs []string) ([]model.Ranked, error) {
	if !s.vectorOK {
		return nil, fmt.Errorf("vector search disabled: %s", s.vectorSkipReason)
	}
	if limit <= 0 {
		limit = 20
	}
	blob, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return nil, fmt.Errorf("serialize query vec: %w", err)
	}
	q := `
SELECT ` + resourceCols + `
FROM resource_vec v
JOIN resources r ON r.catalog_id || ':' || r.id = v.doc_id
WHERE v.embedding MATCH ?
  AND k = ?`
	args := []any{blob, limit}
	if clause, cargs := catalogWhere("r", catalogs); clause != "" {
		q += clause
		args = append(args, cargs...)
	}
	q += `
ORDER BY v.distance`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("vector: %w", err)
	}
	defer rows.Close()
	return scanRanked(rows)
}

func catalogWhere(alias string, catalogs []string) (string, []any) {
	if len(catalogs) == 0 {
		return "", nil
	}
	ph := make([]string, len(catalogs))
	args := make([]any, len(catalogs))
	for i, c := range catalogs {
		ph[i] = "?"
		args[i] = c
	}
	col := "catalog_id"
	if alias != "" {
		col = alias + ".catalog_id"
	}
	return " AND " + col + " IN (" + strings.Join(ph, ",") + ")", args
}

func scanRanked(rows *sql.Rows) ([]model.Ranked, error) {
	var out []model.Ranked
	rank := 0
	for rows.Next() {
		r, err := scanResource(rows)
		if err != nil {
			return nil, fmt.Errorf("ranked scan: %w", err)
		}
		rank++
		out = append(out, model.Ranked{Resource: *r, Rank: rank})
	}
	return out, rows.Err()
}

func columnFilter(lang string) string {
	if lang == "en" {
		return `{name_en description_en}: `
	}
	return `{name_sv description_sv}: `
}

func quoteFTS(q string) string {
	q = strings.TrimSpace(q)
	q = strings.ReplaceAll(q, `"`, `""`)
	return `"` + q + `"`
}

type scanner interface {
	Scan(dest ...any) error
}

func scanResource(row scanner) (*model.Resource, error) {
	var r model.Resource
	var conv, details string
	if err := row.Scan(
		&r.CatalogID, &r.ResourceID, &r.NameSV, &r.NameEN, &r.DescriptionSV, &r.DescriptionEN,
		&r.ApplicabilitySV, &r.ApplicabilityEN, &r.Synonyms,
		&r.A1A3, &r.Unit, &conv, &r.Category, &r.CategoryCode, &r.Version, &r.Hash, &r.RawJSON, &details,
	); err != nil {
		return nil, err
	}
	r.Conversions = map[string]float64{}
	if conv != "" {
		if err := json.Unmarshal([]byte(conv), &r.Conversions); err != nil {
			return nil, fmt.Errorf("conversions json: %w", err)
		}
	}
	if details != "" && details != "{}" {
		if err := json.Unmarshal([]byte(details), &r.Details); err != nil {
			return nil, fmt.Errorf("details json: %w", err)
		}
	}
	return &r, nil
}

// AmbiguousError is returned when a bare Resource ID matches more than one Catalog.
type AmbiguousError struct {
	ID       string
	Catalogs []string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("resource %s is present in multiple catalogs; use catalog_id:resource_id", e.ID)
}

func (e *AmbiguousError) Unwrap() error { return ErrAmbiguous }

// ParseCatalogs splits a comma-separated databases query. Empty means all Catalogs.
func ParseCatalogs(s string) []string {
	return model.ParseCatalogIDs([]string{s})
}
