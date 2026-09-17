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
	mu        sync.Mutex
	modelPath string
	tokenizer Tokenizer
	dim       int
	inited    bool
	initErr   error
	session   *ort.DynamicAdvancedSession
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
	_ = ids
	_ = mask
	return nil, fmt.Errorf("onnx session is up but Embed inference is not wired (EOS pool + L2); see docs/LAUNCH.md §1 or use --embedder=fake")
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
