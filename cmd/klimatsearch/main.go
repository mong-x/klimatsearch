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
	"github.com/mong-x/klimatsearch/internal/guard"
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

	rr, err := reranker.New(cfg.Reranker, cfg.Models, cfg.RerankerModel)
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
	if wh := ingest.NewHTTPWebhook(cfg.WebhookURL, cfg.WebhookSecret); wh != nil {
		runner.Notify = wh
	}
	h := api.New(eng, st, cfg.Source)
	h.Runner = runner
	h.AdminToken = cfg.AdminToken
	h.Register(mux)

	useVector := st.VectorEnabled()
	useRerank := cfg.Reranker != "none"
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

	log.Info("listening", "addr", cfg.Listen, "embedder", cfg.Embedder, "reranker", cfg.Reranker, "db", cfg.DB, "hosted", cfg.Hosted(), "guard_allow_all", g.AllowAll)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("http", "err", err)
		os.Exit(1)
	}
}
