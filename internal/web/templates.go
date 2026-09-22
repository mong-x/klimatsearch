package web

import (
	"github.com/mong-x/klimatsearch/internal/ingest"

	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed templates/*.html static/*
var assets embed.FS

type pageTmpl struct {
	t *template.Template
}

func mustParse() map[string]*pageTmpl {
	pages := []string{"search", "resource", "compare", "connect", "ingest", "data", "webhooks"}
	out := make(map[string]*pageTmpl, len(pages))
	for _, name := range pages {
		t, err := template.New("").Funcs(template.FuncMap{
			"active": func(cur, want string) string {
				if cur == want {
					return "is-active"
				}
				return ""
			},
			"wantsEvent": ingest.WantsEvent,
		}).ParseFS(assets, "templates/layout.html", "templates/"+name+".html")
		if err != nil {
			panic(err)
		}
		out[name] = &pageTmpl{t: t}
	}
	return out
}

func mustStatic() fs.FS {
	sub, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	return sub
}

func (h *Handler) render(w http.ResponseWriter, name string, p page) {
	pt := h.pages[name]
	if pt == nil {
		http.Error(w, "template missing", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := pt.t.ExecuteTemplate(&buf, "layout", p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	status := p.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
