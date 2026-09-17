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
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/migrations"
)

const DefaultDim = 320

var (
	vecOnce     sync.Once
	migrateMu   sync.Mutex
	ErrNotFound = errors.New("resource not found")
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

func (s *Store) Hash(ctx context.Context, id string) (string, error) {
	var h string
	err := s.db.QueryRowContext(ctx, `SELECT content_hash FROM resources WHERE id = ?`, id).Scan(&h)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("hash %s: %w", id, err)
	}
	return h, nil
}

func (s *Store) Upsert(ctx context.Context, r model.Resource, embedding []float32) error {
	if r.Conversions == nil {
		r.Conversions = map[string]float64{}
	}
	conv, err := json.Marshal(r.Conversions)
	if err != nil {
		return fmt.Errorf("marshal conversions: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO resources (
    id, name_sv, name_en, description_sv, description_en,
    a1_a3, unit, conversions_json, category, version,
    content_hash, raw_json, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
ON CONFLICT(id) DO UPDATE SET
    name_sv=excluded.name_sv,
    name_en=excluded.name_en,
    description_sv=excluded.description_sv,
    description_en=excluded.description_en,
    a1_a3=excluded.a1_a3,
    unit=excluded.unit,
    conversions_json=excluded.conversions_json,
    category=excluded.category,
    version=excluded.version,
    content_hash=excluded.content_hash,
    raw_json=excluded.raw_json,
    updated_at=excluded.updated_at
`, r.ResourceID, r.NameSV, r.NameEN, r.DescriptionSV, r.DescriptionEN,
		r.A1A3, r.Unit, string(conv), r.Category, r.Version, r.ContentHash, r.RawJSON)
	if err != nil {
		return fmt.Errorf("upsert resource %s: %w", r.ResourceID, err)
	}
	if len(embedding) > 0 {
		if err := s.upsertVec(ctx, r.ResourceID, embedding); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) upsertVec(ctx context.Context, id string, embedding []float32) error {
	blob, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return fmt.Errorf("serialize embedding: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM resource_vec WHERE resource_id = ?`, id); err != nil {
		return fmt.Errorf("delete vec %s: %w", id, err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO resource_vec(resource_id, embedding) VALUES (?, ?)`, id, blob); err != nil {
		return fmt.Errorf("insert vec %s: %w", id, err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, id string) (*model.Resource, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name_sv, name_en, description_sv, description_en, a1_a3, unit,
       conversions_json, category, version, content_hash, raw_json
FROM resources WHERE id = ?`, id)
	r, err := scanResource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", id, err)
	}
	return r, nil
}

func (s *Store) List(ctx context.Context) ([]model.Resource, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name_sv, name_en, description_sv, description_en, a1_a3, unit,
       conversions_json, category, version, content_hash, raw_json
FROM resources ORDER BY id`)
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

// SearchFTS runs BM25 over language-specific columns.
func (s *Store) SearchFTS(ctx context.Context, query, lang string, limit int) ([]search.SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	match := columnFilter(lang) + quoteFTS(query)
	rows, err := s.db.QueryContext(ctx, `
SELECT r.id, r.name_sv, r.name_en, r.description_sv, r.description_en, r.a1_a3, r.unit,
       bm25(resources_fts) AS rank
FROM resources_fts
JOIN resources r ON r.rowid = resources_fts.rowid
WHERE resources_fts MATCH ?
ORDER BY rank
LIMIT ?`, match, limit)
	if err != nil {
		return nil, fmt.Errorf("fts: %w", err)
	}
	defer rows.Close()
	var out []search.SearchResult
	for rows.Next() {
		var res search.SearchResult
		var rank float64
		if err := rows.Scan(&res.ID, &res.NameSV, &res.NameEN, &res.DescriptionSV, &res.DescriptionEN, &res.A1A3, &res.Unit, &rank); err != nil {
			return nil, fmt.Errorf("fts scan: %w", err)
		}
		res.Lang = lang
		res.Source = "fts"
		res.Score = 1.0 / (1.0 + rank)
		out = append(out, res)
	}
	return out, rows.Err()
}

// SearchVector runs sqlite-vec cosine KNN.
func (s *Store) SearchVector(ctx context.Context, embedding []float32, lang string, limit int) ([]search.SearchResult, error) {
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
	rows, err := s.db.QueryContext(ctx, `
SELECT r.id, r.name_sv, r.name_en, r.description_sv, r.description_en, r.a1_a3, r.unit,
       v.distance
FROM resource_vec v
JOIN resources r ON r.id = v.resource_id
WHERE v.embedding MATCH ?
  AND k = ?
ORDER BY v.distance`, blob, limit)
	if err != nil {
		return nil, fmt.Errorf("vector: %w", err)
	}
	defer rows.Close()
	var out []search.SearchResult
	for rows.Next() {
		var res search.SearchResult
		var dist float64
		if err := rows.Scan(&res.ID, &res.NameSV, &res.NameEN, &res.DescriptionSV, &res.DescriptionEN, &res.A1A3, &res.Unit, &dist); err != nil {
			return nil, fmt.Errorf("vector scan: %w", err)
		}
		res.Lang = lang
		res.Source = "vector"
		res.Score = 1.0 / (1.0 + dist)
		out = append(out, res)
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
	var conv string
	if err := row.Scan(
		&r.ResourceID, &r.NameSV, &r.NameEN, &r.DescriptionSV, &r.DescriptionEN,
		&r.A1A3, &r.Unit, &conv, &r.Category, &r.Version, &r.ContentHash, &r.RawJSON,
	); err != nil {
		return nil, err
	}
	r.Conversions = map[string]float64{}
	if conv != "" {
		if err := json.Unmarshal([]byte(conv), &r.Conversions); err != nil {
			return nil, fmt.Errorf("conversions json: %w", err)
		}
	}
	return &r, nil
}
