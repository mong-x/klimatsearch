package web

import (
	"errors"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/mong-x/klimatsearch/internal/config"
	"github.com/mong-x/klimatsearch/internal/ingest"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
)

// Status is process facts the operator rail shows (MCP, retrieval defaults).
type Status struct {
	Tools    []string
	Vector   bool
	Rerank   bool
	Embedder string
	Reranker string
	Quant    string
}

// Handler is the html/template operator UI on the same ServeMux as REST and MCP.
type Handler struct {
	Engine     search.SearchEngine
	Store      *store.Store
	Source     string
	Status     Status
	Runner     *ingest.Runner
	AdminToken string
	pages      map[string]*pageTmpl
	static     fs.FS
}

func New(eng search.SearchEngine, st *store.Store, source string, status Status) *Handler {
	if source == "" {
		source = config.Attribution
	}
	if status.Tools == nil {
		status.Tools = []string{}
	}
	return &Handler{
		Engine: eng,
		Store:  st,
		Source: source,
		Status: status,
		pages:  mustParse(),
		static: mustStatic(),
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.search)
	mux.HandleFunc("GET /resources/{id}", h.resource)
	mux.HandleFunc("GET /compare", h.compare)
	mux.HandleFunc("GET /connect", h.connect)
	mux.HandleFunc("GET /ingest", h.ingestGet)
	mux.HandleFunc("POST /ingest", h.ingestPost)
	mux.HandleFunc("GET /data", h.data)
	mux.HandleFunc("GET /data/embed/{id}", h.embed)
	mux.HandleFunc("GET /webhooks", h.webhooksGet)
	mux.HandleFunc("POST /webhooks", h.webhooksPost)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(h.static))))
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	p := h.base(r, "search", "Search")
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	p.Q = q
	if p.Error != "" {
		h.render(w, "search", p)
		return
	}
	if q == "" {
		h.render(w, "search", p)
		return
	}
	if h.Engine == nil {
		p.Error = "search is not configured"
		h.render(w, "search", p)
		return
	}
	qreq, err := search.FromURL(r.URL.Query(), search.Defaults{})
	if err != nil {
		p.Error = err.Error()
		h.render(w, "search", p)
		return
	}
	start := time.Now()
	hits, err := h.Engine.Search(r.Context(), qreq)
	p.Elapsed = time.Since(start).Truncate(time.Microsecond).String()
	if err != nil {
		var bad search.ErrBadLang
		if errors.As(err, &bad) {
			p.Error = err.Error()
			h.render(w, "search", p)
			return
		}
		p.Error = err.Error()
		h.render(w, "search", p)
		return
	}
	p.Hits = hitVMs(hits, p.Lang)
	p.HitCount = len(p.Hits)
	if len(hits) > 0 {
		p.JSONPreview = previewHit(hits[0], h.Source)
	}
	h.render(w, "search", p)
}

func (h *Handler) resource(w http.ResponseWriter, r *http.Request) {
	p := h.base(r, "resource", "Resource")
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		p.Error = "id is required"
		p.Status = 400
		h.render(w, "resource", p)
		return
	}
	if p.Error != "" {
		h.render(w, "resource", p)
		return
	}
	it, err := h.load(r, id)
	if err != nil {
		h.render(w, "resource", p.withErr(err))
		return
	}
	p.Title = nameOf(*it, p.Lang)
	p.Resource = resourceVMOf(*it, p.Lang, h.Source)
	h.render(w, "resource", p)
}

func (h *Handler) compare(w http.ResponseWriter, r *http.Request) {
	p := h.base(r, "compare", "Compare")
	idA := strings.TrimSpace(r.URL.Query().Get("a"))
	idB := strings.TrimSpace(r.URL.Query().Get("b"))
	p.IDA = idA
	p.IDB = idB
	p.Unit = strings.TrimSpace(r.URL.Query().Get("unit"))
	p.Impact = strings.TrimSpace(r.URL.Query().Get("impact"))
	if p.Impact == "" {
		p.Impact = model.ImpactTypical
	}
	if p.Error != "" {
		h.render(w, "compare", p)
		return
	}
	if idA == "" || idB == "" {
		h.render(w, "compare", p)
		return
	}
	ra, err := h.load(r, idA)
	if err != nil {
		h.render(w, "compare", p.withErr(err))
		return
	}
	rb, err := h.load(r, idB)
	if err != nil {
		h.render(w, "compare", p.withErr(err))
		return
	}
	cmp, err := model.Compare(*ra, *rb, h.Source, p.Unit, p.Impact)
	if err != nil {
		var uerr model.ErrUnitUnavailable
		if errors.As(err, &uerr) {
			p.Error = err.Error()
			h.render(w, "compare", p)
			return
		}
		var ierr model.ErrImpactUnknown
		if errors.As(err, &ierr) {
			p.Error = err.Error()
			h.render(w, "compare", p)
			return
		}
		p.Error = err.Error()
		h.render(w, "compare", p)
		return
	}
	p.Impact = cmp.Impact
	p.Unit = cmp.Unit
	p.Compare = compareVMOf(cmp, p.Lang)
	h.render(w, "compare", p)
}

func (h *Handler) connect(w http.ResponseWriter, r *http.Request) {
	p := h.base(r, "connect", "Connect")
	h.render(w, "connect", p)
}

func (h *Handler) ingestGet(w http.ResponseWriter, r *http.Request) {
	p := h.base(r, "ingest", "Ingest")
	p.Catalog = model.CatalogDKBR
	h.render(w, "ingest", p)
}

func (h *Handler) ingestPost(w http.ResponseWriter, r *http.Request) {
	p := h.base(r, "ingest", "Ingest")
	if h.Runner == nil {
		p.Error = "ingest is not configured"
		p.Status = http.StatusServiceUnavailable
		h.render(w, "ingest", p)
		return
	}
	if h.AdminToken != "" {
		tok := strings.TrimSpace(r.Header.Get("X-Admin-Token"))
		if tok == "" {
			tok = strings.TrimSpace(r.FormValue("token"))
		}
		if tok != h.AdminToken {
			p.Error = "unauthorized"
			p.Status = http.StatusUnauthorized
			h.render(w, "ingest", p)
			return
		}
	}
	up, err := ingest.ReadMultipart(r)
	if err != nil && !errors.Is(err, ingest.ErrFileRequired) {
		p.Error = err.Error()
		p.Status = http.StatusBadRequest
		h.render(w, "ingest", p)
		return
	}
	if up.Catalog == "" {
		up.Catalog = model.CatalogDKBR
	}
	p.Catalog = up.Catalog
	p.Version = up.Version
	p.ColMap = up.Map
	if r.FormValue("action") == "preview" {
		if len(up.Data) == 0 {
			p.Error = ingest.ErrFileRequired.Error()
			p.Status = http.StatusBadRequest
			h.render(w, "ingest", p)
			return
		}
		prev, err := ingest.PreviewFile(up.Data, up.Filename)
		if err != nil {
			p.Error = err.Error()
			p.Status = http.StatusBadRequest
			h.render(w, "ingest", p)
			return
		}
		p.PreviewID = h.Runner.FileStash().Put(up)
		p.FilePreview = &prev
		p.Filename = up.Filename
		h.render(w, "ingest", p)
		return
	}
	up, err = up.Resolve(h.Runner.FileStash())
	if err != nil {
		p.Error = err.Error()
		p.Status = http.StatusBadRequest
		h.render(w, "ingest", p)
		return
	}
	res, err := h.Runner.Run(r.Context(), up.Ingester())
	if err != nil {
		p.Error = err.Error()
		p.Status = http.StatusBadRequest
		h.render(w, "ingest", p)
		return
	}
	p.Filename = up.Filename
	p.Ingest = &ingestVM{
		Catalog:  res.Catalog,
		Origin:   res.Origin,
		Version:  res.Version,
		Seen:     res.Seen,
		Upserted: res.Upserted,
		Skipped:  res.Skipped,
	}
	h.render(w, "ingest", p)
}

func (h *Handler) load(r *http.Request, id string) (*model.Resource, error) {
	if h.Store == nil {
		return nil, errNotConfigured
	}
	it, err := h.Store.Get(r.Context(), id)
	if err != nil {
		return nil, err
	}
	return it, nil
}

var errNotConfigured = errors.New("store is not configured")
