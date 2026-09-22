// Package testworld is the shared, data-only fixture behind every transport
// test: the canonical Resource pair, a seeded store, and the engine over it.
// Data only — no builders, no assertions — so the Resource contract changes
// in one file instead of six suites in lockstep.
package testworld

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/reranker"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
)

// The canonical pair: Betong (full Details) and Stål (lean). These are the
// ids transport assertions run against.
var (
	Betong = model.Resource{
		CatalogID:       model.CatalogBoverket,
		ResourceID:      "6000000991",
		NameSV:          "Betong",
		NameEN:          "Concrete",
		DescriptionSV:   "Generisk betong",
		DescriptionEN:   "Generic concrete",
		ApplicabilitySV: "Stomme",
		A1A3:            0.12,
		Unit:            "kg",
		Conversions:     map[string]float64{"kg/m³": 2400},
		Category:        "Betong",
		CategoryCode:    "6",
		Version:         "t",
		Details: model.Details{
			A1A3Conservative: 1.25,
			A4:               ptr(0.04),
			A51:              ptr(0.02),
		},
	}
	Stal = model.Resource{
		CatalogID:  model.CatalogBoverket,
		ResourceID: "6000000992",
		NameSV:     "Konstruktionsstål",
		NameEN:     "Structural steel",
		A1A3:       1.55,
		Unit:       "kg",
		Version:    "t",
	}
)

func ptr(v float64) *float64 { return &v }

// SkipCGO is the one home of the CGO skip guard.
func SkipCGO(t testing.TB) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
}

// Open returns a fresh store with the fake embedder configured and nothing
// seeded - for tests whose data is load-bearing (ambiguity, ingest, guard).
func Open(t testing.TB) (*store.Store, embedder.Fake) {
	t.Helper()
	SkipCGO(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "testworld.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())
	return st, fake
}

// Put hashes, embeds, and upserts one Resource.
func Put(t testing.TB, st *store.Store, fake embedder.Fake, r model.Resource) {
	t.Helper()
	h, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	r.Hash = h
	vec, err := fake.Embed(r.EmbeddingText())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(t.Context(), r, vec); err != nil {
		t.Fatal(err)
	}
}

// Engine returns the standard engine over a store.
func Engine(st *store.Store, fake embedder.Fake) search.SearchEngine {
	return search.New(st, st, fake, reranker.None{})
}

// Seed returns a store with the canonical pair upserted and the engine over
// it - the standard world of the transport tests.
func Seed(t testing.TB) (*store.Store, search.SearchEngine, embedder.Fake) {
	t.Helper()
	st, fake := Open(t)
	Put(t, st, fake, Betong)
	Put(t, st, fake, Stal)
	return st, Engine(st, fake), fake
}

// Jar is a minimal cookie jar for guard/console composition tests.
type Jar struct{ Values map[string]string }

func NewJar() *Jar { return &Jar{Values: map[string]string{}} }

func (j *Jar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	for _, c := range cookies {
		j.Values[c.Name] = c.Value
	}
}

func (j *Jar) Cookies(u *url.URL) []*http.Cookie {
	var out []*http.Cookie
	for k, v := range j.Values {
		out = append(out, &http.Cookie{Name: k, Value: v})
	}
	return out
}
