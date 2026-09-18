package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultListen         = ":8080"
	DefaultDB             = "./data/klimat.db"
	DefaultModels         = "./models"
	DefaultEmbeddingModel = "f2llm-v2-80m"
	DefaultRerankerModel  = "bge-reranker-v2-m3"
	DefaultONNXQuant      = "auto"
	DefaultSource         = "Boverket Klimatdatabas"
	DefaultAPIBase        = "https://api.boverket.se/klimatdatabas"
	DefaultIngestInterval = 168 * time.Hour
	Attribution           = "Boverket Klimatdatabas"
	ExcelSV               = "https://www.boverket.se/contentassets/4668ed4cc3da447385788ed30bff7d49/boverkets-klimatdatabas-version-02.07.000-sv-se.xlsx"
	ExcelEN               = "https://www.boverket.se/contentassets/4668ed4cc3da447385788ed30bff7d49/boverkets-climate-database-version-02.07.000-en-gb.xlsx"
	DefaultFixtureJSON    = "testdata/fixtures/resources.json"
)

// Config is process configuration from flags and environment.
type Config struct {
	Listen                  string
	DB                      string
	Models                  string
	Embedder                string
	Reranker                string
	EmbeddingModel          string
	RerankerModel           string
	ONNXQuant               string
	IngestOnStart           bool
	IngestInterval          time.Duration
	BoverketAPIBase         string
	BoverketSubscriptionKey string
	Source                  string
	DemoFixture             bool
	FixturePath             string
	AdminToken              string
	WebhookURL              string
	WebhookSecret           string
	hosted
}

// Parse reads flags and env. Flag values win over env over defaults.
// A gitignored `.env` in the working directory is loaded first (does not
// override variables already set in the process environment).
func Parse(args []string) (Config, error) {
	loadDotEnv(".env")
	c := Config{
		Listen:                  env("KLIMAT_LISTEN", DefaultListen),
		DB:                      env("KLIMAT_DB", DefaultDB),
		Models:                  env("KLIMAT_MODELS", DefaultModels),
		Embedder:                env("KLIMAT_EMBEDDER", "auto"),
		Reranker:                env("KLIMAT_RERANKER", "none"),
		EmbeddingModel:          env("KLIMAT_EMBEDDING_MODEL", DefaultEmbeddingModel),
		RerankerModel:           env("KLIMAT_RERANKER_MODEL", DefaultRerankerModel),
		ONNXQuant:               env("KLIMAT_ONNX_QUANT", DefaultONNXQuant),
		IngestOnStart:           envBool("KLIMAT_INGEST_ON_START", true),
		IngestInterval:          DefaultIngestInterval,
		BoverketAPIBase:         env("BOVERKET_API_BASE", env("KLIMAT_BOVERKET_API_BASE", DefaultAPIBase)),
		BoverketSubscriptionKey: env("BOVERKET_SUBSCRIPTION_KEY", ""),
		Source:                  env("KLIMAT_SOURCE", DefaultSource),
		DemoFixture:             envBool("KLIMAT_DEMO_FIXTURE", false),
		FixturePath:             env("KLIMAT_FIXTURE_PATH", DefaultFixtureJSON),
		AdminToken:              env("KLIMAT_ADMIN_TOKEN", ""),
		WebhookURL:              env("KLIMAT_WEBHOOK_URL", ""),
		WebhookSecret:           env("KLIMAT_WEBHOOK_SECRET", ""),
	}
	loadHosted(&c)
	if v := os.Getenv("KLIMAT_INGEST_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("KLIMAT_INGEST_INTERVAL: %w", err)
		}
		c.IngestInterval = d
	}

	fs := flag.NewFlagSet("klimatsearch", flag.ContinueOnError)
	fs.StringVar(&c.Listen, "listen", c.Listen, "HTTP listen address (`KLIMAT_LISTEN`)")
	fs.StringVar(&c.DB, "db", c.DB, "SQLite path (`KLIMAT_DB`)")
	fs.StringVar(&c.Models, "models", c.Models, "ONNX models directory (`KLIMAT_MODELS`)")
	fs.StringVar(&c.Embedder, "embedder", c.Embedder, "embedder: auto|fake|onnx (`KLIMAT_EMBEDDER`)")
	fs.StringVar(&c.Reranker, "reranker", c.Reranker, "reranker: none|fake|onnx (`KLIMAT_RERANKER`)")
	fs.StringVar(&c.EmbeddingModel, "embedding-model", c.EmbeddingModel, "embedder directory under --models")
	fs.StringVar(&c.RerankerModel, "reranker-model", c.RerankerModel, "reranker directory under --models (`KLIMAT_RERANKER_MODEL`)")
	fs.StringVar(&c.ONNXQuant, "onnx-quant", c.ONNXQuant, "onnx file pick: auto|int8|fp32 (`KLIMAT_ONNX_QUANT`)")
	fs.BoolVar(&c.IngestOnStart, "ingest-on-start", c.IngestOnStart, "run ingest once at boot")
	fs.DurationVar(&c.IngestInterval, "ingest-interval", c.IngestInterval, "repeat ingest interval (0 disables ticker)")
	fs.StringVar(&c.BoverketAPIBase, "boverket-api-base", c.BoverketAPIBase, "Boverket APIM base URL")
	fs.StringVar(&c.BoverketSubscriptionKey, "boverket-subscription-key", c.BoverketSubscriptionKey, "APIM subscription key")
	fs.StringVar(&c.Source, "source", c.Source, "attribution string in JSON responses")
	fs.BoolVar(&c.DemoFixture, "demo-fixture", c.DemoFixture, "ingest testdata fixtures, no network")
	fs.StringVar(&c.FixturePath, "fixture-path", c.FixturePath, "JSON fixture path when --demo-fixture")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	c.Embedder = strings.ToLower(strings.TrimSpace(c.Embedder))
	c.Reranker = strings.ToLower(strings.TrimSpace(c.Reranker))
	c.ONNXQuant = strings.ToLower(strings.TrimSpace(c.ONNXQuant))
	if c.ONNXQuant == "" {
		c.ONNXQuant = DefaultONNXQuant
	}
	switch c.ONNXQuant {
	case "auto", "int8", "fp32", "none":
	default:
		return Config{}, fmt.Errorf("invalid --onnx-quant %q (auto|int8|fp32)", c.ONNXQuant)
	}
	_ = os.Setenv("KLIMAT_ONNX_QUANT", c.ONNXQuant)
	switch c.Embedder {
	case "auto", "fake", "onnx":
	default:
		return Config{}, fmt.Errorf("invalid --embedder %q (auto|fake|onnx)", c.Embedder)
	}
	switch c.Reranker {
	case "none", "fake", "onnx":
	default:
		return Config{}, fmt.Errorf("invalid --reranker %q (none|fake|onnx)", c.Reranker)
	}
	return c, nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func loadDotEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if key == "" {
			continue
		}
		if _, set := os.LookupEnv(key); set {
			continue
		}
		_ = os.Setenv(key, val)
	}
}

func envBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
