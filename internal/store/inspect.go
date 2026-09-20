package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"github.com/mong-x/klimatsearch/internal/model"
)

// Inspectable is the allowlist of objects the operator console may browse.
// Arbitrary table names from the query string are rejected.
var Inspectable = []InspectTable{
	{Name: "resources", Kind: "table", Note: "Catalog rows. Identity is (catalog_id, id)."},
	{Name: "resources_fts", Kind: "fts5", Note: "BM25 index over names and descriptions."},
	{Name: "resource_vec", Kind: "vec0", Note: "sqlite-vec cosine embeddings, key doc_id."},
	{Name: "meta", Kind: "kv", Note: "schema_version, embedding_dim, last ingest."},
}

// InspectTable is one SQLite object the console can open.
type InspectTable struct {
	Name string
	Kind string
	Note string
	Rows int
}

// Coverage is embedding presence vs resources.
type Coverage struct {
	Resources  int
	Vectors    int
	FTS        int
	Missing    int
	Dim        int
	VectorOK   bool
	SkipReason string
}

// InspectRow is a resource plus whether a vector exists.
type InspectRow struct {
	CatalogID string
	ID        string
	DocID     string
	Name      string
	Category  string
	Unit      string
	Version   string
	Hash      string
	A1A3      float64
	HasVec    bool
	BlobBytes int
}

// EmbeddingView is a deserialized vector for one doc_id.
type EmbeddingView struct {
	DocID   string
	Dim     int
	Norm    float64
	Min     float64
	Max     float64
	Mean    float64
	Preview []float32
	Tail    []float32
}

// MetaPair is one meta row.
type MetaPair struct {
	Key   string
	Value string
}

func inspectable(name string) bool {
	for _, t := range Inspectable {
		if t.Name == name {
			return true
		}
	}
	return false
}

// TableStats returns row counts for allowlisted objects.
func (s *Store) TableStats(ctx context.Context) ([]InspectTable, error) {
	out := make([]InspectTable, len(Inspectable))
	copy(out, Inspectable)
	for i := range out {
		n, err := s.countTable(ctx, out[i].Name)
		if err != nil {
			return nil, err
		}
		out[i].Rows = n
	}
	return out, nil
}

func (s *Store) countTable(ctx context.Context, name string) (int, error) {
	if !inspectable(name) {
		return 0, fmt.Errorf("unknown inspect table %q", name)
	}
	var n int
	q := `SELECT COUNT(*) FROM ` + name
	if err := s.db.QueryRowContext(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("count %s: %w", name, err)
	}
	return n, nil
}

// Coverage reports how many Resources have a vector and an FTS row.
func (s *Store) Coverage(ctx context.Context) (Coverage, error) {
	c := Coverage{Dim: s.embeddingDim, VectorOK: s.vectorOK, SkipReason: s.vectorSkipReason}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM resources`).Scan(&c.Resources); err != nil {
		return c, fmt.Errorf("coverage resources: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource_vec`).Scan(&c.Vectors); err != nil {
		return c, fmt.Errorf("coverage vec: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM resources_fts`).Scan(&c.FTS); err != nil {
		return c, fmt.Errorf("coverage fts: %w", err)
	}
	c.Missing = c.Resources - c.Vectors
	if c.Missing < 0 {
		c.Missing = 0
	}
	return c, nil
}

// MetaAll returns every meta key.
func (s *Store) MetaAll(ctx context.Context) ([]MetaPair, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM meta ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("meta all: %w", err)
	}
	defer rows.Close()
	var out []MetaPair
	for rows.Next() {
		var p MetaPair
		if err := rows.Scan(&p.Key, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// InspectResources pages Catalog rows with a vector presence flag.
func (s *Store) InspectResources(ctx context.Context, catalog, q string, missingOnly bool, offset, limit int) ([]InspectRow, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where := []string{"1=1"}
	args := []any{}
	if catalog != "" {
		where = append(where, "r.catalog_id = ?")
		args = append(args, catalog)
	}
	if q = strings.TrimSpace(q); q != "" {
		where = append(where, "(r.id LIKE ? OR r.name_sv LIKE ? OR r.name_en LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	if missingOnly {
		where = append(where, "v.doc_id IS NULL")
	}
	clause := strings.Join(where, " AND ")
	var total int
	countQ := `SELECT COUNT(*) FROM resources r LEFT JOIN resource_vec v ON v.doc_id = r.catalog_id || ':' || r.id WHERE ` + clause
	if err := s.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("inspect count: %w", err)
	}
	qstr := `
SELECT r.catalog_id, r.id, r.name_sv, r.name_en, r.category, r.unit, r.version, r.content_hash, r.a1_a3,
       v.doc_id IS NOT NULL, COALESCE(length(v.embedding), 0)
FROM resources r
LEFT JOIN resource_vec v ON v.doc_id = r.catalog_id || ':' || r.id
WHERE ` + clause + `
ORDER BY r.catalog_id, r.id
LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, qstr, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("inspect resources: %w", err)
	}
	defer rows.Close()
	var out []InspectRow
	for rows.Next() {
		var r InspectRow
		var nameSV, nameEN string
		if err := rows.Scan(&r.CatalogID, &r.ID, &nameSV, &nameEN, &r.Category, &r.Unit, &r.Version, &r.Hash, &r.A1A3, &r.HasVec, &r.BlobBytes); err != nil {
			return nil, 0, err
		}
		r.DocID = model.DocID(r.CatalogID, r.ID)
		r.Name = nameSV
		if r.Name == "" {
			r.Name = nameEN
		}
		if len(r.Hash) > 12 {
			r.Hash = r.Hash[:12]
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// InspectFTS pages the FTS5 indexed text joined to resources.
func (s *Store) InspectFTS(ctx context.Context, q string, offset, limit int) ([][]string, []string, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	headers := []string{"doc_id", "name_sv", "name_en", "description_sv", "description_en"}
	where := "1=1"
	args := []any{}
	if q = strings.TrimSpace(q); q != "" {
		where = "r.id LIKE ? OR f.name_sv LIKE ? OR f.name_en LIKE ?"
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	var total int
	countQ := `SELECT COUNT(*) FROM resources_fts f JOIN resources r ON r.rowid = f.rowid WHERE ` + where
	if err := s.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, headers, 0, fmt.Errorf("inspect fts count: %w", err)
	}
	qstr := `
SELECT r.catalog_id || ':' || r.id, f.name_sv, f.name_en, f.description_sv, f.description_en
FROM resources_fts f
JOIN resources r ON r.rowid = f.rowid
WHERE ` + where + `
ORDER BY r.catalog_id, r.id
LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, qstr, args...)
	if err != nil {
		return nil, headers, 0, fmt.Errorf("inspect fts: %w", err)
	}
	defer rows.Close()
	grid, err := scanGrid(rows, 5)
	return grid, headers, total, err
}

// InspectVec pages vec0 keys without dumping the full float payload.
func (s *Store) InspectVec(ctx context.Context, q string, offset, limit int) ([][]string, []string, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	headers := []string{"doc_id", "catalog_id", "bytes", "floats"}
	where := "1=1"
	args := []any{}
	if q = strings.TrimSpace(q); q != "" {
		where = "doc_id LIKE ? OR catalog_id LIKE ?"
		like := "%" + q + "%"
		args = append(args, like, like)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource_vec WHERE `+where, args...).Scan(&total); err != nil {
		return nil, headers, 0, fmt.Errorf("inspect vec count: %w", err)
	}
	qstr := `SELECT doc_id, catalog_id, length(embedding) FROM resource_vec WHERE ` + where + ` ORDER BY doc_id LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, qstr, args...)
	if err != nil {
		return nil, headers, 0, fmt.Errorf("inspect vec: %w", err)
	}
	defer rows.Close()
	var grid [][]string
	for rows.Next() {
		var doc, cat string
		var n int
		if err := rows.Scan(&doc, &cat, &n); err != nil {
			return nil, headers, 0, err
		}
		floats := 0
		if n%4 == 0 {
			floats = n / 4
		}
		grid = append(grid, []string{doc, cat, fmt.Sprintf("%d", n), fmt.Sprintf("%d", floats)})
	}
	return grid, headers, total, rows.Err()
}

// InspectEmbedding deserializes one sqlite-vec blob.
func (s *Store) InspectEmbedding(ctx context.Context, docID string) (*EmbeddingView, error) {
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return nil, fmt.Errorf("doc id is required")
	}
	var blob []byte
	err := s.db.QueryRowContext(ctx, `SELECT embedding FROM resource_vec WHERE doc_id = ?`, docID).Scan(&blob)
	if errorsIsNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("embedding %s: %w", docID, err)
	}
	vec, err := deserializeVec(blob)
	if err != nil {
		return nil, fmt.Errorf("embedding %s: %w", docID, err)
	}
	v := EmbeddingView{DocID: docID, Dim: len(vec)}
	if len(vec) == 0 {
		return &v, nil
	}
	var sum, sumsq float64
	mn, mx := float64(vec[0]), float64(vec[0])
	for _, x := range vec {
		f := float64(x)
		sum += f
		sumsq += f * f
		if f < mn {
			mn = f
		}
		if f > mx {
			mx = f
		}
	}
	v.Min, v.Max, v.Mean = mn, mx, sum/float64(len(vec))
	v.Norm = math.Sqrt(sumsq)
	n := 12
	if len(vec) < n {
		n = len(vec)
	}
	v.Preview = append([]float32(nil), vec[:n]...)
	if len(vec) > n {
		v.Tail = append([]float32(nil), vec[len(vec)-8:]...)
	}
	return &v, nil
}

func deserializeVec(blob []byte) ([]float32, error) {
	if len(blob) == 0 {
		return nil, fmt.Errorf("empty embedding blob")
	}
	if len(blob)%4 != 0 {
		return nil, fmt.Errorf("blob length %d is not a multiple of 4", len(blob))
	}
	out := make([]float32, len(blob)/4)
	if err := binary.Read(bytes.NewReader(blob), binary.LittleEndian, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func scanGrid(rows *sql.Rows, cols int) ([][]string, error) {
	var grid [][]string
	for rows.Next() {
		raw := make([]sql.NullString, cols)
		ptrs := make([]any, cols)
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make([]string, cols)
		for i, v := range raw {
			if v.Valid {
				row[i] = v.String
			}
		}
		grid = append(grid, row)
	}
	return grid, rows.Err()
}

func errorsIsNoRows(err error) bool {
	return err == sql.ErrNoRows
}
