// Command evalgolden scores testdata/golden-queries.json against a local DB.
//
// It reports Hit@1 / Hit@3 / MRR / sibling inversions / latency per lane:
// FTS, hybrid (FTS+vector), and hybrid+rerank for each ONNX reranker file
// that exists (BGE / zerank × fp32 / int8).
//
// Do not use the Fake embedder for hybrid or rerank numbers.
// Re-ingest with KLIMAT_EMBEDDER=onnx so stored vectors match F2LLM.
//
//	make eval-golden
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mong-x/klimatsearch/internal/embedder"
	klimateval "github.com/mong-x/klimatsearch/internal/eval"
	"github.com/mong-x/klimatsearch/internal/reranker"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
)

type lane struct {
	name   string
	vector bool
	rerank bool
	model  string
	quant  string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "evalgolden: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("evalgolden", flag.ContinueOnError)
	golden := fs.String("golden", "testdata/golden-queries.json", "labeled queries")
	db := fs.String("db", env("KLIMAT_DB", "./data/klimat.db"), "SQLite path")
	models := fs.String("models", env("KLIMAT_MODELS", "./models"), "ONNX models directory")
	embKind := fs.String("embedder", env("KLIMAT_EMBEDDER", "onnx"), "embedder: onnx|fake (fake skips hybrid)")
	embName := fs.String("embedding-model", env("KLIMAT_EMBEDDING_MODEL", "f2llm-v2-80m"), "embedder dir")
	outPath := fs.String("out", "", "write full JSON report here")
	allowFake := fs.Bool("allow-fake", false, "run hybrid lanes with the Fake embedder (not a quality signal)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	g, err := klimateval.LoadFile(*golden)
	if err != nil {
		return err
	}
	st, err := store.Open(*db)
	if err != nil {
		return err
	}
	defer st.Close()
	emb, err := embedder.New(*embKind, *models, *embName)
	if err != nil {
		return err
	}
	st.ConfigureVector(embedder.DimOf(emb))

	_, fake := emb.(embedder.Fake)
	vectorOK := st.VectorEnabled() && (!fake || *allowFake)
	if fake && !*allowFake {
		fmt.Fprintln(os.Stderr, "warning: Fake embedder — hybrid and rerank lanes skipped. Re-ingest with KLIMAT_EMBEDDER=onnx.")
		vectorOK = false
	} else if fake {
		fmt.Fprintln(os.Stderr, "warning: --allow-fake: hybrid numbers are not a quality signal.")
	} else if !st.VectorEnabled() {
		fmt.Fprintf(os.Stderr, "warning: vector search disabled (%s)\n", st.VectorSkipReason())
		vectorOK = false
	}

	lanes := []lane{{name: "fts", vector: false, rerank: false}}
	if vectorOK {
		lanes = append(lanes, lane{name: "hybrid", vector: true, rerank: false})
		for _, spec := range []struct {
			name, dir string
		}{
			{"bge", "bge-reranker-v2-m3"},
			{"zerank", "zerank-1-small"},
		} {
			dir := filepath.Join(*models, spec.dir)
			if _, err := embedder.ResolveONNX(dir, "fp32"); err == nil {
				lanes = append(lanes, lane{name: "hybrid+" + spec.name + "-fp32", vector: true, rerank: true, model: spec.dir, quant: "fp32"})
			}
			if _, err := embedder.ResolveONNX(dir, "int8"); err == nil {
				lanes = append(lanes, lane{name: "hybrid+" + spec.name + "-int8", vector: true, rerank: true, model: spec.dir, quant: "int8"})
			}
		}
	}

	ctx := context.Background()
	if vectorOK && !fake {
		probe := search.New(st, st, emb, reranker.None{})
		hits, err := probe.Search(ctx, search.Query{Text: "spånskiva", Lang: "sv", Vector: true})
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: hybrid probe failed: %v\n", err)
		} else if len(hits) == 0 || hits[0].ResourceID != "6000000000" {
			got := ""
			if len(hits) > 0 {
				got = hits[0].ResourceID
			}
			fmt.Fprintf(os.Stderr, "warning: hybrid top-1 for spånskiva is %q, want 6000000000. Stored vectors may not be F2LLM — re-ingest with KLIMAT_EMBEDDER=onnx.\n", got)
		}
	}
	byLane := map[string][]klimateval.Case{}
	var order []string
	for _, ln := range lanes {
		fmt.Fprintf(os.Stderr, "lane %s (%d queries)\n", ln.name, len(g.Queries))
		cases, err := runLane(ctx, st, emb, *models, ln, g.Queries)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip %s: %v\n", ln.name, err)
			continue
		}
		byLane[ln.name] = cases
		order = append(order, ln.name)
	}

	summaries := make([]klimateval.LaneSummary, 0, len(order))
	for _, name := range order {
		summaries = append(summaries, klimateval.Summarize(name, byLane[name]))
	}

	var agreements []klimateval.Agreement
	pairs := [][2]string{
		{"hybrid+bge-fp32", "hybrid+bge-int8"},
		{"hybrid+zerank-fp32", "hybrid+zerank-int8"},
	}
	for _, p := range pairs {
		a, okA := byLane[p[0]]
		b, okB := byLane[p[1]]
		if okA && okB {
			agreements = append(agreements, klimateval.Agree(p[0], p[1], a, b))
		}
	}

	printTable(os.Stdout, g, summaries, agreements, byLane, fake, vectorOK)

	if *outPath != "" {
		report := map[string]any{
			"catalog":         g.Catalog,
			"dataset_version": g.DatasetVersion,
			"n_queries":       len(g.Queries),
			"fake_embedder":   fake,
			"vector_enabled":  vectorOK,
			"summaries":       summaries,
			"agreements":      agreements,
			"cases":           flatten(byLane, order),
		}
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*outPath, b, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", *outPath)
	}
	return nil
}

func runLane(ctx context.Context, st *store.Store, emb search.Embedder, models string, ln lane, queries []klimateval.Query) ([]klimateval.Case, error) {
	rr := search.Reranker(reranker.None{})
	if ln.rerank {
		prev := os.Getenv("KLIMAT_ONNX_QUANT")
		_ = os.Setenv("KLIMAT_ONNX_QUANT", ln.quant)
		loaded, err := reranker.New("onnx", models, ln.model)
		_ = os.Setenv("KLIMAT_ONNX_QUANT", prev)
		if err != nil {
			return nil, err
		}
		rr = loaded
		if c, ok := loaded.(interface{ Close() }); ok {
			defer c.Close()
		}
	}
	eng := search.New(st, st, emb, rr)
	var cases []klimateval.Case
	for i, q := range queries {
		ids, d, err := searchIDs(ctx, eng, q, ln.vector, ln.rerank)
		if err != nil {
			return nil, fmt.Errorf("query %d %q: %w", i, q.Q, err)
		}
		cases = append(cases, klimateval.Score(ln.name, q, ids, d))
	}
	return cases, nil
}

func searchIDs(ctx context.Context, eng *search.Engine, q klimateval.Query, vector, rerank bool) ([]string, time.Duration, error) {
	start := time.Now()
	hits, err := eng.Search(ctx, search.Query{Text: q.Q, Lang: q.Lang, Vector: vector, Rerank: rerank})
	d := time.Since(start)
	if err != nil {
		return nil, d, err
	}
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.ResourceID)
	}
	return ids, d, nil
}

func flatten(byLane map[string][]klimateval.Case, order []string) []klimateval.Case {
	var out []klimateval.Case
	for _, name := range order {
		out = append(out, byLane[name]...)
	}
	return out
}

func printTable(w io.Writer, g klimateval.File, sums []klimateval.LaneSummary, ag []klimateval.Agreement, byLane map[string][]klimateval.Case, fake, vectorOK bool) {
	fmt.Fprintf(w, "# klimatsearch golden eval\n")
	fmt.Fprintf(w, "dataset %s  n=%d  fake_embedder=%v  vector=%v\n\n", g.DatasetVersion, len(g.Queries), fake, vectorOK)
	fmt.Fprintf(w, "| lane | n | hit@1 | hit@3 | MRR | inversions | miss | p50 ms | p95 ms |\n")
	fmt.Fprintf(w, "| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, s := range sums {
		fmt.Fprintf(w, "| %s | %d | %.3f | %.3f | %.3f | %.3f | %.3f | %.1f | %.1f |\n",
			s.Lane, s.N, s.HitAt1, s.HitAt3, s.MRR, s.Inversions, s.Miss, s.P50Ms, s.P95Ms)
	}
	if len(ag) > 0 {
		fmt.Fprintf(w, "\n## INT8 vs fp32 rank agreement\n\n")
		fmt.Fprintf(w, "| pair | n | top1 agree | overlap@10 |\n| --- | ---: | ---: | ---: |\n")
		for _, a := range ag {
			fmt.Fprintf(w, "| %s vs %s | %d | %.3f | %.3f |\n", a.A, a.B, a.N, a.Top1Agree, a.OverlapAt10)
		}
	}
	fmt.Fprintf(w, "\n## Failures (miss, hit@1 miss, or sibling inversion)\n\n")
	any := false
	for _, s := range sums {
		for _, c := range byLane[s.Lane] {
			if c.Miss || !c.HitAt1 || c.Inversion {
				any = true
				why := "hit@1 miss"
				if c.Miss {
					why = "not in top-20"
				} else if c.Inversion {
					why = "sibling inversion"
				}
				fmt.Fprintf(w, "- [%s] %s %q expect %s rank=%d (%s) top=%s\n",
					c.Lane, c.QueryID, c.Q, c.ExpectID, c.Rank, why, strings.Join(c.Top, ","))
			}
		}
	}
	if !any {
		fmt.Fprintf(w, "_none_\n")
	}
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
