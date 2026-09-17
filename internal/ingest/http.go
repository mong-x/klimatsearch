package ingest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mong-x/klimatsearch/internal/config"
)

const maxBody = 32 << 20

// HTTPFetcher probes the Boverket JSON API then falls back to Excel.
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

func (f *HTTPFetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return http.DefaultClient
}

func (f *HTTPFetcher) Fetch(ctx context.Context) (Batch, error) {
	if batch, err := f.fetchJSON(ctx); err == nil && len(batch.Resources) > 0 {
		return batch, nil
	} else if err != nil {
		// continue to Excel
		_ = err
	}
	return f.fetchExcel(ctx)
}

func (f *HTTPFetcher) fetchJSON(ctx context.Context) (Batch, error) {
	paths := []string{
		"/klimatdatabas/v1/resources",
		"/klimatdatabas/v1/resources?lang=sv",
		"/klimatdatabas/resources",
		"/api/klimatdatabas/v1/resources",
		"/klimatdatabas/v1/Resources",
	}
	var last error
	for _, p := range paths {
		u := f.APIBase + p
		body, err := f.get(ctx, u)
		if err != nil {
			last = err
			continue
		}
		batch, err := ParseJSON(body)
		if err != nil {
			last = fmt.Errorf("%s: %w", u, err)
			continue
		}
		if len(batch.Resources) == 0 {
			last = fmt.Errorf("%s: empty resources", u)
			continue
		}
		batch.Origin = "json"
		return batch, nil
	}
	if last == nil {
		last = fmt.Errorf("json api returned no resources")
	}
	return Batch{}, last
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
		return sv, nil
	}
	en, err := ParseExcel(enBody, "en")
	if err != nil {
		return sv, nil
	}
	return MergeLang(sv, en), nil
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

func (f FixtureFetcher) Fetch(context.Context) (Batch, error) {
	return LoadFixtureJSON(f.Path)
}

// StaticFetcher returns a fixed batch (tests).
type StaticFetcher struct {
	Batch Batch
	Err   error
}

func (f StaticFetcher) Fetch(context.Context) (Batch, error) {
	return f.Batch, f.Err
}
