package store

import (
	"testing"
)

func TestAppendIngestEventPrunes(t *testing.T) {
	st := testStore(t)
	ctx := t.Context()
	for i := 0; i < maxIngestEvents+5; i++ {
		if err := st.AppendIngestEvent(ctx, "ingest.failed", "boverket", "x", "{}"); err != nil {
			t.Fatal(err)
		}
	}
	evs, err := st.ListIngestEvents(ctx, 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != maxIngestEvents {
		t.Fatalf("len=%d want %d", len(evs), maxIngestEvents)
	}
}
