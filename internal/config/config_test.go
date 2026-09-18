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
