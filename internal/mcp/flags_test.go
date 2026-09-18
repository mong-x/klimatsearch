package mcp

import "testing"

func TestPickBool(t *testing.T) {
	tr, fa := true, false
	if pickBool(nil, true) != true || pickBool(nil, false) != false {
		t.Fatal("nil keeps default")
	}
	if pickBool(&fa, true) != false {
		t.Fatal("explicit false must win")
	}
	if pickBool(&tr, false) != true {
		t.Fatal("explicit true must win")
	}
}
