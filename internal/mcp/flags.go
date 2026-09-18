package mcp

func pickBool(p *bool, def bool) bool {
	if p != nil {
		return *p
	}
	return def
}
