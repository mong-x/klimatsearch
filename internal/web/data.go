package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/mong-x/klimatsearch/internal/store"
)

type embedVM struct {
	DocID   string
	Dim     int
	Norm    string
	Min     string
	Max     string
	Mean    string
	Preview []string
	Tail    []string
}

func (h *Handler) data(w http.ResponseWriter, r *http.Request) {
	p := h.base(r, "data", "Data")
	if h.Store == nil {
		p.Error = "store is not configured"
		p.Status = http.StatusServiceUnavailable
		h.render(w, "data", p)
		return
	}
	ctx := r.Context()
	cov, err := h.Store.Coverage(ctx)
	if err != nil {
		p.Error = err.Error()
		h.render(w, "data", p)
		return
	}
	tables, err := h.Store.TableStats(ctx)
	if err != nil {
		p.Error = err.Error()
		h.render(w, "data", p)
		return
	}
	meta, err := h.Store.MetaAll(ctx)
	if err != nil {
		p.Error = err.Error()
		h.render(w, "data", p)
		return
	}
	p.Coverage = &cov
	p.Tables = tables
	p.MetaRows = meta
	if evs, err := h.Store.ListIngestEvents(ctx, 20); err == nil {
		p.LogEvents = evs
	}
	p.PctVec = pct(cov.Vectors, cov.Resources)
	p.PctFTS = pct(cov.FTS, cov.Resources)

	table := strings.TrimSpace(r.URL.Query().Get("table"))
	if table == "" {
		table = "resources"
	}
	p.Table = table
	p.Q = strings.TrimSpace(r.URL.Query().Get("q"))
	p.Catalog = strings.TrimSpace(r.URL.Query().Get("catalog"))
	p.MissingOnly = r.URL.Query().Get("missing") == "1"
	p.Offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	if p.Offset < 0 {
		p.Offset = 0
	}
	p.Limit = 50

	switch table {
	case "resources":
		rows, total, err := h.Store.InspectResources(ctx, p.Catalog, p.Q, p.MissingOnly, p.Offset, p.Limit)
		if err != nil {
			p.Error = err.Error()
			h.render(w, "data", p)
			return
		}
		p.InspectRows = rows
		p.Total = total
	case "resources_fts":
		grid, headers, total, err := h.Store.InspectFTS(ctx, p.Q, p.Offset, p.Limit)
		if err != nil {
			p.Error = err.Error()
			h.render(w, "data", p)
			return
		}
		p.Grid, p.Headers, p.Total = grid, headers, total
	case "resource_vec":
		grid, headers, total, err := h.Store.InspectVec(ctx, p.Q, p.Offset, p.Limit)
		if err != nil {
			p.Error = err.Error()
			h.render(w, "data", p)
			return
		}
		p.Grid, p.Headers, p.Total = grid, headers, total
	case "meta":
		p.Total = len(meta)
	default:
		p.Error = "unknown table"
		p.Status = http.StatusBadRequest
	}
	p.HasPrev = p.Offset > 0
	p.HasNext = p.Offset+p.Limit < p.Total
	p.NextOff = p.Offset + p.Limit
	p.PrevOff = p.Offset - p.Limit
	if p.PrevOff < 0 {
		p.PrevOff = 0
	}
	p.PrevURL = dataQuery(table, p.Q, p.Catalog, p.MissingOnly, p.PrevOff)
	p.NextURL = dataQuery(table, p.Q, p.Catalog, p.MissingOnly, p.NextOff)
	h.render(w, "data", p)
}

func (h *Handler) embed(w http.ResponseWriter, r *http.Request) {
	p := h.base(r, "data", "Embedding")
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		p.Error = "id is required"
		p.Status = http.StatusBadRequest
		h.render(w, "data", p)
		return
	}
	if h.Store == nil {
		h.render(w, "data", p.withErr(errNotConfigured))
		return
	}
	view, err := h.Store.InspectEmbedding(r.Context(), id)
	if err != nil {
		h.render(w, "data", p.withErr(err))
		return
	}
	p.Embed = embedVMOf(view)
	if it, err := h.Store.Get(r.Context(), id); err == nil {
		p.Resource = resourceVMOf(*it, p.Lang, h.Source)
		p.EmbedText = it.EmbeddingText()
	}
	h.render(w, "data", p)
}

func pct(part, whole int) int {
	if whole <= 0 {
		return 0
	}
	return (part * 100) / whole
}

func dataQuery(table, q, catalog string, missing bool, offset int) string {
	v := url.Values{}
	v.Set("table", table)
	if q != "" {
		v.Set("q", q)
	}
	if catalog != "" {
		v.Set("catalog", catalog)
	}
	if missing {
		v.Set("missing", "1")
	}
	if offset > 0 {
		v.Set("offset", strconv.Itoa(offset))
	}
	return "/data?" + v.Encode()
}

func embedVMOf(v *store.EmbeddingView) *embedVM {
	if v == nil {
		return nil
	}
	e := &embedVM{
		DocID: v.DocID,
		Dim:   v.Dim,
		Norm:  fmtFloat(v.Norm),
		Min:   fmtFloat(v.Min),
		Max:   fmtFloat(v.Max),
		Mean:  fmtFloat(v.Mean),
	}
	for _, x := range v.Preview {
		e.Preview = append(e.Preview, fmtFloat(float64(x)))
	}
	for _, x := range v.Tail {
		e.Tail = append(e.Tail, fmtFloat(float64(x)))
	}
	return e
}

func fmtFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 4, 64)
}
