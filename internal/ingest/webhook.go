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

// DeliveryHook is one outbound URL (env or SQLite).
type DeliveryHook struct {
	URL    string
	Secret string
	Events string // empty = all events
}

// HookSource is SQLite-backed operator webhooks.
type HookSource interface {
	DeliveryHooks(ctx context.Context) ([]DeliveryHook, error)
}

// HooksFunc adapts a function to HookSource.
type HooksFunc func(ctx context.Context) ([]DeliveryHook, error)

func (f HooksFunc) DeliveryHooks(ctx context.Context) ([]DeliveryHook, error) {
	if f == nil {
		return nil, nil
	}
	return f(ctx)
}

// HTTPWebhook POSTs JSON (or Slack text) to env URLs plus HookSource.
type HTTPWebhook struct {
	URLs   []string
	Secret string
	Client *http.Client
	Hooks  HookSource
}

// NewHTTPWebhook parses a comma-separated URL list. Always non-nil so a HookSource can be attached.
func NewHTTPWebhook(urls, secret string) *HTTPWebhook {
	var out []string
	for _, u := range strings.Split(urls, ",") {
		u = strings.TrimSpace(u)
		if u != "" {
			out = append(out, u)
		}
	}
	return &HTTPWebhook{
		URLs:   out,
		Secret: strings.TrimSpace(secret),
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

func WantsEvent(spec, event string) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" || event == "webhook.test" {
		return true
	}
	for _, p := range strings.Split(spec, ",") {
		if strings.TrimSpace(p) == event {
			return true
		}
	}
	return false
}

func (h *HTTPWebhook) client() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (h *HTTPWebhook) Notify(ctx context.Context, ev Event) error {
	if h == nil {
		return nil
	}
	targets := h.targets(ctx, ev.Event)
	if len(targets) == 0 {
		return nil
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	var first error
	for _, t := range targets {
		if err := h.deliver(ctx, shape(t, ev, body)); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (h *HTTPWebhook) targets(ctx context.Context, event string) []DeliveryHook {
	var out []DeliveryHook
	for _, u := range h.URLs {
		out = append(out, DeliveryHook{URL: u, Secret: h.Secret})
	}
	if h.Hooks == nil {
		return out
	}
	extra, err := h.Hooks.DeliveryHooks(ctx)
	if err != nil {
		return out
	}
	for _, t := range extra {
		if t.URL == "" || !WantsEvent(t.Events, event) {
			continue
		}
		if t.Secret == "" {
			t.Secret = h.Secret
		}
		out = append(out, t)
	}
	return out
}

// delivery is one shaped outbound POST. The signing secret travels with the
// body it covers; secret "" means unsigned.
type delivery struct {
	url    string
	body   []byte
	event  string
	secret string
}

// shape builds the POST for one target. Chat targets (Slack, Discord) are
// never signed: their webhook URL is itself the bearer credential, and an
// HMAC header would expose a shared secret to a third-party edge.
func shape(t DeliveryHook, ev Event, canonical []byte) delivery {
	d := delivery{url: t.URL, body: canonical, event: ev.Event, secret: t.Secret}
	switch HookKind(t.URL) {
	case "slack":
		d.body, d.secret = slackBody(ev), ""
	case "discord":
		d.body, d.secret = discordBody(ev), ""
	}
	return d
}

// PostHook sends one Event to a single hook (console Test). It applies the
// same inheritance as scheduled delivery: an empty hook secret inherits the
// process secret.
func (h *HTTPWebhook) PostHook(ctx context.Context, hook DeliveryHook, ev Event) error {
	if h == nil {
		h = &HTTPWebhook{}
	}
	if hook.Secret == "" {
		hook.Secret = h.Secret
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return h.deliver(ctx, shape(hook, ev, body))
}

// HookKind is slack, discord, or json.
func HookKind(rawURL string) string {
	u := strings.ToLower(rawURL)
	if strings.Contains(u, "hooks.slack.com") {
		return "slack"
	}
	if strings.Contains(u, "discord.com/api/webhooks") || strings.Contains(u, "discordapp.com/api/webhooks") {
		if strings.HasSuffix(strings.TrimSuffix(u, "/"), "/slack") {
			return "slack"
		}
		return "discord"
	}
	return "json"
}

func discordBody(ev Event) []byte {
	b, _ := json.Marshal(map[string]string{"content": chatText(ev)})
	return b
}

func slackBody(ev Event) []byte {
	b, _ := json.Marshal(map[string]string{"text": chatText(ev)})
	return b
}

func chatText(ev Event) string {
	var b strings.Builder
	b.WriteString("klimatsearch ")
	b.WriteString(ev.Event)
	if ev.Catalog != "" {
		b.WriteString(" ")
		b.WriteString(ev.Catalog)
	}
	if ev.Version != "" {
		b.WriteString(" version=")
		b.WriteString(ev.Version)
	}
	if ev.Upserted > 0 {
		fmt.Fprintf(&b, " upserted=%d", ev.Upserted)
	}
	if ev.Error != "" {
		b.WriteString(" — ")
		b.WriteString(ev.Error)
	}
	return b.String()
}

// SplitHookURLs splits a paste of several destinations (newline, comma, or space).
func SplitHookURLs(raw string) []string {
	raw = strings.ReplaceAll(raw, ",", "\n")
	var out []string
	seen := map[string]bool{}
	for _, p := range strings.Fields(raw) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func (h *HTTPWebhook) deliver(ctx context.Context, d delivery) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url, bytes.NewReader(d.body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "klimatsearch")
	req.Header.Set(EventHeader, d.event)
	if d.secret != "" {
		req.Header.Set(SignatureHeader, "sha256="+Sign(d.secret, d.body))
	}
	resp, err := h.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: HTTP %d", d.url, resp.StatusCode)
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
