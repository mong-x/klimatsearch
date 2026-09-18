package eval

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestScoreHitAndInversion(t *testing.T) {
	q := Query{ID: "x", Q: "glasull", Lang: "sv", ExpectID: "3", NearMiss: []string{"4", "5"}}
	c := Score("fts", q, []string{"4", "3", "5"}, time.Millisecond)
	if c.HitAt1 || !c.HitAt3 || !c.Inversion || c.Rank != 2 {
		t.Fatalf("%+v", c)
	}
	ok := Score("fts", q, []string{"3", "4"}, 0)
	if !ok.HitAt1 || ok.Inversion || ok.Miss {
		t.Fatalf("%+v", ok)
	}
	miss := Score("fts", q, []string{"9"}, 0)
	if !miss.Miss || miss.Inversion {
		t.Fatalf("%+v", miss)
	}
}

func TestSummarizeAndAgree(t *testing.T) {
	q := Query{ID: "a", Q: "q", Lang: "sv", ExpectID: "1"}
	a := []Case{
		Score("fp32", q, []string{"1", "2"}, time.Millisecond),
	}
	b := []Case{
		Score("int8", q, []string{"1", "3"}, time.Millisecond),
	}
	s := Summarize("fp32", a)
	if s.N != 1 || s.HitAt1 != 1 || s.MRR != 1 {
		t.Fatalf("%+v", s)
	}
	ag := Agree("fp32", "int8", a, b)
	if ag.Top1Agree != 1 || ag.OverlapAt10 <= 0 {
		t.Fatalf("%+v", ag)
	}
}

func TestLoadRepoGolden(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "golden-queries.json")
	f, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Queries) < 30 {
		t.Fatalf("want ≥30 labeled queries, got %d", len(f.Queries))
	}
	if f.DatasetVersion != "02.07.000" {
		t.Fatalf("dataset_version=%s", f.DatasetVersion)
	}
}
