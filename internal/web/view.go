package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/mong-x/klimatsearch/internal/ingest"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
)

type page struct {
	Title       string
	Active      string
	Lang        string
	Q           string
	Vector      bool
	Rerank      bool
	Source      string
	Proc        Status
	Count       int
	Error       string
	Status      int
	Hits        []hitVM
	HitCount    int
	Resource    *resourceVM
	Compare     *compareVM
	IDA         string
	IDB         string
	Unit        string
	Impact      string
	Examples    []example
	MCPSnippet  string
	NeedToken   bool
	Catalog     string
	Version     string
	Filename    string
	Ingest      *ingestVM
	Catalogs    []model.CatalogInfo
	Selected    map[string]bool
	Databases   string
	ShowRank    bool
	FilePreview *ingest.Preview
	ColMap      map[string]string
	PreviewID   string
	LogEvents   []store.IngestEvent
	Hooks       []store.Webhook
	EnvHooks    []string
	EditHook    *store.Webhook
	Elapsed     string
	RESTURL     string
	MCPCall     string
	JSONPreview string
	Coverage    *store.Coverage
	Tables      []store.InspectTable
	MetaRows    []store.MetaPair
	InspectRows []store.InspectRow
	Headers     []string
	Grid        [][]string
	Table       string
	Offset      int
	Limit       int
	Total       int
	HasPrev     bool
	HasNext     bool
	PrevOff     int
	NextOff     int
	PrevURL     string
	NextURL     string
	MissingOnly bool
	PctVec      int
	PctFTS      int
	Embed       *embedVM
	EmbedText   string
}

type ingestVM struct {
	Catalog  string
	Origin   string
	Version  string
	Seen     int
	Upserted int
	Skipped  int
}

type hitVM struct {
	ID       string
	DocID    string
	Name     string
	Category string
	Unit     string
	Source   string
	Reranked bool
	A1A3     string
	A4       string
	A51      string
	Score    string
}

type resourceVM struct {
	ID            string
	DocID         string
	Name          string
	NameOther     string
	Category      string
	CategoryCode  string
	Unit          string
	Version       string
	Origin        string
	A1A3          string
	A1A3Cons      string
	A4            string
	A51           string
	Waste         string
	ConsFactor    string
	Biogenic      string
	ServiceLife   string
	GWPUnit       string
	Description   string
	Applicability string
	UseAdvice     string
	Comment       string
	StdName       string
	StdCalc       string
	Geography     string
	BK04          string
	Transports    []transportVM
}

type transportVM struct {
	Name       string
	DistanceKM string
	Type       string
	Fuel       string
	EnergyUse  string
}

type compareVM struct {
	AName        string
	BName        string
	AID          string
	BID          string
	ADoc         string
	BDoc         string
	Unit         string
	Impact       string
	Delta        string
	LowerID      string
	LowerName    string
	Incomparable bool
	Rows         []metricRow
}

type metricRow struct {
	Label string
	A     string
	B     string
	MarkA bool
	MarkB bool
}

type example struct {
	Q     string
	Lang  string
	Label string
	Hint  string
}

func (h *Handler) base(r *http.Request, active, title string) page {
	p := page{
		Title:      title,
		Active:     active,
		Source:     h.Source,
		Proc:       h.Status,
		Examples:   examples,
		MCPSnippet: mcpSnippet,
		Status:     http.StatusOK,
		NeedToken:  h.AdminToken != "",
		Catalog:    model.CatalogDKBR,
		Catalogs:   model.Catalogs(),
	}
	qreq, err := search.FromURL(r.URL.Query(), search.Defaults{})
	if err != nil {
		p.Error = err.Error()
		p.Lang = "sv"
	} else {
		p.Lang = qreq.Lang
		p.Vector = qreq.Vector
		p.Rerank = qreq.Rerank
		p.Selected = selectedCatalogs(qreq.Catalogs)
	}
	if p.Selected == nil {
		p.Selected = selectedCatalogs(nil)
	}
	p.fillProbe(qreq)
	if h.Store != nil {
		if n, err := h.Store.Count(r.Context()); err == nil {
			p.Count = n
		}
	}
	return p
}

func (p page) withErr(err error) page {
	switch {
	case errors.Is(err, store.ErrNotFound):
		p.Error = "not found"
		p.Status = http.StatusNotFound
	case errors.Is(err, store.ErrAmbiguous):
		p.Error = err.Error()
		p.Status = http.StatusConflict
	case errors.Is(err, errNotConfigured):
		p.Error = err.Error()
		p.Status = http.StatusServiceUnavailable
	default:
		p.Error = err.Error()
		p.Status = http.StatusInternalServerError
	}
	return p
}

func hitVMs(hits []search.Hit, lang string) []hitVM {
	out := make([]hitVM, 0, len(hits))
	for _, h := range hits {
		out = append(out, hitVM{
			ID:       h.ResourceID,
			DocID:    h.DocID(),
			Name:     nameOf(h.Resource, lang),
			Category: h.Category,
			Unit:     h.Unit,
			Source:   h.Source,
			A1A3:     fmtGWP(h.A1A3),
			A4:       fmtPtr(h.Details.A4),
			A51:      fmtPtr(h.Details.A51),
			Score:    strconv.FormatFloat(h.Score, 'f', 4, 64),
			Reranked: h.Reranked,
		})
	}
	return out
}

func resourceVMOf(r model.Resource, lang, _ string) *resourceVM {
	d := r.Details
	vm := &resourceVM{
		ID:            r.ResourceID,
		DocID:         r.DocID(),
		Name:          nameOf(r, lang),
		NameOther:     otherName(r, lang),
		Category:      r.Category,
		CategoryCode:  r.CategoryCode,
		Unit:          r.Unit,
		Version:       r.Version,
		Origin:        r.Origin(),
		A1A3:          fmtGWP(r.A1A3),
		A1A3Cons:      fmtGWP(d.A1A3Conservative),
		A4:            fmtPtr(d.A4),
		A51:           fmtPtr(d.A51),
		Waste:         fmtGWP(d.WasteFactor),
		ConsFactor:    fmtGWP(d.ConservativeFactor),
		Biogenic:      fmtPtr(d.BiogenicCarbon),
		ServiceLife:   pick(lang, d.ServiceLife, d.ServiceLife),
		GWPUnit:       d.GWPUnit,
		Description:   pick(lang, r.DescriptionSV, r.DescriptionEN),
		Applicability: pick(lang, r.ApplicabilitySV, r.ApplicabilityEN),
		UseAdvice:     pick(lang, d.UseAdviceSV, d.UseAdviceEN),
		Comment:       pick(lang, d.CommentSV, d.CommentEN),
		StdName:       d.StdName,
		StdCalc:       d.StdCalc,
		Geography:     d.Geography,
		BK04:          strings.TrimSpace(strings.TrimSpace(d.BK04Code) + " " + d.BK04Text),
	}
	if sl := strings.TrimSpace(d.ServiceLife); sl != "" {
		vm.ServiceLife = sl
	}
	for _, t := range d.Transports {
		vm.Transports = append(vm.Transports, transportVM{
			Name:       t.Name,
			DistanceKM: fmtGWP(t.DistanceKM),
			Type:       t.Type,
			Fuel:       t.Fuel,
			EnergyUse:  strings.TrimSpace(t.EnergyUse),
		})
	}
	return vm
}

func compareVMOf(c model.Comparison, lang string) *compareVM {
	a, _ := c.A["id"].(string)
	b, _ := c.B["id"].(string)
	aCat, _ := c.A["catalog"].(string)
	bCat, _ := c.B["catalog"].(string)
	vm := &compareVM{
		AName:        viewName(c.A, lang),
		BName:        viewName(c.B, lang),
		AID:          a,
		BID:          b,
		ADoc:         model.DocID(aCat, a),
		BDoc:         model.DocID(bCat, b),
		Unit:         c.Unit,
		Impact:       c.Impact,
		Delta:        fmtSigned(c.DeltaA1A3),
		LowerID:      c.LowerImpactID,
		Incomparable: c.Incomparable,
	}
	if c.LowerImpactID == a {
		vm.LowerName = vm.AName
	} else if c.LowerImpactID == b {
		vm.LowerName = vm.BName
	}
	va := climateFromView(c.A, c.Impact)
	vb := climateFromView(c.B, c.Impact)
	vm.Rows = []metricRow{
		row("Typical A1–A3", fmtViewNum(c.A, "a1a3"), fmtViewNum(c.B, "a1a3"), false, false),
		row("Conservative A1–A3", fmtViewNum(c.A, "a1a3_conservative"), fmtViewNum(c.B, "a1a3_conservative"), false, false),
		row("A4", fmtViewNum(c.A, "a4"), fmtViewNum(c.B, "a4"), false, false),
		row("A5.1", fmtViewNum(c.A, "a5_1"), fmtViewNum(c.B, "a5_1"), false, false),
		{
			Label: "Compared (" + c.Impact + ")",
			A:     va,
			B:     vb,
			MarkA: c.LowerImpactID == a && !c.Incomparable,
			MarkB: c.LowerImpactID == b && !c.Incomparable,
		},
	}
	return vm
}

func row(label, a, b string, markA, markB bool) metricRow {
	return metricRow{Label: label, A: a, B: b, MarkA: markA, MarkB: markB}
}

func nameOf(r model.Resource, lang string) string {
	if lang == "en" {
		if s := strings.TrimSpace(r.NameEN); s != "" {
			return s
		}
		return strings.TrimSpace(r.NameSV)
	}
	if s := strings.TrimSpace(r.NameSV); s != "" {
		return s
	}
	return strings.TrimSpace(r.NameEN)
}

func otherName(r model.Resource, lang string) string {
	if lang == "en" {
		return strings.TrimSpace(r.NameSV)
	}
	return strings.TrimSpace(r.NameEN)
}

func pick(lang, sv, en string) string {
	if lang == "en" {
		if s := strings.TrimSpace(en); s != "" {
			return s
		}
		return strings.TrimSpace(sv)
	}
	if s := strings.TrimSpace(sv); s != "" {
		return s
	}
	return strings.TrimSpace(en)
}

func viewName(m map[string]any, lang string) string {
	if lang == "en" {
		if s, _ := m["name_en"].(string); strings.TrimSpace(s) != "" {
			return s
		}
	}
	if s, _ := m["name_sv"].(string); strings.TrimSpace(s) != "" {
		return s
	}
	if s, _ := m["name_en"].(string); strings.TrimSpace(s) != "" {
		return s
	}
	return ""
}

func fmtGWP(v float64) string {
	if v == 0 {
		return "—"
	}
	s := strconv.FormatFloat(v, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}

func fmtSigned(v float64) string {
	if v == 0 {
		return "0"
	}
	s := fmtGWP(v)
	if v > 0 && s != "—" {
		return "+" + s
	}
	return s
}

func fmtPtr(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmtGWP(*p)
}

func fmtViewNum(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case float64:
		return fmtGWP(v)
	case float32:
		return fmtGWP(float64(v))
	default:
		return "—"
	}
}

func climateFromView(m map[string]any, impact string) string {
	switch impact {
	case model.ImpactConservative:
		return fmtViewNum(m, "a1a3_conservative")
	case model.ImpactA4:
		return fmtViewNum(m, "a4")
	case model.ImpactA51:
		return fmtViewNum(m, "a5_1")
	default:
		return fmtViewNum(m, "a1a3")
	}
}

func selectedCatalogs(parsed []string) map[string]bool {
	m := map[string]bool{}
	if len(parsed) == 0 {
		for _, c := range model.Catalogs() {
			m[c.ID] = true
		}
		return m
	}
	for _, id := range parsed {
		m[model.NormalizeCatalogID(id)] = true
	}
	return m
}

func (p *page) fillProbe(q search.Query) {
	u := url.Values{}
	u.Set("q", q.Text)
	u.Set("lang", q.Lang)
	if q.Lang == "" {
		u.Set("lang", p.Lang)
	}
	if p.Vector {
		u.Set("vector", "true")
	}
	if p.Rerank {
		u.Set("rerank", "true")
	}
	var cats []string
	for _, c := range model.Catalogs() {
		if p.Selected[c.ID] {
			cats = append(cats, c.ID)
		}
	}
	if len(cats) > 0 && len(cats) < len(model.Catalogs()) {
		p.Databases = strings.Join(cats, ",")
		u.Set("databases", p.Databases)
	}
	p.RESTURL = "/api/search?" + u.Encode()
	args := map[string]any{"query": q.Text, "lang": firstNonEmpty(q.Lang, p.Lang)}
	if p.Vector {
		args["vector"] = true
	}
	if p.Rerank {
		args["rerank"] = true
	}
	if p.Databases != "" {
		args["databases"] = strings.Split(p.Databases, ",")
	}
	b, err := json.MarshalIndent(map[string]any{
		"name":      "search_climate_data",
		"arguments": args,
	}, "", "  ")
	if err == nil {
		p.MCPCall = string(b)
	}
}

func previewHit(h search.Hit, attr string) string {
	b, err := json.MarshalIndent(h.View(attr), "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}

var examples = []example{
	{Q: "spånskiva", Lang: "sv", Label: "spånskiva", Hint: "particle board"},
	{Q: "fabriksbetong C25/30", Lang: "sv", Label: "fabriksbetong C25/30", Hint: "ready-mix"},
	{Q: "glasull", Lang: "sv", Label: "glasull", Hint: "glass wool"},
	{Q: "particle board", Lang: "en", Label: "particle board", Hint: "en + FTS"},
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

const mcpSnippet = `{
  "mcpServers": {
    "klimatsearch": {
      "url": "ORIGIN/mcp"
    }
  }
}`
