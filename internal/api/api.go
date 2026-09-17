package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/mong-x/klimatsearch/internal/config"
	"github.com/mong-x/klimatsearch/internal/ingest"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
)

// Handler serves REST on a ServeMux.
type Handler struct {
	Engine     search.SearchEngine
	Store      *store.Store
	Source     string
	Runner     *ingest.Runner
	AdminToken string
}

func New(eng search.SearchEngine, st *store.Store, source string) *Handler {
	if source == "" {
		source = config.Attribution
	}
	return &Handler{Engine: eng, Store: st, Source: source}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /api/search", h.search)
	mux.HandleFunc("GET /api/resources", h.list)
	mux.HandleFunc("GET /api/resources/compare", h.compare)
	mux.HandleFunc("GET /api/resources/{id}/origin", h.origin)
	mux.HandleFunc("GET /api/resources/{id}", h.get)
	mux.HandleFunc("POST /admin/ingest/file", h.ingestFile)
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
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query q is required"})
		return
	}
	lang, err := search.NormalizeLang(r.URL.Query().Get("lang"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	useVector := parseBool(r.URL.Query().Get("vector"), false)
	useRerank := parseBool(r.URL.Query().Get("rerank"), false)
	catalogs := store.ParseCatalogs(r.URL.Query().Get("databases"))
	hits, err := h.Engine.Search(r.Context(), q, useVector, useRerank, lang, catalogs)
	if err != nil {
		var bad search.ErrBadLang
		if errors.As(err, &bad) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if hits == nil {
		hits = []search.Hit{}
	}
	results := make([]map[string]any, 0, len(hits))
	for _, hit := range hits {
		results = append(results, hit.View(h.Source))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source":  h.Source,
		"query":   q,
		"lang":    lang,
		"results": results,
	})
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
	cmp, err := model.Compare(*ra, *rb, h.Source, unit)
	if err != nil {
		var uerr model.ErrUnitUnavailable
		if errors.As(err, &uerr) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source":          cmp.Attribution,
		"a":               cmp.A,
		"b":               cmp.B,
		"unit":            cmp.Unit,
		"delta_a1a3":      cmp.DeltaA1A3,
		"lower_impact_id": cmp.LowerImpactID,
		"incomparable":    cmp.Incomparable,
	})
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
	if h.AdminToken != "" && r.Header.Get("X-Admin-Token") != h.AdminToken {
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
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 32<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
		return
	}
	name := ""
	if hdr != nil {
		name = hdr.Filename
	}
	ing := ingest.FileIngester{Catalog: catalog, Data: data, Name: name}
	res, err := h.Runner.Run(r.Context(), ing)
	if err != nil {
		var unknown ingest.UnknownCatalogError
		if errors.As(err, &unknown) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		var ni ingest.ErrCatalogNotImplemented
		if errors.As(err, &ni) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"catalog":  catalog,
		"origin":   res.Origin,
		"version":  res.Version,
		"seen":     res.Seen,
		"upserted": res.Upserted,
		"skipped":  res.Skipped,
	})
}

func parseBool(v string, def bool) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
