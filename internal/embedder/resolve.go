package embedder

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolveONNX picks model.int8.onnx or model.onnx.
// quant is auto|int8|fp32 (empty = auto).
func ResolveONNX(dir, quant string) (string, error) {
	quant = strings.ToLower(strings.TrimSpace(quant))
	if quant == "" {
		quant = "auto"
	}
	int8 := filepath.Join(dir, "model.int8.onnx")
	fp32 := filepath.Join(dir, "model.onnx")
	switch quant {
	case "int8":
		if fileExists(int8) {
			return int8, nil
		}
		return "", fmt.Errorf("onnx int8: missing %s (run python scripts/quantize-onnx.py %s)", int8, dir)
	case "fp32", "none":
		if fileExists(fp32) {
			return fp32, nil
		}
		return "", fmt.Errorf("onnx: missing %s", fp32)
	case "auto":
		if fileExists(int8) {
			return int8, nil
		}
		if fileExists(fp32) {
			return fp32, nil
		}
		return "", fmt.Errorf("onnx: missing %s (and no model.int8.onnx)", fp32)
	default:
		return "", fmt.Errorf("unknown KLIMAT_ONNX_QUANT %q (auto|int8|fp32)", quant)
	}
}
