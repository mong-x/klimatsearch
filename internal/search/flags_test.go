package search

import "testing"

func TestCoalesce(t *testing.T) {
	tr, fa := true, false
	if Coalesce(nil, true) != true || Coalesce(nil, false) != false {
		t.Fatal("nil keeps default")
	}
	if Coalesce(&fa, true) {
		t.Fatal("explicit false")
	}
	if !Coalesce(&tr, false) {
		t.Fatal("explicit true")
	}
}

func TestParseFlag(t *testing.T) {
	if ParseFlag("", false) != nil {
		t.Fatal("absent")
	}
	p := ParseFlag("true", true)
	if p == nil || !*p {
		t.Fatal("true")
	}
	p = ParseFlag("0", true)
	if p == nil || *p {
		t.Fatal("0 is false")
	}
}
