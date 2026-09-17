package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mong-x/klimatsearch/internal/hash"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/search"
)

// Store is the persistence ingest needs.
type Store interface {
	Hash(ctx context.Context, id string) (string, error)
	Upsert(ctx context.Context, r model.Resource, embedding []float32) error
	SetMeta(ctx context.Context, key, value string) error
}

// Fetcher loads Resources from JSON API, Excel, or a fixture. Tests inject fakes.
type Fetcher interface {
	Fetch(ctx context.Context) (Batch, error)
}

// Batch is one ingest snapshot.
type Batch struct {
	Resources []model.Resource
	Version   string
	Origin    string // "json", "excel", "fixture"
}

// Result counts upserts.
type Result struct {
	Seen     int
	Upserted int
	Skipped  int
	Origin   string
	Version  string
}

// Runner hashes, embeds changed Resources, and upserts.
type Runner struct {
	Store    Store
	Embedder search.Embedder
	Log      *slog.Logger
}

func (r *Runner) log() *slog.Logger {
	if r.Log != nil {
		return r.Log
	}
	return slog.Default()
}

func (r *Runner) Apply(ctx context.Context, batch Batch) (Result, error) {
	res := Result{Seen: len(batch.Resources), Origin: batch.Origin, Version: batch.Version}
	for _, item := range batch.Resources {
		if item.ResourceID == "" {
			res.Skipped++
			continue
		}
		h, err := hash.Content(item)
		if err != nil {
			return res, fmt.Errorf("hash %s: %w", item.ResourceID, err)
		}
		item.ContentHash = h
		prev, err := r.Store.Hash(ctx, item.ResourceID)
		if err != nil {
			return res, err
		}
		if prev != "" && prev == h {
			res.Skipped++
			continue
		}
		var emb []float32
		if r.Embedder != nil {
			emb, err = r.Embedder.Embed(item.EmbeddingText())
			if err != nil {
				return res, fmt.Errorf("embed %s: %w", item.ResourceID, err)
			}
		}
		if item.RawJSON == "" {
			b, err := json.Marshal(item)
			if err != nil {
				return res, fmt.Errorf("raw json %s: %w", item.ResourceID, err)
			}
			item.RawJSON = string(b)
		}
		if err := r.Store.Upsert(ctx, item, emb); err != nil {
			return res, err
		}
		res.Upserted++
	}
	if batch.Version != "" {
		if err := r.Store.SetMeta(ctx, "dataset_version", batch.Version); err != nil {
			return res, err
		}
	}
	r.log().Info("ingest complete", "origin", batch.Origin, "version", batch.Version, "seen", res.Seen, "upserted", res.Upserted, "skipped", res.Skipped)
	return res, nil
}

func (r *Runner) Run(ctx context.Context, f Fetcher) (Result, error) {
	batch, err := f.Fetch(ctx)
	if err != nil {
		return Result{}, err
	}
	return r.Apply(ctx, batch)
}

// Loop runs ingest on start and on interval. interval 0 means no ticker.
func Loop(ctx context.Context, r *Runner, f Fetcher, onStart bool, interval time.Duration) {
	if onStart {
		if _, err := r.Run(ctx, f); err != nil {
			r.log().Error("ingest on start failed", "err", err)
		}
	}
	if interval <= 0 {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := r.Run(ctx, f); err != nil {
				r.log().Error("scheduled ingest failed", "err", err)
			}
		}
	}
}

// LoadFixtureJSON reads testdata/fixtures/resources.json.
func LoadFixtureJSON(path string) (Batch, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Batch{}, fmt.Errorf("read fixture %s: %w", path, err)
	}
	batch, err := ParseJSON(b)
	if err != nil {
		return Batch{}, err
	}
	batch.Origin = "fixture"
	if batch.Version == "" {
		batch.Version = "fixture"
	}
	return batch, nil
}
