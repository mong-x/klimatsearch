package search

import (
	"strconv"
	"strings"
)

// Coalesce is the Query Vector/Rerank rule: explicit pointer wins, else default.
func Coalesce(explicit *bool, def bool) bool {
	if explicit != nil {
		return *explicit
	}
	return def
}

// ParseFlag turns a query-string value into an explicit bool. Missing key → nil.
func ParseFlag(raw string, present bool) *bool {
	if !present {
		return nil
	}
	b, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		z := false
		return &z
	}
	return &b
}
