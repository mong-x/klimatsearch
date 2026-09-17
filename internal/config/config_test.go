package config

import "testing"

func TestDefaultAPIBase(t *testing.T) {
	if DefaultAPIBase != "https://api.boverket.se/klimatdatabas" {
		t.Fatalf("DefaultAPIBase=%s", DefaultAPIBase)
	}
	c, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.BoverketAPIBase != DefaultAPIBase {
		t.Fatalf("BoverketAPIBase=%s", c.BoverketAPIBase)
	}
}
