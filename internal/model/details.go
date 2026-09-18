package model

// Details is Boverket payload beyond typical A1–A3. Empty for BR25 until mapped.
// a1a3 on Resource stays the typical GWP-GHG value used for Compare.
type Details struct {
	A1A3Conservative     float64     `json:"a1a3_conservative,omitempty"`
	A4                   *float64    `json:"a4,omitempty"`
	A51                  *float64    `json:"a5_1,omitempty"`
	GWPUnit              string      `json:"gwp_unit,omitempty"`
	ConservativeFactor   float64     `json:"conservative_factor,omitempty"`
	WasteFactor          float64     `json:"waste_factor,omitempty"`
	BiogenicCarbon       *float64    `json:"biogenic_carbon,omitempty"`
	ServiceLife          string      `json:"service_life,omitempty"`
	ServiceLifeCommentSV string      `json:"service_life_comment_sv,omitempty"`
	ServiceLifeCommentEN string      `json:"service_life_comment_en,omitempty"`
	UseAdviceSV          string      `json:"use_advice_sv,omitempty"`
	UseAdviceEN          string      `json:"use_advice_en,omitempty"`
	CommentSV            string      `json:"comment_sv,omitempty"`
	CommentEN            string      `json:"comment_en,omitempty"`
	StdName              string      `json:"std_name,omitempty"`
	StdCalc              string      `json:"std_calc,omitempty"`
	Geography            string      `json:"geography,omitempty"`
	TimeRepSV            string      `json:"time_representativeness_sv,omitempty"`
	TimeRepEN            string      `json:"time_representativeness_en,omitempty"`
	SupplySV             string      `json:"supply_sv,omitempty"`
	SupplyEN             string      `json:"supply_en,omitempty"`
	ComparativeSV        string      `json:"comparative_sv,omitempty"`
	ComparativeEN        string      `json:"comparative_en,omitempty"`
	A4BackgroundSV       string      `json:"a4_background_sv,omitempty"`
	A4BackgroundEN       string      `json:"a4_background_en,omitempty"`
	BK04Code             string      `json:"bk04_code,omitempty"`
	BK04Text             string      `json:"bk04_text,omitempty"`
	Transports           []Transport `json:"transports,omitempty"`
}

// Transport is one generic A4 transport leg from Boverket TransportItems.
type Transport struct {
	Name           string  `json:"name,omitempty"`
	DistanceKM     float64 `json:"distance_km,omitempty"`
	Type           string  `json:"type,omitempty"`
	EnergyUse      string  `json:"energy_use,omitempty"`
	EnergyUseValue float64 `json:"energy_use_value,omitempty"`
	Fuel           string  `json:"fuel,omitempty"`
	FuelResourceID string  `json:"fuel_resource_id,omitempty"`
}

// MergeInto copies non-zero Details onto a Resource View map. JSON keys stay flat.
func (d Details) MergeInto(m map[string]any) {
	if d.A1A3Conservative != 0 {
		m["a1a3_conservative"] = d.A1A3Conservative
	}
	if d.A4 != nil {
		m["a4"] = *d.A4
	}
	if d.A51 != nil {
		m["a5_1"] = *d.A51
	}
	putStr(m, "gwp_unit", d.GWPUnit)
	if d.ConservativeFactor != 0 {
		m["conservative_factor"] = d.ConservativeFactor
	}
	if d.WasteFactor != 0 {
		m["waste_factor"] = d.WasteFactor
	}
	if d.BiogenicCarbon != nil {
		m["biogenic_carbon"] = *d.BiogenicCarbon
	}
	putStr(m, "service_life", d.ServiceLife)
	putStr(m, "service_life_comment_sv", d.ServiceLifeCommentSV)
	putStr(m, "service_life_comment_en", d.ServiceLifeCommentEN)
	putStr(m, "use_advice_sv", d.UseAdviceSV)
	putStr(m, "use_advice_en", d.UseAdviceEN)
	putStr(m, "comment_sv", d.CommentSV)
	putStr(m, "comment_en", d.CommentEN)
	putStr(m, "std_name", d.StdName)
	putStr(m, "std_calc", d.StdCalc)
	putStr(m, "geography", d.Geography)
	putStr(m, "time_representativeness_sv", d.TimeRepSV)
	putStr(m, "time_representativeness_en", d.TimeRepEN)
	putStr(m, "supply_sv", d.SupplySV)
	putStr(m, "supply_en", d.SupplyEN)
	putStr(m, "comparative_sv", d.ComparativeSV)
	putStr(m, "comparative_en", d.ComparativeEN)
	putStr(m, "a4_background_sv", d.A4BackgroundSV)
	putStr(m, "a4_background_en", d.A4BackgroundEN)
	putStr(m, "bk04_code", d.BK04Code)
	putStr(m, "bk04_text", d.BK04Text)
	if len(d.Transports) > 0 {
		m["transports"] = d.Transports
	}
}

func putStr(m map[string]any, k, v string) {
	if v != "" {
		m[k] = v
	}
}

func (d Details) empty() bool {
	return d.A1A3Conservative == 0 && d.A4 == nil && d.A51 == nil && d.GWPUnit == "" &&
		d.ConservativeFactor == 0 && d.WasteFactor == 0 && d.BiogenicCarbon == nil &&
		d.ServiceLife == "" && d.StdName == "" && d.StdCalc == "" && d.Geography == "" &&
		d.BK04Code == "" && len(d.Transports) == 0 &&
		d.UseAdviceSV == "" && d.UseAdviceEN == "" && d.CommentSV == "" && d.CommentEN == ""
}

// MergeDetails overlays src onto dst. fromEN fills *_en text; otherwise *_sv.
func MergeDetails(dst, src Details, fromEN bool) Details {
	if dst.A1A3Conservative == 0 {
		dst.A1A3Conservative = src.A1A3Conservative
	}
	if dst.A4 == nil {
		dst.A4 = src.A4
	}
	if dst.A51 == nil {
		dst.A51 = src.A51
	}
	if dst.GWPUnit == "" {
		dst.GWPUnit = src.GWPUnit
	}
	if dst.ConservativeFactor == 0 {
		dst.ConservativeFactor = src.ConservativeFactor
	}
	if dst.WasteFactor == 0 {
		dst.WasteFactor = src.WasteFactor
	}
	if dst.BiogenicCarbon == nil {
		dst.BiogenicCarbon = src.BiogenicCarbon
	}
	if dst.ServiceLife == "" {
		dst.ServiceLife = src.ServiceLife
	}
	if dst.StdName == "" {
		dst.StdName = src.StdName
	}
	if dst.StdCalc == "" {
		dst.StdCalc = src.StdCalc
	}
	if dst.Geography == "" {
		dst.Geography = src.Geography
	}
	if dst.BK04Code == "" {
		dst.BK04Code = src.BK04Code
		dst.BK04Text = src.BK04Text
	}
	if len(dst.Transports) == 0 {
		dst.Transports = src.Transports
	}
	if fromEN {
		if src.ServiceLifeCommentEN != "" {
			dst.ServiceLifeCommentEN = src.ServiceLifeCommentEN
		}
		if src.UseAdviceEN != "" {
			dst.UseAdviceEN = src.UseAdviceEN
		}
		if src.CommentEN != "" {
			dst.CommentEN = src.CommentEN
		}
		if src.TimeRepEN != "" {
			dst.TimeRepEN = src.TimeRepEN
		}
		if src.SupplyEN != "" {
			dst.SupplyEN = src.SupplyEN
		}
		if src.ComparativeEN != "" {
			dst.ComparativeEN = src.ComparativeEN
		}
		if src.A4BackgroundEN != "" {
			dst.A4BackgroundEN = src.A4BackgroundEN
		}
	} else {
		if src.ServiceLifeCommentSV != "" {
			dst.ServiceLifeCommentSV = src.ServiceLifeCommentSV
		}
		if src.UseAdviceSV != "" {
			dst.UseAdviceSV = src.UseAdviceSV
		}
		if src.CommentSV != "" {
			dst.CommentSV = src.CommentSV
		}
		if src.TimeRepSV != "" {
			dst.TimeRepSV = src.TimeRepSV
		}
		if src.SupplySV != "" {
			dst.SupplySV = src.SupplySV
		}
		if src.ComparativeSV != "" {
			dst.ComparativeSV = src.ComparativeSV
		}
		if src.A4BackgroundSV != "" {
			dst.A4BackgroundSV = src.A4BackgroundSV
		}
	}
	return dst
}
