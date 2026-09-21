package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mong-x/klimatsearch/internal/api"
	"github.com/mong-x/klimatsearch/internal/config"
	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/guard"
	"github.com/mong-x/klimatsearch/internal/ingest"
	mcpserver "github.com/mong-x/klimatsearch/internal/mcp"
	"github.com/mong-x/klimatsearch/internal/reranker"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
	"github.com/mong-x/klimatsearch/internal/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(log)

	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(2)
	}

	st, err := store.Open(cfg.DB)
	if err != nil {
		log.Error("store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	emb, err := embedder.New(cfg.Embedder, cfg.Models, cfg.EmbeddingModel, cfg.ONNXQuant)
	if err != nil {
		log.Error("embedder", "err", err)
		os.Exit(1)
	}
	st.ConfigureVector(embedder.DimOf(emb))

	rr, err := reranker.New(cfg.Reranker, cfg.Models, cfg.RerankerModel, cfg.ONNXQuant)
	if err != nil {
		log.Error("reranker", "err", err)
		os.Exit(1)
	}

	g, err := guard.New(cfg)
	if err != nil {
		log.Error("guard", "err", err)
		os.Exit(1)
	}

	eng := search.New(st, st, emb, rr)
	mux := http.NewServeMux()
	runner := &ingest.Runner{Store: st, Embedder: emb, Log: log, Source: cfg.Source}
	wh := ingest.NewHTTPWebhook(cfg.WebhookURL, cfg.WebhookSecret)
	wh.Hooks = ingest.HooksFunc(func(ctx context.Context) ([]ingest.DeliveryHook, error) {
		rows, err := st.DeliveryWebhooks(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]ingest.DeliveryHook, 0, len(rows))
		for _, r := range rows {
			out = append(out, ingest.DeliveryHook{URL: r.URL, Secret: r.Secret, Events: r.Events})
		}
		return out, nil
	})
	runner.Notify = wh
	runner.Events = ingest.LogTo(st)
	h := api.New(eng, st, cfg.Source)
	h.Runner = runner
	h.AdminToken = cfg.AdminToken
	h.Register(mux)

	useVector := st.VectorEnabled()
	useRerank := cfg.Reranker != "none"
	quant := cfg.ONNXQuant
	if p, err := embedder.ResolveONNX(filepath.Join(cfg.Models, cfg.EmbeddingModel), cfg.ONNXQuant); err == nil {
		quant = embedder.QuantLabel(p)
	}
	ui := web.New(eng, st, cfg.Source, web.Status{
		Tools:    []string{mcpserver.ToolSearch, mcpserver.ToolGet, mcpserver.ToolCompare},
		Vector:   useVector,
		Rerank:   useRerank,
		Embedder: cfg.Embedder,
		Reranker: cfg.Reranker,
		Quant:    quant,
	})
	ui.Runner = runner
	ui.AdminToken = cfg.AdminToken
	ui.Register(mux)
	mcpserver.New(eng, st, cfg.Source, useVector, useRerank).Mount(mux)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var fetcher ingest.Ingester
	if cfg.DemoFixture {
		fetcher = ingest.FixtureFetcher{Path: cfg.FixturePath}
	} else {
		fetcher = ingest.NewHTTPFetcher(cfg)
	}
	if cfg.IngestOnStart {
		if _, err := runner.Run(ctx, fetcher); err != nil {
			log.Error("ingest on start failed", "err", err)
			if cfg.DemoFixture {
				os.Exit(1)
			}
		}
	}
	go ingest.Loop(ctx, runner, fetcher, cfg.IngestInterval)

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           g.Wrap(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shctx)
	}()

	log.Info("listening", "addr", cfg.Listen, "embedder", cfg.Embedder, "reranker", cfg.Reranker, "db", cfg.DB, "hosted", cfg.Hosted(), "guard_allow_all", g.AllowAll, "api_keys", len(cfg.APIKeys))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("http", "err", err)
		os.Exit(1)
	}
}
