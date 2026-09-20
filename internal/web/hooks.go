package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mong-x/klimatsearch/internal/ingest"
	"github.com/mong-x/klimatsearch/internal/store"
)

var errHookURL = errors.New("webhook URL is required")

func (h *Handler) webhooksGet(w http.ResponseWriter, r *http.Request) {
	p := h.hookPage(r)
	h.render(w, "webhooks", p)
}

func (h *Handler) webhooksPost(w http.ResponseWriter, r *http.Request) {
	p := h.hookPage(r)
	if !h.hookAuth(r) {
		p.Error = "unauthorized"
		p.Status = http.StatusUnauthorized
		h.render(w, "webhooks", p)
		return
	}
	if h.Store == nil {
		p.Error = "store is not configured"
		p.Status = http.StatusServiceUnavailable
		h.render(w, "webhooks", p)
		return
	}
	if err := r.ParseForm(); err != nil {
		p.Error = "invalid form"
		p.Status = http.StatusBadRequest
		h.render(w, "webhooks", p)
		return
	}
	action := r.FormValue("action")
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	ctx := r.Context()
	var err error
	switch action {
	case "add":
		events := strings.Join(r.Form["events"], ",")
		secret := r.FormValue("secret")
		urls := ingest.SplitHookURLs(r.FormValue("url"))
		if len(urls) == 0 {
			err = errHookURL
			break
		}
		for _, u := range urls {
			if _, e := h.Store.AddWebhook(ctx, u, secret, events); e != nil {
				err = e
				break
			}
		}
	case "delete":
		err = h.Store.DeleteWebhook(ctx, id)
	case "enable":
		err = h.Store.SetWebhookEnabled(ctx, id, true)
	case "disable":
		err = h.Store.SetWebhookEnabled(ctx, id, false)
	case "test":
		err = h.testHook(r, id)
	default:
		p.Error = "unknown action"
		p.Status = http.StatusBadRequest
		h.render(w, "webhooks", p)
		return
	}
	if err != nil {
		p.Error = err.Error()
		p.Status = http.StatusBadRequest
		p.Hooks, p.EnvHooks = h.loadHooks(r)
		h.render(w, "webhooks", p)
		return
	}
	http.Redirect(w, r, "/webhooks", http.StatusSeeOther)
}

func (h *Handler) hookPage(r *http.Request) page {
	p := h.base(r, "webhooks", "Webhooks")
	p.Hooks, p.EnvHooks = h.loadHooks(r)
	return p
}

func (h *Handler) loadHooks(r *http.Request) ([]store.Webhook, []string) {
	var env []string
	if h.Runner != nil {
		if wh, ok := h.Runner.Notify.(*ingest.HTTPWebhook); ok {
			env = append(env, wh.URLs...)
		}
	}
	if h.Store == nil {
		return nil, env
	}
	rows, err := h.Store.ListWebhooks(r.Context())
	if err != nil {
		return nil, env
	}
	for i := range rows {
		rows[i].Kind = ingest.HookKind(rows[i].URL)
	}
	return rows, env
}

func (h *Handler) hookAuth(r *http.Request) bool {
	if h.AdminToken == "" {
		return true
	}
	tok := strings.TrimSpace(r.Header.Get("X-Admin-Token"))
	if tok == "" {
		tok = strings.TrimSpace(r.FormValue("token"))
	}
	return tok == h.AdminToken
}

func (h *Handler) testHook(r *http.Request, id int64) error {
	if h.Store == nil {
		return errNotConfigured
	}
	w, err := h.Store.GetWebhook(r.Context(), id)
	if err != nil {
		return err
	}
	secret := w.Secret
	var wh *ingest.HTTPWebhook
	if h.Runner != nil {
		wh, _ = h.Runner.Notify.(*ingest.HTTPWebhook)
		if secret == "" && wh != nil {
			secret = wh.Secret
		}
	}
	if wh == nil {
		wh = ingest.NewHTTPWebhook("", secret)
	}
	ev := ingest.Event{
		Event:  "webhook.test",
		Source: h.Source,
		At:     time.Now().UTC(),
	}
	return wh.PostURL(r.Context(), w.URL, secret, ev)
}
