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
	if !slackWebhook("https://hooks.slack.com/services/T000/B000/xxx") {
		t.Fatal("detect slack incoming url")
	}
	if slackWebhook("https://example.com/hook") {
		t.Fatal("generic url is not slack")
	}
	ev := Event{Event: EventCatalogUnreachable, Catalog: "boverket", Error: "timeout"}
	got, sign := slackBody(ev)
	if sign {
		t.Fatal("slack body is not HMAC'd")
	}
	var m map[string]string
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m["text"], "catalog.unreachable") || !strings.Contains(m["text"], "boverket") {
		t.Fatalf("text=%q", m["text"])
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
