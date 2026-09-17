package embedder

import (
	"fmt"
	"os"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// ONNX is a lazy-init production Embedder. Tests must not construct this
// without a model file; Embed fails clearly if weights or runtime are missing.
type ONNX struct {
	mu         sync.Mutex
	modelPath  string
	tokenizer  Tokenizer
	dim        int
	inited     bool
	initErr    error
	session    *ort.DynamicAdvancedSession
	inputNames []string
}

func NewONNX(modelPath string, tok Tokenizer, dim int) (*ONNX, error) {
	if dim <= 0 {
		dim = Dim
	}
	if tok == nil {
		tok = StubTokenizer{}
	}
	return &ONNX{modelPath: modelPath, tokenizer: tok, dim: dim}, nil
}

func (o *ONNX) Dim() int { return o.dim }

// Warm loads ONNX Runtime and the session so a missing library fails at process start.
func (o *ONNX) Warm() error { return o.init() }

// QueryPrefix is the F2LLM-v2 Instruct wrapper. Documents must not use it.
const QueryPrefix = "Instruct: Given a search query, retrieve the matching generic construction product or energy carrier from Boverket Klimatdatabas.\nQuery: "

func (o *ONNX) EmbedQuery(query string) ([]float32, error) {
	return o.Embed(QueryPrefix + query)
}

func (o *ONNX) Embed(text string) ([]float32, error) {
	if err := o.init(); err != nil {
		return nil, err
	}
	ids, mask, err := o.tokenizer.Encode(text)
	if err != nil {
		return nil, fmt.Errorf("tokenize: %w", err)
	}
	if len(ids) == 0 || len(ids) != len(mask) {
		return nil, fmt.Errorf("tokenizer returned empty or mismatched ids/mask")
	}
	seq := int64(len(ids))
	pos := make([]int64, seq)
	for i := range pos {
		pos[i] = int64(i)
	}
	idT, err := ort.NewTensor(ort.NewShape(1, seq), ids)
	if err != nil {
		return nil, fmt.Errorf("input_ids tensor: %w", err)
	}
	defer idT.Destroy()
	maskT, err := ort.NewTensor(ort.NewShape(1, seq), mask)
	if err != nil {
		return nil, fmt.Errorf("attention_mask tensor: %w", err)
	}
	defer maskT.Destroy()
	posT, err := ort.NewTensor(ort.NewShape(1, seq), pos)
	if err != nil {
		return nil, fmt.Errorf("position_ids tensor: %w", err)
	}
	defer posT.Destroy()

	inputs := make([]ort.Value, len(o.inputNames))
	for i, name := range o.inputNames {
		switch name {
		case "input_ids":
			inputs[i] = idT
		case "attention_mask":
			inputs[i] = maskT
		case "position_ids":
			inputs[i] = posT
		default:
			return nil, fmt.Errorf("unexpected ONNX input %q", name)
		}
	}
	outputs := []ort.Value{nil}
	o.mu.Lock()
	err = o.session.Run(inputs, outputs)
	o.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("onnx run: %w", err)
	}
	if outputs[0] != nil {
		defer outputs[0].Destroy()
	}
	hidden, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("onnx output is %T, want Tensor[float32]", outputs[0])
	}
	data := hidden.GetData()
	shape := hidden.GetShape()
	if len(shape) != 3 || shape[2] != int64(o.dim) {
		return nil, fmt.Errorf("unexpected last_hidden_state shape %v dim=%d", shape, o.dim)
	}
	last := lastTokenIndex(mask)
	if last < 0 || int64(last) >= shape[1] {
		return nil, fmt.Errorf("bad EOS index %d seq=%d", last, shape[1])
	}
	off := last * o.dim
	vec := make([]float32, o.dim)
	copy(vec, data[off:off+o.dim])
	return l2normalize(vec), nil
}

func lastTokenIndex(mask []int64) int {
	last := -1
	for i, m := range mask {
		if m != 0 {
			last = i
		}
	}
	return last
}

func (o *ONNX) init() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.inited {
		return o.initErr
	}
	o.inited = true
	if _, err := os.Stat(o.modelPath); err != nil {
		o.initErr = fmt.Errorf("onnx model %s: %w (run scripts/download-models.sh)", o.modelPath, err)
		return o.initErr
	}
	if !ort.IsInitialized() {
		if p := os.Getenv("ONNXRUNTIME_LIB"); p != "" {
			ort.SetSharedLibraryPath(p)
		} else if p := defaultORTPath(); p != "" {
			ort.SetSharedLibraryPath(p)
		}
		if err := ort.InitializeEnvironment(); err != nil {
			o.initErr = fmt.Errorf("onnxruntime init: %w (install libonnxruntime and set ONNXRUNTIME_LIB)", err)
			return o.initErr
		}
	}
	inInfo, outInfo, err := ort.GetInputOutputInfo(o.modelPath)
	if err != nil {
		o.initErr = fmt.Errorf("onnx io names: %w", err)
		return o.initErr
	}
	in := make([]string, len(inInfo))
	for i, inf := range inInfo {
		in[i] = inf.Name
	}
	out := make([]string, len(outInfo))
	for i, inf := range outInfo {
		out[i] = inf.Name
	}
	sess, err := ort.NewDynamicAdvancedSession(o.modelPath, in, out, nil)
	if err != nil {
		o.initErr = fmt.Errorf("onnx session: %w", err)
		return o.initErr
	}
	o.session = sess
	o.inputNames = in
	return nil
}

func defaultORTPath() string {
	switch runtime.GOOS {
	case "darwin":
		for _, p := range []string{
			"/usr/local/lib/libonnxruntime.dylib",
			"/opt/homebrew/lib/libonnxruntime.dylib",
		} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	case "linux":
		for _, p := range []string{
			"/usr/lib/libonnxruntime.so",
			"/usr/local/lib/libonnxruntime.so",
		} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}
