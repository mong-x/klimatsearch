package ingest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mong-x/klimatsearch/internal/model"
)

// ParseJSON accepts a Boverket-shaped document or a bare array of resources.
func ParseJSON(raw []byte) (Batch, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return Batch{}, fmt.Errorf("empty json")
	}
	var version string
	var items []json.RawMessage
	if raw[0] == '[' {
		if err := json.Unmarshal(raw, &items); err != nil {
			return Batch{}, fmt.Errorf("json array: %w", err)
		}
	} else {
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(raw, &doc); err != nil {
			return Batch{}, fmt.Errorf("json object: %w", err)
		}
		version = stringField(doc, "version", "datasetVersion", "dataset_version")
		for _, key := range []string{"resources", "data", "value", "items", "results"} {
			if v, ok := doc[key]; ok {
				if err := json.Unmarshal(v, &items); err != nil {
					return Batch{}, fmt.Errorf("json %s: %w", key, err)
				}
				break
			}
		}
		if items == nil {
			// single resource object
			items = []json.RawMessage{raw}
		}
	}
	out := Batch{Version: version, Origin: "json"}
	for _, it := range items {
		r, err := parseResource(it)
		if err != nil {
			return Batch{}, err
		}
		if r.ResourceID == "" {
			continue
		}
		if r.Version == "" {
			r.Version = version
		}
		out.Resources = append(out.Resources, r)
	}
	return out, nil
}

func parseResource(raw json.RawMessage) (model.Resource, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return model.Resource{}, fmt.Errorf("resource object: %w", err)
	}
	r := model.Resource{
		ResourceID:    firstString(m, "resourceId", "resource_id", "id", "Resurs-ID", "Resource ID"),
		NameSV:        firstString(m, "nameSV", "name_sv", "nameSv", "produktnamn", "NameSV"),
		NameEN:        firstString(m, "nameEN", "name_en", "nameEn", "NameEN"),
		DescriptionSV: firstString(m, "descriptionSV", "description_sv", "DescriptionSV"),
		DescriptionEN: firstString(m, "descriptionEN", "description_en", "DescriptionEN"),
		Unit:          firstString(m, "unit", "Unit", "enhet"),
		Category:      firstString(m, "category", "Category", "kategori"),
		Version:       firstString(m, "version", "Version"),
		RawJSON:       string(raw),
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
