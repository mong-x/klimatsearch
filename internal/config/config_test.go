package config

import (
	"reflect"
	"testing"
)

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

func TestONNXQuantFlag(t *testing.T) {
	c, err := Parse([]string{"-onnx-quant", "int8"})
	if err != nil {
		t.Fatal(err)
	}
	if c.ONNXQuant != "int8" {
		t.Fatalf("ONNXQuant=%s", c.ONNXQuant)
	}
	if _, err := Parse([]string{"-onnx-quant", "q4"}); err == nil {
		t.Fatal("expected invalid quant")
	}
}

func TestParseAPIKeys(t *testing.T) {
	cases := []struct {
		raw  string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{",,,", nil},
		{"one", []string{"one"}},
		{"one,two", []string{"one", "two"}},
		{" one , two ,,", []string{"one", "two"}},
		{"one two\tthree", []string{"one", "two", "three"}},
	}
	for _, tc := range cases {
		if got := ParseAPIKeys(tc.raw); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseAPIKeys(%q)=%v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestAPIKeysEnvAndFlag(t *testing.T) {
	t.Setenv("KLIMAT_API_KEYS", "env-one,env-two")
	c, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"env-one", "env-two"}; !reflect.DeepEqual(c.APIKeys, want) {
		t.Fatalf("env APIKeys=%v, want %v", c.APIKeys, want)
	}
	c, err = Parse([]string{"-api-keys", "flag-one"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"flag-one"}; !reflect.DeepEqual(c.APIKeys, want) {
		t.Fatalf("flag APIKeys=%v, want %v", c.APIKeys, want)
	}
}

func TestAPIKeysEmptyMeansNoGating(t *testing.T) {
	c, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.APIKeys != nil {
		t.Fatalf("APIKeys=%v, want nil", c.APIKeys)
	}
}
