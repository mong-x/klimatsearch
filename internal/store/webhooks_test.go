package store

import "testing"

func TestWebhookCRUD(t *testing.T) {
	st := testStore(t)
	ctx := t.Context()
	w, err := st.AddWebhook(ctx, "https://hooks.slack.com/services/T/B/x", "", "catalog.unreachable")
	if err != nil {
		t.Fatal(err)
	}
	if w.ID == 0 {
		t.Fatal("id")
	}
	_, err = st.AddWebhook(ctx, "not-a-url", "", "")
	if err == nil {
		t.Fatal("expected bad url")
	}
	list, err := st.ListWebhooks(ctx)
	if err != nil || len(list) != 1 || list[0].HasSecret {
		t.Fatalf("%+v %v", list, err)
	}
	if err := st.SetWebhookEnabled(ctx, w.ID, false); err != nil {
		t.Fatal(err)
	}
	deliv, err := st.DeliveryWebhooks(ctx)
	if err != nil || len(deliv) != 0 {
		t.Fatalf("disabled still delivered: %+v", deliv)
	}
	if err := st.DeleteWebhook(ctx, w.ID); err != nil {
		t.Fatal(err)
	}
}
