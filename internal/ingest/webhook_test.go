package ingest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/store"
)

func TestWebhookFiresOnlyOnContentChange(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "wh.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())

	var hits atomic.Int32
	var last Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(EventHeader) != EventCatalogChanged {
			t.Errorf("event header=%s", r.Header.Get(EventHeader))
		}
		body, _ := io.ReadAll(r.Body)
		if !Valid("sekret", r.Header.Get(SignatureHeader), body) {
			t.Errorf("bad signature %s", r.Header.Get(SignatureHeader))
		}
		if err := json.Unmarshal(body, &last); err != nil {
			t.Error(err)
		}
		hits.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	r := &Runner{
		Store:    st,
		Embedder: fake,
		Notify:   NewHTTPWebhook(srv.URL, "sekret"),
		Source:   "Boverket Klimatdatabas",
	}
	batch := Batch{
		Origin:    "fixture",
		Version:   "1",
		CatalogID: model.CatalogBoverket,
		Resources: []model.Resource{{
			CatalogID: model.CatalogBoverket, ResourceID: "1", NameSV: "Betong", A1A3: 0.1, Unit: "kg",
		}},
	}
	res, err := r.Apply(t.Context(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if res.Upserted != 1 || hits.Load() != 1 {
		t.Fatalf("first apply upserted=%d hits=%d", res.Upserted, hits.Load())
	}
	if last.Event != EventCatalogChanged || last.Catalog != "boverket" || len(last.IDs) != 1 {
		t.Fatalf("event %+v", last)
	}
	if last.IDs[0] != "boverket:1" {
		t.Fatalf("id=%s", last.IDs[0])
	}

	res, err = r.Apply(t.Context(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped != 1 || hits.Load() != 1 {
		t.Fatalf("unchanged must not webhook: skipped=%d hits=%d", res.Skipped, hits.Load())
	}

	batch.Resources[0].A1A3 = 0.2
	res, err = r.Apply(t.Context(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if res.Upserted != 1 || hits.Load() != 2 {
		t.Fatalf("change should webhook: upserted=%d hits=%d", res.Upserted, hits.Load())
	}
}

func TestWebhookFailureDoesNotFailIngest(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "wh2.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	r := &Runner{Store: st, Embedder: fake, Notify: NewHTTPWebhook(srv.URL, "")}
	_, err = r.Apply(context.Background(), Batch{
		Resources: []model.Resource{{ResourceID: "1", NameSV: "x"}},
	})
	if err != nil {
		t.Fatalf("ingest must succeed: %v", err)
	}
}

func TestSignRoundTrip(t *testing.T) {
	body := []byte(`{"event":"catalog.changed"}`)
	sig := "sha256=" + Sign("s", body)
	if !Valid("s", sig, body) {
		t.Fatal("expected valid")
	}
	if Valid("other", sig, body) {
		t.Fatal("wrong secret")
	}
}

func TestRunUnreachableNotifiesAndStores(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "wh3.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var last Event
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &last)
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	r := &Runner{
		Store:  st,
		Notify: NewHTTPWebhook(srv.URL, ""),
		Events: LogTo(st),
	}
	_, err = r.Run(t.Context(), failFetch{err: &UnreachableError{Catalog: "boverket", Err: context.DeadlineExceeded}})
	if err == nil {
		t.Fatal("expected fetch error")
	}
	if hits.Load() != 1 || last.Event != EventCatalogUnreachable || last.Reason != ReasonUnreachable {
		t.Fatalf("hits=%d event=%+v", hits.Load(), last)
	}
	evs, err := st.ListIngestEvents(t.Context(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Event != EventCatalogUnreachable {
		t.Fatalf("stored %+v", evs)
	}
}

type failFetch struct{ err error }

func (failFetch) CatalogID() string                      { return model.CatalogBoverket }
func (f failFetch) Fetch(context.Context) (Batch, error) { return Batch{}, f.err }

func TestSlackIncomingWebhookShape(t *testing.T) {
	if HookKind("https://hooks.slack.com/services/T000/B000/xxx") != "slack" {
		t.Fatal("detect slack incoming url")
	}
	if HookKind("https://discord.com/api/webhooks/1/tok") != "discord" {
		t.Fatal("detect discord")
	}
	if HookKind("https://discord.com/api/webhooks/1/tok/slack") != "slack" {
		t.Fatal("discord slack-compat")
	}
	if HookKind("https://example.com/hook") != "json" {
		t.Fatal("generic url is json")
	}
	ev := Event{Event: EventCatalogUnreachable, Catalog: "boverket", Error: "timeout"}
	got := slackBody(ev)
	var m map[string]string
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m["text"], "catalog.unreachable") || !strings.Contains(m["text"], "boverket") {
		t.Fatalf("text=%q", m["text"])
	}
	d := discordBody(ev)
	if err := json.Unmarshal(d, &m); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m["content"], "catalog.unreachable") {
		t.Fatalf("content=%q", m["content"])
	}
	if len(SplitHookURLs("https://hooks.slack.com/a https://discord.com/api/webhooks/1/t")) != 2 {
		t.Fatal("split two destinations")
	}
}

func TestNewHTTPWebhookEmpty(t *testing.T) {
	h := NewHTTPWebhook("", "x")
	if h == nil {
		t.Fatal("constructor is always non-nil")
	}
	if err := h.Notify(t.Context(), Event{Event: EventCatalogChanged}); err != nil {
		t.Fatal(err)
	}
}

func TestWantsEvent(t *testing.T) {
	if !WantsEvent("", "catalog.changed") {
		t.Fatal("empty spec is all")
	}
	if !WantsEvent("catalog.unreachable", "catalog.unreachable") {
		t.Fatal("match")
	}
	if WantsEvent("catalog.changed", "ingest.failed") {
		t.Fatal("filter")
	}
	if !WantsEvent("catalog.changed", "webhook.test") {
		t.Fatal("test always allowed")
	}
}

// receiver captures the last delivery: raw body plus headers.
type receiver struct {
	body   []byte
	sig    string
	event  string
	hits   atomic.Int32
	server *httptest.Server
}

func newReceiver(t *testing.T) *receiver {
	t.Helper()
	rec := &receiver{}
	rec.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.body, _ = io.ReadAll(r.Body)
		rec.sig = r.Header.Get(SignatureHeader)
		rec.event = r.Header.Get(EventHeader)
		rec.hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(rec.server.Close)
	return rec
}

func TestNotifyPerHookSecretWithEmptyEnvSecret(t *testing.T) {
	rec := newReceiver(t)
	wh := NewHTTPWebhook("", "")
	wh.Hooks = HooksFunc(func(context.Context) ([]DeliveryHook, error) {
		return []DeliveryHook{{URL: rec.server.URL, Secret: "s1"}}, nil
	})
	if err := wh.Notify(t.Context(), Event{Event: EventCatalogChanged}); err != nil {
		t.Fatal(err)
	}
	if rec.hits.Load() != 1 {
		t.Fatalf("hits=%d", rec.hits.Load())
	}
	if !Valid("s1", rec.sig, rec.body) {
		t.Fatalf("expected signature under the hook's own secret, got %q", rec.sig)
	}
}

func TestNotifySignsEachTargetWithItsOwnSecret(t *testing.T) {
	alpha := newReceiver(t)
	gamma := newReceiver(t)
	envRec := newReceiver(t)
	wh := NewHTTPWebhook(envRec.server.URL, "beta")
	wh.Hooks = HooksFunc(func(context.Context) ([]DeliveryHook, error) {
		return []DeliveryHook{
			{URL: alpha.server.URL, Secret: "alpha"},
			{URL: gamma.server.URL, Secret: "gamma"},
		}, nil
	})
	if err := wh.Notify(t.Context(), Event{Event: EventCatalogChanged}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name   string
		rec    *receiver
		ownKey string
		otherA string
		otherB string
	}{
		{"alpha", alpha, "alpha", "gamma", "beta"},
		{"gamma", gamma, "gamma", "alpha", "beta"},
		{"env", envRec, "beta", "alpha", "gamma"},
	} {
		if !Valid(c.ownKey, c.rec.sig, c.rec.body) {
			t.Errorf("%s: not signed with its own key (%q)", c.name, c.rec.sig)
		}
		if Valid(c.otherA, c.rec.sig, c.rec.body) || Valid(c.otherB, c.rec.sig, c.rec.body) {
			t.Errorf("%s: signature verifies under a foreign key", c.name)
		}
	}
}

func TestPostHookSignsWithHookSecretNotEnvSecret(t *testing.T) {
	rec := newReceiver(t)
	wh := NewHTTPWebhook("", "env")
	if err := wh.PostHook(t.Context(), DeliveryHook{URL: rec.server.URL, Secret: "hook"}, Event{Event: "webhook.test"}); err != nil {
		t.Fatal(err)
	}
	if !Valid("hook", rec.sig, rec.body) {
		t.Fatalf("PostHook must sign with the hook secret, got %q", rec.sig)
	}
	if Valid("env", rec.sig, rec.body) {
		t.Fatal("PostHook must not sign with the process secret")
	}

	// Nil receiver stays usable; the hook secret signs on its own.
	rec2 := newReceiver(t)
	var nilWH *HTTPWebhook
	if err := nilWH.PostHook(t.Context(), DeliveryHook{URL: rec2.server.URL, Secret: "hook"}, Event{Event: "webhook.test"}); err != nil {
		t.Fatal(err)
	}
	if !Valid("hook", rec2.sig, rec2.body) {
		t.Fatalf("nil receiver PostHook signature=%q", rec2.sig)
	}

	// A secretless hook inherits the process secret.
	rec3 := newReceiver(t)
	if err := wh.PostHook(t.Context(), DeliveryHook{URL: rec3.server.URL}, Event{Event: "webhook.test"}); err != nil {
		t.Fatal(err)
	}
	if !Valid("env", rec3.sig, rec3.body) {
		t.Fatalf("empty hook secret must inherit the process secret, got %q", rec3.sig)
	}
}

func TestNotifyChatTargetNeverSigned(t *testing.T) {
	rec := newReceiver(t)
	url := "https://hooks.slack.com/services/T000/B000/x"
	wh := NewHTTPWebhook("", "")
	wh.Client = &http.Client{Transport: fakeRoundTrip{rec}}
	wh.Hooks = HooksFunc(func(context.Context) ([]DeliveryHook, error) {
		return []DeliveryHook{{URL: url, Secret: "s1"}}, nil
	})
	if err := wh.Notify(t.Context(), Event{Event: EventCatalogChanged, Catalog: "boverket"}); err != nil {
		t.Fatal(err)
	}
	if rec.sig != "" {
		t.Fatalf("chat targets are never signed, got %q", rec.sig)
	}
	var m map[string]string
	if err := json.Unmarshal(rec.body, &m); err != nil {
		t.Fatal(err)
	}
	if m["text"] == "" {
		t.Fatalf("slack body %s", rec.body)
	}
}

// fakeRoundTrip captures the request without hitting the real Slack edge.
type fakeRoundTrip struct{ rec *receiver }

func (f fakeRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	f.rec.body = body
	f.rec.sig = req.Header.Get(SignatureHeader)
	f.rec.event = req.Header.Get(EventHeader)
	f.rec.hits.Add(1)
	return &http.Response{StatusCode: 200, Status: "200 OK", Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header), Request: req}, nil
}

func TestNotifyHookWithoutSecretInheritsProcessSecret(t *testing.T) {
	rec := newReceiver(t)
	wh := NewHTTPWebhook("", "env")
	wh.Hooks = HooksFunc(func(context.Context) ([]DeliveryHook, error) {
		return []DeliveryHook{{URL: rec.server.URL}}, nil
	})
	if err := wh.Notify(t.Context(), Event{Event: EventCatalogChanged}); err != nil {
		t.Fatal(err)
	}
	if !Valid("env", rec.sig, rec.body) {
		t.Fatalf("secretless hook must inherit the process secret, got %q", rec.sig)
	}
}
