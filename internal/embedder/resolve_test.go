package embedder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveONNX(t *testing.T) {
	dir := t.TempDir()
	if _, err := ResolveONNX(dir, "auto"); err == nil {
		t.Fatal("expected error when no onnx file exists")
	}
	fp32 := filepath.Join(dir, "model.onnx")
	if err := os.WriteFile(fp32, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ResolveONNX(dir, "auto")
	if err != nil || p != fp32 {
		t.Fatalf("auto fp32: path=%s err=%v", p, err)
	}
	int8 := filepath.Join(dir, "model.int8.onnx")
	if err := os.WriteFile(int8, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err = ResolveONNX(dir, "")
	if err != nil || p != int8 {
		t.Fatalf("auto prefers int8: path=%s err=%v", p, err)
	}
	p, err = ResolveONNX(dir, "fp32")
	if err != nil || p != fp32 {
		t.Fatalf("fp32: path=%s err=%v", p, err)
	}
	p, err = ResolveONNX(dir, "int8")
	if err != nil || p != int8 {
		t.Fatalf("int8: path=%s err=%v", p, err)
	}
	if _, err := ResolveONNX(dir, "q4"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("want unknown quant error, got %v", err)
	}
}

func TestResolveONNXInt8Missing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "model.onnx"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveONNX(dir, "int8"); err == nil {
		t.Fatal("expected missing int8 error")
	}
}
