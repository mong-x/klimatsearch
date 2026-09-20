package ingest

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	EventCatalogChanged = "catalog.changed"
	SignatureHeader     = "X-Klimat-Signature"
	EventHeader         = "X-Klimat-Event"
)

// Event is posted on ingest content change or ingest failure.
type Event struct {
	Event        string    `json:"event"`
	Catalog      string    `json:"catalog"`
	Version      string    `json:"version,omitempty"`
	IngestOrigin string    `json:"ingest_origin,omitempty"`
	Seen         int       `json:"seen,omitempty"`
	Upserted     int       `json:"upserted,omitempty"`
	Skipped      int       `json:"skipped,omitempty"`
	IDs          []string  `json:"ids,omitempty"`
	Source       string    `json:"source"`
	At           time.Time `json:"at"`
	Error        string    `json:"error,omitempty"`
	Reason       string    `json:"reason,omitempty"`
}

// Notifier is called after ingest changes rows or ingest fails.
type Notifier interface {
	Notify(ctx context.Context, ev Event) error
}

// EventLog persists Events (bounded SQLite log).
type EventLog interface {
	AppendEvent(ctx context.Context, ev Event) error
}

// HTTPWebhook POSTs JSON to one or more URLs. Secret, if set, HMAC-SHA256s the body.
type HTTPWebhook struct {
	URLs   []string
	Secret string
	Client *http.Client
}

// NewHTTPWebhook parses a comma-separated URL list. Empty input is a no-op notifier.
func NewHTTPWebhook(urls, secret string) *HTTPWebhook {
	var out []string
	for _, u := range strings.Split(urls, ",") {
		u = strings.TrimSpace(u)
		if u != "" {
			out = append(out, u)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return &HTTPWebhook{
		URLs:   out,
		Secret: strings.TrimSpace(secret),
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (h *HTTPWebhook) client() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (h *HTTPWebhook) Notify(ctx context.Context, ev Event) error {
	if h == nil || len(h.URLs) == 0 {
		return nil
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	var first error
	for _, u := range h.URLs {
		if err := h.post(ctx, u, body, ev.Event); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (h *HTTPWebhook) post(ctx context.Context, rawURL string, body []byte, event string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "klimatsearch")
	req.Header.Set(EventHeader, event)
	if h.Secret != "" {
		req.Header.Set(SignatureHeader, "sha256="+Sign(h.Secret, body))
	}
	resp, err := h.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: HTTP %d", rawURL, resp.StatusCode)
	}
	return nil
}

// Sign returns the hex HMAC-SHA256 of body.
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// Valid reports whether header is sha256=<hex> for secret and body.
func Valid(secret, header string, body []byte) bool {
	header = strings.TrimSpace(header)
	const p = "sha256="
	if !strings.HasPrefix(strings.ToLower(header), p) {
		return false
	}
	got, err := hex.DecodeString(strings.TrimSpace(header[len(p):]))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}
