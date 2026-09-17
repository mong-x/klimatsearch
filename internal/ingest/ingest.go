package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/search"
)

// Store is the persistence ingest needs.
type Store interface {
	Hash(ctx context.Context, catalogID, id string) (string, error)
	Upsert(ctx context.Context, r model.Resource, embedding []float32) error
	SetMeta(ctx context.Context, key, value string) error
}

// Ingester is fetch-and-map for one Catalog.
type Ingester interface {
	CatalogID() string
	Fetch(ctx context.Context) (Batch, error)
}

// Batch is one ingest snapshot.
type Batch struct {
	Resources []model.Resource
	Version   string
	Origin    string // "json", "excel", "fixture", "file"
	CatalogID string
}

// Result counts upserts.
type Result struct {
	Seen     int
	Upserted int
	Skipped  int
	Origin   string
	Version  string
	Catalog  string
	Changed  []string // DocIDs whose ContentHash changed
}

// Runner hashes, embeds changed Resources, and upserts.
type Runner struct {
	Store    Store
	Embedder search.Embedder
	Notify   Notifier
	Source   string
	Log      *slog.Logger
}

func (r *Runner) log() *slog.Logger {
	if r.Log != nil {
		return r.Log
	}
	return slog.Default()
}

func (r *Runner) Apply(ctx context.Context, batch Batch) (Result, error) {
	res := Result{Seen: len(batch.Resources), Origin: batch.Origin, Version: batch.Version, Catalog: batch.CatalogID}
	for i := range batch.Resources {
		if batch.Resources[i].CatalogID == "" {
			if batch.CatalogID != "" {
				batch.Resources[i].CatalogID = batch.CatalogID
			} else {
				batch.Resources[i].CatalogID = model.CatalogBoverket
			}
		}
	}
	for _, item := range batch.Resources {
		if item.ResourceID == "" {
			res.Skipped++
			continue
		}
		h, err := item.ContentHash()
		if err != nil {
			return res, fmt.Errorf("hash %s: %w", item.ResourceID, err)
		}
		item.Hash = h
		prev, err := r.Store.Hash(ctx, item.CatalogID, item.ResourceID)
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
		res.Changed = append(res.Changed, item.DocID())
		if res.Catalog == "" {
			res.Catalog = item.CatalogID
		}
	}
	if batch.Version != "" {
		if err := r.Store.SetMeta(ctx, "dataset_version", batch.Version); err != nil {
			return res, err
		}
	}
	r.log().Info("ingest complete", "origin", batch.Origin, "catalog", res.Catalog, "version", batch.Version, "seen", res.Seen, "upserted", res.Upserted, "skipped", res.Skipped)
	r.notifyChanged(ctx, res)
	return res, nil
}

func (r *Runner) notifyChanged(ctx context.Context, res Result) {
	if r.Notify == nil || res.Upserted == 0 {
		return
	}
	src := r.Source
	if src == "" {
		src = "Boverket Klimatdatabas"
	}
	ev := Event{
		Event:        EventCatalogChanged,
		Catalog:      res.Catalog,
		Version:      res.Version,
		IngestOrigin: res.Origin,
		Seen:         res.Seen,
		Upserted:     res.Upserted,
		Skipped:      res.Skipped,
		IDs:          res.Changed,
		Source:       src,
		At:           time.Now().UTC(),
	}
	if err := r.Notify.Notify(ctx, ev); err != nil {
		r.log().Error("webhook catalog.changed failed", "err", err, "catalog", res.Catalog, "upserted", res.Upserted)
	}
}

func (r *Runner) Run(ctx context.Context, f Ingester) (Result, error) {
	batch, err := f.Fetch(ctx)
	if err != nil {
		return Result{}, err
	}
	if batch.CatalogID == "" {
		batch.CatalogID = f.CatalogID()
	}
	return r.Apply(ctx, batch)
}

// Loop runs ingest on interval. interval 0 means no ticker.
// Boot ingest is owned by main (--ingest-on-start), not Loop, so ingest is not run twice.
func Loop(ctx context.Context, r *Runner, f Ingester, interval time.Duration) {
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
	batch.CatalogID = model.CatalogBoverket
	if batch.Version == "" {
		batch.Version = "fixture"
	}
	return batch, nil
}
