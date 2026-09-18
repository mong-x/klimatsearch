package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// File is testdata/golden-queries.json.
type File struct {
	Catalog        string  `json:"catalog"`
	DatasetVersion string  `json:"dataset_version"`
	Notes          string  `json:"notes"`
	Queries        []Query `json:"queries"`
}

// Query is one labeled retrieval case.
type Query struct {
	ID       string   `json:"id,omitempty"`
	Q        string   `json:"q"`
	Lang     string   `json:"lang"`
	ExpectID string   `json:"expect_id"`
	NearMiss []string `json:"near_miss,omitempty"`
	Cluster  string   `json:"cluster,omitempty"`
	Kind     string   `json:"kind,omitempty"`
}

// LoadFile reads a golden JSON file.
func LoadFile(path string) (File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return File{}, fmt.Errorf("golden json: %w", err)
	}
	if err := f.Validate(); err != nil {
		return File{}, err
	}
	return f, nil
}

func (f File) Validate() error {
	if len(f.Queries) == 0 {
		return fmt.Errorf("golden file has no queries")
	}
	for i, q := range f.Queries {
		if q.Q == "" || q.ExpectID == "" {
			return fmt.Errorf("query %d: q and expect_id are required", i)
		}
		if q.Lang != "sv" && q.Lang != "en" {
			return fmt.Errorf("query %d %q: lang must be sv|en", i, q.Q)
		}
	}
	return nil
}

// Case is one (lane, query) outcome.
type Case struct {
	Lane      string        `json:"lane"`
	QueryID   string        `json:"query_id"`
	Q         string        `json:"q"`
	Lang      string        `json:"lang"`
	Cluster   string        `json:"cluster,omitempty"`
	Kind      string        `json:"kind,omitempty"`
	ExpectID  string        `json:"expect_id"`
	Rank      int           `json:"rank"` // 1-based; 0 = not in the list
	Top       []string      `json:"top"`
	HitAt1    bool          `json:"hit_at_1"`
	HitAt3    bool          `json:"hit_at_3"`
	Inversion bool          `json:"inversion"`
	Miss      bool          `json:"miss"`
	Latency   time.Duration `json:"latency_ns"`
}

// Score ranks expect_id among retrieved IDs.
func Score(lane string, q Query, ids []string, d time.Duration) Case {
	qid := q.ID
	if qid == "" {
		qid = q.Lang + ":" + q.Q
	}
	rank := index1(ids, q.ExpectID)
	inv := inverted(q.ExpectID, q.NearMiss, ids)
	return Case{
		Lane:      lane,
		QueryID:   qid,
		Q:         q.Q,
		Lang:      q.Lang,
		Cluster:   q.Cluster,
		Kind:      q.Kind,
		ExpectID:  q.ExpectID,
		Rank:      rank,
		Top:       clip(ids, 10),
		HitAt1:    rank == 1,
		HitAt3:    rank > 0 && rank <= 3,
		Inversion: inv,
		Miss:      rank == 0,
		Latency:   d,
	}
}

func index1(ids []string, want string) int {
	for i, id := range ids {
		if id == want {
			return i + 1
		}
	}
	return 0
}

func inverted(expect string, near []string, ids []string) bool {
	er := index1(ids, expect)
	if er == 0 {
		return false
	}
	for _, n := range near {
		nr := index1(ids, n)
		if nr > 0 && nr < er {
			return true
		}
	}
	return false
}

func clip(ids []string, n int) []string {
	if len(ids) <= n {
		out := make([]string, len(ids))
		copy(out, ids)
		return out
	}
	out := make([]string, n)
	copy(out, ids[:n])
	return out
}

// LaneSummary is aggregate metrics for one retrieval lane.
type LaneSummary struct {
	Lane       string  `json:"lane"`
	N          int     `json:"n"`
	HitAt1     float64 `json:"hit_at_1"`
	HitAt3     float64 `json:"hit_at_3"`
	MRR        float64 `json:"mrr"`
	Inversions float64 `json:"inversions"`
	Miss       float64 `json:"miss"`
	P50Ms      float64 `json:"p50_ms"`
	P95Ms      float64 `json:"p95_ms"`
}

func Summarize(lane string, cases []Case) LaneSummary {
	var s LaneSummary
	s.Lane = lane
	if len(cases) == 0 {
		return s
	}
	s.N = len(cases)
	var hit1, hit3, inv, miss, mrr float64
	ms := make([]float64, len(cases))
	for i, c := range cases {
		if c.HitAt1 {
			hit1++
		}
		if c.HitAt3 {
			hit3++
		}
		if c.Inversion {
			inv++
		}
		if c.Miss {
			miss++
		}
		if c.Rank > 0 {
			mrr += 1 / float64(c.Rank)
		}
		ms[i] = float64(c.Latency.Microseconds()) / 1000
	}
	n := float64(len(cases))
	s.HitAt1 = hit1 / n
	s.HitAt3 = hit3 / n
	s.MRR = mrr / n
	s.Inversions = inv / n
	s.Miss = miss / n
	sort.Float64s(ms)
	s.P50Ms = percentile(ms, 0.50)
	s.P95Ms = percentile(ms, 0.95)
	return s
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := int(p * float64(len(sorted)-1))
	return sorted[idx]
}

// Agreement is top-k ID overlap between two lanes on the same queries.
type Agreement struct {
	A           string  `json:"a"`
	B           string  `json:"b"`
	N           int     `json:"n"`
	Top1Agree   float64 `json:"top1_agree"`
	OverlapAt10 float64 `json:"overlap_at_10"`
}

func Agree(aName, bName string, a, b []Case) Agreement {
	byQ := map[string][]string{}
	for _, c := range b {
		byQ[c.QueryID] = c.Top
	}
	var top1, overlap, n float64
	for _, c := range a {
		other, ok := byQ[c.QueryID]
		if !ok {
			continue
		}
		n++
		if len(c.Top) > 0 && len(other) > 0 && c.Top[0] == other[0] {
			top1++
		}
		overlap += jaccard(c.Top, other)
	}
	out := Agreement{A: aName, B: bName, N: int(n)}
	if n > 0 {
		out.Top1Agree = top1 / n
		out.OverlapAt10 = overlap / n
	}
	return out
}

func jaccard(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	set := map[string]struct{}{}
	for _, x := range a {
		set[x] = struct{}{}
	}
	inter := 0
	for _, x := range b {
		if _, ok := set[x]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
