package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/mong-x/klimatsearch/internal/config"
	"github.com/mong-x/klimatsearch/internal/guard"
	"github.com/mong-x/klimatsearch/internal/ingest"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
)

// Handler serves REST on a ServeMux.
type Handler struct {
	Engine search.SearchEngine
	Store  *store.Store
	Source string
	Runner *ingest.Runner
	Admin  guard.Admin
}

func New(eng search.SearchEngine, st *store.Store, source string) *Handler {
	if source == "" {
		source = config.Attribution
	}
	return &Handler{Engine: eng, Store: st, Source: source}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /openapi.yaml", h.openapiYAML)
	mux.HandleFunc("GET /docs", h.docs)
	mux.HandleFunc("GET /api/search", h.search)
	mux.HandleFunc("GET /api/resources", h.list)
	mux.HandleFunc("GET /api/resources/compare", h.compare)
	mux.HandleFunc("GET /api/resources/{id}/origin", h.origin)
	mux.HandleFunc("GET /api/resources/{id}", h.get)
	mux.HandleFunc("POST /admin/ingest/file", h.ingestFile)
	mux.HandleFunc("POST /admin/ingest/preview", h.ingestPreview)
	mux.HandleFunc("POST /admin/resources", h.upsertResources)
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	if h.Store != nil {
		if err := h.Store.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	qreq, err := search.FromURL(r.URL.Query(), search.Defaults{})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if qreq.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query q is required"})
		return
	}
	hits, err := h.Engine.Search(r.Context(), qreq)
	if err != nil {
		var bad search.ErrBadLang
		if errors.As(err, &bad) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stampDetails(search.Envelope(h.Source, qreq, hits)))
}

func stampDetails(env map[string]any) map[string]any {
	results, ok := env["results"].([]map[string]any)
	if !ok {
		return env
	}
	for _, m := range results {
		id, _ := m["id"].(string)
		cat, _ := m["catalog"].(string)
		if id == "" {
			continue
		}
		m["details"] = "/api/resources/" + model.DocID(cat, id)
	}
	return env
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	if _, err := search.NormalizeLang(r.URL.Query().Get("lang")); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	catalogs := store.ParseCatalogs(r.URL.Query().Get("databases"))
	items, err := h.Store.List(r.Context(), catalogs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if items == nil {
		items = []model.Resource{}
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, it.View(h.Source))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source":    h.Source,
		"resources": out,
	})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	if _, err := search.NormalizeLang(r.URL.Query().Get("lang")); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	it, err := h.Store.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if errors.Is(err, store.ErrAmbiguous) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	body := it.View(h.Source)
	writeJSON(w, http.StatusOK, body)
}

func (h *Handler) origin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	it, status, err := h.loadResource(r, id)
	if err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	loc := it.Origin()
	if loc == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no origin URL for this catalog"})
		return
	}
	w.Header().Set("Location", loc)
	w.WriteHeader(http.StatusFound)
}

func (h *Handler) compare(w http.ResponseWriter, r *http.Request) {
	idA := strings.TrimSpace(r.URL.Query().Get("a"))
	idB := strings.TrimSpace(r.URL.Query().Get("b"))
	if idA == "" || idB == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query a and b are required"})
		return
	}
	unit := strings.TrimSpace(r.URL.Query().Get("unit"))
	impact := strings.TrimSpace(r.URL.Query().Get("impact"))
	ra, status, err := h.loadResource(r, idA)
	if err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	rb, status, err := h.loadResource(r, idB)
	if err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	cmp, err := model.Compare(*ra, *rb, h.Source, unit, impact)
	if err != nil {
		var uerr model.ErrUnitUnavailable
		if errors.As(err, &uerr) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		var ierr model.ErrImpactUnknown
		if errors.As(err, &ierr) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cmp.View())
}

func (h *Handler) loadResource(r *http.Request, id string) (*model.Resource, int, error) {
	it, err := h.Store.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, http.StatusNotFound, errors.New("not found")
	}
	if errors.Is(err, store.ErrAmbiguous) {
		return nil, http.StatusConflict, err
	}
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return it, http.StatusOK, nil
}

func (h *Handler) ingestFile(w http.ResponseWriter, r *http.Request) {
	if !h.Admin.OK(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	catalog := strings.TrimSpace(r.URL.Query().Get("catalog"))
	if catalog == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "catalog query is required"})
		return
	}
	if h.Runner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ingest is not configured"})
		return
	}
	p, err := ingest.ReadMultipart(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	p.Catalog = catalog
	if v := strings.TrimSpace(r.URL.Query().Get("version")); v != "" && p.Version == "" {
		p.Version = v
	}
	p, err = p.Resolve(h.Runner.FileStash())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	res, err := h.Runner.Run(r.Context(), p.Ingester())
	if err != nil {
		writeJSON(w, ingestStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ingest.ResultBody(res))
}

func (h *Handler) ingestPreview(w http.ResponseWriter, r *http.Request) {
	if !h.Admin.OK(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	p, err := ingest.ReadMultipart(r)
	if err != nil || len(p.Data) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": ingest.ErrFileRequired.Error()})
		return
	}
	prev, err := ingest.PreviewFile(p.Data, p.Filename)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if h.Runner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ingest is not configured"})
		return
	}
	id := h.Runner.FileStash().Put(p)
	writeJSON(w, http.StatusOK, map[string]any{
		"preview_id": id,
		"filename":   p.Filename,
		"headers":    prev.Headers,
		"sample":     prev.Sample,
		"schema":     prev.Schema,
	})
}

func (h *Handler) upsertResources(w http.ResponseWriter, r *http.Request) {
	if !h.Admin.OK(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if h.Runner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ingest is not configured"})
		return
	}
	var body ingest.ResourceBatch
	if err := json.NewDecoder(io.LimitReader(r.Body, 32<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	res, err := h.Runner.Run(r.Context(), body)
	if err != nil {
		writeJSON(w, ingestStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"catalog":  res.Catalog,
		"origin":   res.Origin,
		"version":  res.Version,
		"seen":     res.Seen,
		"upserted": res.Upserted,
		"skipped":  res.Skipped,
	})
}

func ingestStatus(err error) int {
	var unknown ingest.UnknownCatalogError
	var ni ingest.ErrCatalogNotImplemented
	if errors.As(err, &unknown) || errors.As(err, &ni) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
