package ingest

import (
	"context"
	"encoding/json"
	"errors"
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
	MissingVectors(ctx context.Context) ([]model.Resource, error)
	PutVector(ctx context.Context, catalogID, id string, embedding []float32) error
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
	Files    *FileStash
	Embedder search.Embedder
	Notify   Notifier
	Events   EventLog
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
		if item.Version == "" && batch.Version != "" {
			item.Version = batch.Version
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
	if batch.Version != "" && res.Catalog != "" {
		if err := r.Store.SetMeta(ctx, model.DatasetVersionMetaKey(res.Catalog), batch.Version); err != nil {
			return res, err
		}
	}
	r.log().Info("ingest complete", "origin", batch.Origin, "catalog", res.Catalog, "version", batch.Version, "seen", res.Seen, "upserted", res.Upserted, "skipped", res.Skipped)
	r.notifyChanged(ctx, res)
	return res, nil
}

func (r *Runner) notifyChanged(ctx context.Context, res Result) {
	if res.Upserted == 0 {
		return
	}
	r.emit(ctx, Event{
		Event:        EventCatalogChanged,
		Catalog:      res.Catalog,
		Version:      res.Version,
		IngestOrigin: res.Origin,
		Seen:         res.Seen,
		Upserted:     res.Upserted,
		Skipped:      res.Skipped,
		IDs:          res.Changed,
		Source:       r.Source,
		At:           time.Now().UTC(),
	})
}

func (r *Runner) fail(ctx context.Context, catalog string, err error, reason string) {
	if err == nil {
		return
	}
	if reason == "" {
		reason = ReasonFetch
	}
	ev := Event{
		Event:   EventIngestFailed,
		Catalog: catalog,
		Error:   err.Error(),
		Reason:  reason,
		Source:  r.Source,
		At:      time.Now().UTC(),
	}
	var u *UnreachableError
	if errors.As(err, &u) {
		ev.Event = EventCatalogUnreachable
		ev.Reason = ReasonUnreachable
		if ev.Catalog == "" && u != nil {
			ev.Catalog = u.Catalog
		}
	}
	r.emit(ctx, ev)
}

func (r *Runner) emit(ctx context.Context, ev Event) {
	if r.Events != nil {
		if err := r.Events.AppendEvent(ctx, ev); err != nil {
			r.log().Error("persist ingest event", "err", err, "event", ev.Event)
		}
	}
	if r.Notify == nil {
		return
	}
	if err := r.Notify.Notify(ctx, ev); err != nil {
		r.log().Error("webhook failed", "err", err, "event", ev.Event, "catalog", ev.Catalog)
	}
}

// BackfillVectors embeds Resources whose vector row is missing and writes
// the vectors without touching ContentHash (the content did not change).
// This is the repair path for embedder-less ingest and for a model-dim
// change after meta.embedding_dim is migrated.
func (r *Runner) BackfillVectors(ctx context.Context) (int, error) {
	if r.Store == nil {
		return 0, errors.New("store is not configured")
	}
	if r.Embedder == nil {
		return 0, fmt.Errorf("backfill requires an embedder")
	}
	missing, err := r.Store.MissingVectors(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, item := range missing {
		vec, err := r.Embedder.Embed(item.EmbeddingText())
		if err != nil {
			return n, fmt.Errorf("embed %s: %w", item.DocID(), err)
		}
		if err := r.Store.PutVector(ctx, item.CatalogID, item.ResourceID, vec); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (r *Runner) Run(ctx context.Context, f Ingester) (Result, error) {
	batch, err := f.Fetch(ctx)
	if err != nil {
		cat := ""
		if f != nil {
			cat = f.CatalogID()
		}
		r.fail(ctx, cat, err, ReasonFetch)
		return Result{}, err
	}
	if batch.CatalogID == "" {
		batch.CatalogID = f.CatalogID()
	}
	res, err := r.Apply(ctx, batch)
	if err != nil {
		r.fail(ctx, batch.CatalogID, err, ReasonApply)
	}
	return res, err
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
