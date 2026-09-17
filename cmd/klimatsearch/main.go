package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mong-x/klimatsearch/internal/api"
	"github.com/mong-x/klimatsearch/internal/config"
	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/ingest"
	mcpserver "github.com/mong-x/klimatsearch/internal/mcp"
	"github.com/mong-x/klimatsearch/internal/reranker"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
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

	emb, err := embedder.New(cfg.Embedder, cfg.Models, cfg.EmbeddingModel)
	if err != nil {
		log.Error("embedder", "err", err)
		os.Exit(1)
	}
	st.ConfigureVector(embedder.DimOf(emb))

	rr, err := reranker.New(cfg.Reranker, cfg.Models, cfg.EmbeddingModel)
	if err != nil {
		log.Error("reranker", "err", err)
		os.Exit(1)
	}

	eng := search.New(st, st, emb, rr)
	mux := http.NewServeMux()
	api.New(eng, st, cfg.Source).Register(mux)

	useVector := st.VectorEnabled()
	useRerank := cfg.Reranker != "none"
	mcpserver.New(eng, st, cfg.Source, useVector, useRerank).Mount(mux)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runner := &ingest.Runner{Store: st, Embedder: emb, Log: log}
	var fetcher ingest.Fetcher
	if cfg.DemoFixture {
		fetcher = ingest.FixtureFetcher{Path: cfg.FixturePath}
	} else {
		fetcher = ingest.NewHTTPFetcher(cfg)
	}
	go ingest.Loop(ctx, runner, fetcher, cfg.IngestOnStart, cfg.IngestInterval)

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shctx)
	}()

	log.Info("listening", "addr", cfg.Listen, "embedder", cfg.Embedder, "reranker", cfg.Reranker, "db", cfg.DB)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("http", "err", err)
		os.Exit(1)
	}
}
