package reranker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/search"
)

// ONNX is a cross-encoder session. New loads it or errors.
// BGE pair-encodes; zerank/qwen instruct-encodes (chat template + Yes logit).
type ONNX struct {
	mu         sync.Mutex
	modelPath  string
	tokPath    string
	session    *ort.DynamicAdvancedSession
	inputNames []string
	instruct   bool
}

func newONNX(modelsDir, modelName, quant string) (*ONNX, error) {
	dir := filepath.Join(modelsDir, modelName)
	modelPath, err := embedder.ResolveONNX(dir, quant)
	if err != nil {
		return nil, fmt.Errorf("reranker %w (see docs/SELFHOST.md)", err)
	}
	tokPath := filepath.Join(dir, "tokenizer.json")
	if _, err := os.Stat(tokPath); err != nil {
		return nil, fmt.Errorf("reranker onnx: missing %s", tokPath)
	}
	if err := embedder.EnsureRuntime(); err != nil {
		return nil, err
	}
	inInfo, outInfo, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		return nil, fmt.Errorf("reranker onnx io: %w", err)
	}
	in := make([]string, len(inInfo))
	for i, inf := range inInfo {
		in[i] = inf.Name
	}
	out := make([]string, len(outInfo))
	for i, inf := range outInfo {
		out[i] = inf.Name
	}
	sess, err := ort.NewDynamicAdvancedSession(modelPath, in, out, nil)
	if err != nil {
		return nil, fmt.Errorf("reranker onnx session: %w", err)
	}
	return &ONNX{
		modelPath:  modelPath,
		tokPath:    tokPath,
		session:    sess,
		inputNames: in,
		instruct:   isInstructModel(modelName),
	}, nil
}

// Close releases the ONNX session. Safe to call twice.
func (o *ONNX) Close() {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.session != nil {
		o.session.Destroy()
		o.session = nil
	}
}

func (o *ONNX) Rerank(query string, docs []search.Hit) ([]search.Hit, error) {
	if len(docs) == 0 {
		return docs, nil
	}
	type scored struct {
		idx   int
		score float32
	}
	rank := make([]scored, len(docs))
	for i, d := range docs {
		s, err := o.score(query, d.EmbeddingText())
		if err != nil {
			return nil, fmt.Errorf("rerank %s: %w", d.DocID(), err)
		}
		rank[i] = scored{idx: i, score: s}
	}
	sort.SliceStable(rank, func(i, j int) bool {
		return rank[i].score > rank[j].score
	})
	out := make([]search.Hit, len(docs))
	for i, s := range rank {
		h := docs[s.idx]
		h.Score = float64(s.score)
		h.Source = "rerank"
		out[i] = h
	}
	return out, nil
}

func (o *ONNX) score(query, passage string) (float32, error) {
	var ids, mask []int64
	var err error
	if o.instruct {
		ids, mask, err = encodeInstruct(o.tokPath, query, passage)
	} else {
		ids, mask, err = encodePair(o.tokPath, query, passage)
	}
	if err != nil {
		return 0, err
	}
	seq := int64(len(ids))
	idT, err := ort.NewTensor(ort.NewShape(1, seq), ids)
	if err != nil {
		return 0, fmt.Errorf("input_ids: %w", err)
	}
	defer idT.Destroy()
	maskT, err := ort.NewTensor(ort.NewShape(1, seq), mask)
	if err != nil {
		return 0, fmt.Errorf("attention_mask: %w", err)
	}
	defer maskT.Destroy()

	var typesT, posT *ort.Tensor[int64]
	inputs := make([]ort.Value, len(o.inputNames))
	for i, name := range o.inputNames {
		switch name {
		case "input_ids":
			inputs[i] = idT
		case "attention_mask":
			inputs[i] = maskT
		case "token_type_ids":
			zeros := make([]int64, seq)
			typesT, err = ort.NewTensor(ort.NewShape(1, seq), zeros)
			if err != nil {
				return 0, fmt.Errorf("token_type_ids: %w", err)
			}
			defer typesT.Destroy()
			inputs[i] = typesT
		case "position_ids":
			pos := make([]int64, seq)
			for j := range pos {
				pos[j] = int64(j)
			}
			posT, err = ort.NewTensor(ort.NewShape(1, seq), pos)
			if err != nil {
				return 0, fmt.Errorf("position_ids: %w", err)
			}
			defer posT.Destroy()
			inputs[i] = posT
		default:
			return 0, fmt.Errorf("unexpected reranker ONNX input %q", name)
		}
	}
	outputs := []ort.Value{nil}
	o.mu.Lock()
	err = o.session.Run(inputs, outputs)
	o.mu.Unlock()
	if err != nil {
		return 0, fmt.Errorf("onnx run: %w", err)
	}
	if outputs[0] != nil {
		defer outputs[0].Destroy()
	}
	logits, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return 0, fmt.Errorf("reranker output is %T, want Tensor[float32]", outputs[0])
	}
	data := logits.GetData()
	if len(data) == 0 {
		return 0, fmt.Errorf("empty logits")
	}
	if len(data) == 1 {
		return data[0], nil
	}
	// Binary classifier: positive class is index 1.
	return data[len(data)-1], nil
}
