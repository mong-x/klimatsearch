package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mong-x/klimatsearch/internal/config"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
)

// Handler serves REST on a ServeMux.
type Handler struct {
	Engine search.SearchEngine
	Store  *store.Store
	Source string
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
	mux.HandleFunc("GET /api/resources/{id}", h.get)
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
	hits, err := h.Engine.Search(q, useVector, useRerank, lang)
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
		hits = []search.SearchResult{}
	}
	results := make([]map[string]any, 0, len(hits))
	for _, hit := range hits {
		results = append(results, map[string]any{
			"id":             hit.ID,
			"name_sv":        hit.NameSV,
			"name_en":        hit.NameEN,
			"description_sv": hit.DescriptionSV,
			"description_en": hit.DescriptionEN,
			"a1a3":           hit.A1A3,
			"unit":           hit.Unit,
			"lang":           hit.Lang,
			"score":          hit.Score,
			"match_source":   hit.Source,
		})
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
	items, err := h.Store.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if items == nil {
		items = []model.Resource{}
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, resourceJSON(it, h.Source))
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
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	body := resourceJSON(*it, h.Source)
	body["source"] = h.Source
	writeJSON(w, http.StatusOK, body)
}

func resourceJSON(r model.Resource, source string) map[string]any {
	conv := r.Conversions
	if conv == nil {
		conv = map[string]float64{}
	}
	return map[string]any{
		"id":             r.ResourceID,
		"name_sv":        r.NameSV,
		"name_en":        r.NameEN,
		"description_sv": r.DescriptionSV,
		"description_en": r.DescriptionEN,
		"a1a3":           r.A1A3,
		"unit":           r.Unit,
		"conversions":    conv,
		"category":       r.Category,
		"version":        r.Version,
		"source":         source,
	}
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
