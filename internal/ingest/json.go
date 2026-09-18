package ingest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mong-x/klimatsearch/internal/model"
)

// ParseJSON accepts official Boverket OpenAPI v2 JSON, the old fixture shape
// (`resources` + name_sv), or a bare array of resources.
func ParseJSON(raw []byte) (Batch, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return Batch{}, fmt.Errorf("empty json")
	}
	if raw[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return Batch{}, fmt.Errorf("json array: %w", err)
		}
		if looksV2Items(items) {
			return parseV2Items(items, "", "")
		}
		return parseLegacyItems(items, "")
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Batch{}, fmt.Errorf("json object: %w", err)
	}
	if _, ok := doc["Resources"]; ok {
		return parseV2Doc(doc)
	}
	if looksV2Object(raw) {
		return parseV2Items([]json.RawMessage{raw}, stringField(doc, "Version", "version"), cultureOf(doc))
	}
	return parseLegacyDoc(doc, raw)
}

func parseV2Doc(doc map[string]json.RawMessage) (Batch, error) {
	version := stringField(doc, "Version", "version")
	culture := cultureOf(doc)
	var items []json.RawMessage
	if err := json.Unmarshal(doc["Resources"], &items); err != nil {
		return Batch{}, fmt.Errorf("json Resources: %w", err)
	}
	return parseV2Items(items, version, culture)
}

func parseV2Items(items []json.RawMessage, version, culture string) (Batch, error) {
	out := Batch{Version: version, Origin: "json", CatalogID: model.CatalogBoverket}
	for _, it := range items {
		r, err := parseV2Resource(it, culture)
		if err != nil {
			return Batch{}, err
		}
		if r.ResourceID == "" {
			continue
		}
		if r.Version == "" {
			r.Version = version
		}
		r.CatalogID = model.CatalogBoverket
		out.Resources = append(out.Resources, r)
	}
	return out, nil
}

func parseV2Resource(raw json.RawMessage, culture string) (model.Resource, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return model.Resource{}, fmt.Errorf("resource object: %w", err)
	}
	r := model.Resource{
		CatalogID:   model.CatalogBoverket,
		ResourceID:  firstString(m, "ResourceId", "resourceId"),
		Synonyms:    firstString(m, "Synonyms", "synonyms"),
		Unit:        firstString(m, "InventoryUnit", "inventoryUnit"),
		Version:     firstString(m, "Version", "SourceVersion", "version"),
		RawJSON:     string(raw),
		Conversions: map[string]float64{},
	}
	if names, ok := nestedMap(m, "Names", "names"); ok {
		r.NameSV = firstString(names, "SV", "sv")
		r.NameEN = firstString(names, "EN", "en")
	}
	name := firstString(m, "Name", "name")
	if name != "" {
		if r.NameSV == "" && !strings.HasPrefix(strings.ToLower(culture), "en") {
			r.NameSV = name
		}
		if r.NameEN == "" && strings.HasPrefix(strings.ToLower(culture), "en") {
			r.NameEN = name
		}
		if r.NameSV == "" {
			r.NameSV = name
		}
		if r.NameEN == "" && r.NameSV == "" {
			r.NameEN = name
		}
	}
	desc := firstString(m, "TechnologyDescriptionAndIncludedProcesses")
	appl := firstString(m, "TechnologicalApplicability")
	if strings.HasPrefix(strings.ToLower(culture), "en") {
		r.DescriptionEN = desc
		r.ApplicabilityEN = appl
	} else {
		r.DescriptionSV = desc
		r.ApplicabilitySV = appl
	}
	if convs, ok := m["Conversions"].([]any); ok {
		for _, c := range convs {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			unit := firstString(cm, "Unit")
			if unit == "" {
				continue
			}
			if f, ok := asFloat(cm["Value"]); ok {
				r.Conversions[unit] = f
			}
		}
	}
	if cats, ok := m["Categories"].([]any); ok {
		for _, c := range cats {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if firstString(cm, "ClassificationType") == "Boverket" {
				r.Category = firstString(cm, "Text")
				r.CategoryCode = firstString(cm, "Code")
				break
			}
		}
	}
	parseClimate(&r, m)
	parseDetailsText(&r, m, culture)
	parseTransports(&r, m)
	parseBK04(&r, m)
	return r, nil
}

func parseClimate(r *model.Resource, m map[string]any) {
	d := r.Details
	if f, ok := asFloat(m["ConservativeDataConversionFactor"]); ok {
		d.ConservativeFactor = f
	}
	if f, ok := asFloat(m["WasteFactor"]); ok {
		d.WasteFactor = f
	}
	if f, ok := asFloat(m["CalculatedBiogenicCarbon"]); ok {
		d.BiogenicCarbon = &f
	}
	items, ok := m["DataItems"].([]any)
	if !ok {
		r.Details = d
		return
	}
	for _, di := range items {
		dim, ok := di.(map[string]any)
		if !ok {
			continue
		}
		if u := firstString(dim, "PropertyUnitCode"); u != "" {
			d.GWPUnit = u
		}
		dvis, ok := dim["DataValueItems"].([]any)
		if !ok {
			continue
		}
		for _, dvi := range dvis {
			item, ok := dvi.(map[string]any)
			if !ok {
				continue
			}
			code := firstString(item, "DataModuleCode")
			f, ok := asFloat(item["Value"])
			if !ok {
				continue
			}
			switch code {
			case "A1-A3 Typical":
				r.A1A3 = f
			case "A1-A3 Conservative":
				d.A1A3Conservative = f
			case "A4":
				v := f
				d.A4 = &v
			case "A5.1":
				v := f
				d.A51 = &v
			}
		}
	}
	r.Details = d
}

func parseDetailsText(r *model.Resource, m map[string]any, culture string) {
	d := r.Details
	d.ServiceLife = firstString(m, "RefServiceLifeNormal")
	d.StdName = firstString(m, "StdName")
	d.StdCalc = firstString(m, "StdCalc")
	d.Geography = firstString(m, "GeographicalRepresentativenessDescription")
	lifeC := firstString(m, "RefServiceLifeNormalComment")
	advice := firstString(m, "UseAdviceForDataSet")
	comment := firstString(m, "GeneralComment")
	timeRep := firstString(m, "TimeRepresentativenessDescription")
	supply := firstString(m, "AnnualSupplyOrProductionVolume")
	comp := firstString(m, "ComparativeProperty")
	a4b := firstString(m, "A4ValueBackground")
	if strings.HasPrefix(strings.ToLower(culture), "en") {
		d.ServiceLifeCommentEN = lifeC
		d.UseAdviceEN = advice
		d.CommentEN = comment
		d.TimeRepEN = timeRep
		d.SupplyEN = supply
		d.ComparativeEN = comp
		d.A4BackgroundEN = a4b
	} else {
		d.ServiceLifeCommentSV = lifeC
		d.UseAdviceSV = advice
		d.CommentSV = comment
		d.TimeRepSV = timeRep
		d.SupplySV = supply
		d.ComparativeSV = comp
		d.A4BackgroundSV = a4b
	}
	r.Details = d
}

func parseTransports(r *model.Resource, m map[string]any) {
	items, ok := m["TransportItems"].([]any)
	if !ok {
		return
	}
	var legs []model.Transport
	for _, it := range items {
		tm, ok := it.(map[string]any)
		if !ok {
			continue
		}
		leg := model.Transport{
			Name:           firstString(tm, "Name"),
			Type:           firstString(tm, "TransportTypeName"),
			EnergyUse:      firstString(tm, "EnergyUseName"),
			Fuel:           firstString(tm, "FuelTypeName"),
			FuelResourceID: firstString(tm, "FuelTypeResourceId"),
		}
		if f, ok := asFloat(tm["GenericDistance"]); ok {
			leg.DistanceKM = f
		}
		if f, ok := asFloat(tm["EnergyUseValue"]); ok {
			leg.EnergyUseValue = f
		}
		if leg.Name == "" && leg.Type == "" {
			continue
		}
		legs = append(legs, leg)
	}
	r.Details.Transports = legs
}

func parseBK04(r *model.Resource, m map[string]any) {
	cats, ok := m["Categories"].([]any)
	if !ok {
		return
	}
	for _, c := range cats {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if firstString(cm, "ClassificationType") == "BK04" {
			r.Details.BK04Code = firstString(cm, "Code")
			r.Details.BK04Text = firstString(cm, "Text")
			return
		}
	}
}

func parseLegacyDoc(doc map[string]json.RawMessage, raw []byte) (Batch, error) {
	version := stringField(doc, "version", "datasetVersion", "dataset_version")
	var items []json.RawMessage
	for _, key := range []string{"resources", "data", "value", "items", "results"} {
		if v, ok := doc[key]; ok {
			if err := json.Unmarshal(v, &items); err != nil {
				return Batch{}, fmt.Errorf("json %s: %w", key, err)
			}
			break
		}
	}
	if items == nil {
		items = []json.RawMessage{raw}
	}
	return parseLegacyItems(items, version)
}

func parseLegacyItems(items []json.RawMessage, version string) (Batch, error) {
	out := Batch{Version: version, Origin: "json", CatalogID: model.CatalogBoverket}
	for _, it := range items {
		r, err := parseLegacyResource(it)
		if err != nil {
			return Batch{}, err
		}
		if r.ResourceID == "" {
			continue
		}
		if r.Version == "" {
			r.Version = version
		}
		r.CatalogID = model.CatalogBoverket
		out.Resources = append(out.Resources, r)
	}
	return out, nil
}

func parseLegacyResource(raw json.RawMessage) (model.Resource, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return model.Resource{}, fmt.Errorf("resource object: %w", err)
	}
	r := model.Resource{
		CatalogID:       model.CatalogBoverket,
		ResourceID:      firstString(m, "resourceId", "resource_id", "id", "Resurs-ID", "Resource ID"),
		NameSV:          firstString(m, "nameSV", "name_sv", "nameSv", "produktnamn", "NameSV"),
		NameEN:          firstString(m, "nameEN", "name_en", "nameEn", "NameEN"),
		DescriptionSV:   firstString(m, "descriptionSV", "description_sv", "DescriptionSV"),
		DescriptionEN:   firstString(m, "descriptionEN", "description_en", "DescriptionEN"),
		ApplicabilitySV: firstString(m, "applicabilitySV", "applicability_sv"),
		ApplicabilityEN: firstString(m, "applicabilityEN", "applicability_en"),
		Synonyms:        firstString(m, "synonyms", "Synonyms"),
		Unit:            firstString(m, "unit", "Unit", "enhet"),
		Category:        firstString(m, "category", "Category", "kategori"),
		Version:         firstString(m, "version", "Version"),
		RawJSON:         string(raw),
	}
	if r.NameSV == "" {
		r.NameSV = firstString(m, "name", "productName", "product_name")
	}
	if r.NameEN == "" && r.NameSV == "" {
		r.NameEN = firstString(m, "name")
	}
	r.A1A3 = firstFloat(m, "a1a3", "a1_a3", "A1A3", "a1a3Typical")
	r.Conversions = map[string]float64{}
	if c, ok := m["conversions"]; ok {
		switch t := c.(type) {
		case map[string]any:
			for k, v := range t {
				if f, ok := asFloat(v); ok {
					r.Conversions[k] = f
				}
			}
		}
	}
	return r, nil
}

func looksV2Items(items []json.RawMessage) bool {
	for _, it := range items {
		if looksV2Object(it) {
			return true
		}
	}
	return false
}

func looksV2Object(raw json.RawMessage) bool {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	if _, ok := m["ResourceId"]; ok {
		return true
	}
	if _, ok := m["Resources"]; ok {
		return true
	}
	return false
}

func cultureOf(doc map[string]json.RawMessage) string {
	return stringField(doc, "Culture", "culture")
}

func nestedMap(m map[string]any, keys ...string) (map[string]any, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if nested, ok := v.(map[string]any); ok {
				return nested, true
			}
		}
	}
	return nil, false
}

func stringField(doc map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		if v, ok := doc[k]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil {
				return s
			}
		}
	}
	return ""
}

func firstString(m map[string]any, keys ...string) string {
	lower := map[string]any{}
	for k, v := range m {
		lower[strings.ToLower(k)] = v
	}
	for _, k := range keys {
		if v, ok := lower[strings.ToLower(k)]; ok {
			switch t := v.(type) {
			case string:
				return t
			case float64:
				return strconv.FormatFloat(t, 'f', -1, 64)
			case json.Number:
				return t.String()
			}
		}
	}
	return ""
}

func firstFloat(m map[string]any, keys ...string) float64 {
	lower := map[string]any{}
	for k, v := range m {
		lower[strings.ToLower(k)] = v
	}
	for _, k := range keys {
		if v, ok := lower[strings.ToLower(k)]; ok {
			if f, ok := asFloat(v); ok {
				return f
			}
		}
	}
	return 0
}

func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.ReplaceAll(t, ",", "."), 64)
		return f, err == nil
	default:
		return 0, false
	}
}
