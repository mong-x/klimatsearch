package ingest

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mong-x/klimatsearch/internal/config"
	"github.com/mong-x/klimatsearch/internal/model"
)

const maxBody = 32 << 20

const (
	resourcesSV = "/api/Klimat/v2/GetAllResources/latest/sv/json"
	resourcesEN = "/api/Klimat/v2/GetAllResources/latest/en/json"
)

// HTTPFetcher is the Boverket Ingester: OpenAPI v2 JSON, then Excel fallback.
type HTTPFetcher struct {
	Client          *http.Client
	APIBase         string
	SubscriptionKey string
	ExcelSV         string
	ExcelEN         string
}

func NewHTTPFetcher(cfg config.Config) *HTTPFetcher {
	base := cfg.BoverketAPIBase
	if base == "" {
		base = config.DefaultAPIBase
	}
	return &HTTPFetcher{
		Client:          &http.Client{Timeout: 45 * time.Second},
		APIBase:         strings.TrimRight(base, "/"),
		SubscriptionKey: cfg.BoverketSubscriptionKey,
		ExcelSV:         config.ExcelSV,
		ExcelEN:         config.ExcelEN,
	}
}

func (f *HTTPFetcher) CatalogID() string { return model.CatalogBoverket }

func (f *HTTPFetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return http.DefaultClient
}

func (f *HTTPFetcher) Fetch(ctx context.Context) (Batch, error) {
	batch, err := f.fetchJSON(ctx)
	if err == nil && len(batch.Resources) > 0 {
		return batch, nil
	}
	slog.Warn("json ingest failed, falling back to excel", "err", err, "resources", len(batch.Resources))
	return f.fetchExcel(ctx)
}

func (f *HTTPFetcher) fetchJSON(ctx context.Context) (Batch, error) {
	svURL := f.APIBase + resourcesSV
	enURL := f.APIBase + resourcesEN
	svBody, svErr := f.get(ctx, svURL)
	enBody, enErr := f.get(ctx, enURL)

	var sv, en Batch
	if svErr == nil {
		sv, svErr = ParseJSON(svBody)
	}
	if enErr == nil {
		en, enErr = ParseJSON(enBody)
	}
	if svErr != nil && enErr != nil {
		return Batch{}, fmt.Errorf("json sv: %v; json en: %v", svErr, enErr)
	}
	if svErr != nil {
		if len(en.Resources) == 0 {
			return Batch{}, fmt.Errorf("%s: empty resources (%v)", enURL, svErr)
		}
		stampBoverket(&en)
		en.Origin = "json"
		return en, nil
	}
	if enErr != nil {
		if len(sv.Resources) == 0 {
			return Batch{}, fmt.Errorf("%s: empty resources (%v)", svURL, enErr)
		}
		stampBoverket(&sv)
		sv.Origin = "json"
		return sv, nil
	}
	if len(sv.Resources) == 0 && len(en.Resources) == 0 {
		return Batch{}, fmt.Errorf("json api returned no resources")
	}
	merged := MergeLang(sv, en)
	merged.Origin = "json"
	stampBoverket(&merged)
	return merged, nil
}

func (f *HTTPFetcher) fetchExcel(ctx context.Context) (Batch, error) {
	svBody, err := f.get(ctx, f.ExcelSV)
	if err != nil {
		return Batch{}, fmt.Errorf("excel sv: %w", err)
	}
	sv, err := ParseExcel(svBody, "sv")
	if err != nil {
		return Batch{}, fmt.Errorf("excel sv parse: %w", err)
	}
	enBody, err := f.get(ctx, f.ExcelEN)
	if err != nil {
		stampBoverket(&sv)
		return sv, nil
	}
	en, err := ParseExcel(enBody, "en")
	if err != nil {
		stampBoverket(&sv)
		return sv, nil
	}
	merged := MergeLang(sv, en)
	stampBoverket(&merged)
	return merged, nil
}

func stampBoverket(b *Batch) {
	b.CatalogID = model.CatalogBoverket
	for i := range b.Resources {
		if b.Resources[i].CatalogID == "" {
			b.Resources[i].CatalogID = model.CatalogBoverket
		}
	}
}

func (f *HTTPFetcher) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, application/vnd.openxmlformats-officedocument.spreadsheetml.sheet, */*")
	req.Header.Set("User-Agent", "klimatsearch/0.1 (+https://github.com/mong-x/klimatsearch)")
	if f.SubscriptionKey != "" {
		req.Header.Set("Ocp-Apim-Subscription-Key", f.SubscriptionKey)
	}
	resp, err := f.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return body, nil
}

// FixtureFetcher loads a local JSON file.
type FixtureFetcher struct {
	Path string
}

func (f FixtureFetcher) CatalogID() string { return model.CatalogBoverket }

func (f FixtureFetcher) Fetch(context.Context) (Batch, error) {
	return LoadFixtureJSON(f.Path)
}

// StaticFetcher returns a fixed batch (tests).
type StaticFetcher struct {
	Batch Batch
	Err   error
}

func (f StaticFetcher) CatalogID() string {
	if f.Batch.CatalogID != "" {
		return f.Batch.CatalogID
	}
	return model.CatalogBoverket
}

func (f StaticFetcher) Fetch(context.Context) (Batch, error) {
	return f.Batch, f.Err
}
