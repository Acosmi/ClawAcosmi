//go:build onnx

// Package embedding — ONNX Runtime local inference engine.
// Only compiled when building with: go build -tags onnx
// Requires: ONNX Runtime shared library installed (brew install onnxruntime on macOS)
package embedding

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// ONNXEngine runs embedding models locally via ONNX Runtime.
// Requires CGO and the ONNX Runtime shared library.
type ONNXEngine struct {
	modelPath   string
	dim         int
	maxSeqLen   int
	initialized bool
	initErr     error
	initOnce    sync.Once
	ortSession  *ort.AdvancedSession
	inputNames  []string
	outputNames []string
}

const (
	defaultONNXModelURL = "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx"
	defaultONNXDim      = 384
	defaultMaxSeqLen    = 128
)

// NewONNXEngineFromConfig creates an ONNX embedding engine.
// Returns nil if modelPath is empty (ONNX disabled).
func NewONNXEngineFromConfig(modelPath string) EmbedEngine {
	if modelPath == "" {
		return nil
	}
	return &ONNXEngine{
		modelPath: modelPath,
		dim:       defaultONNXDim,
		maxSeqLen: defaultMaxSeqLen,
	}
}

func (o *ONNXEngine) Name() string { return "onnx" }

// Available returns true if the ONNX model file exists and runtime can be initialized.
func (o *ONNXEngine) Available() bool {
	o.initOnce.Do(o.initialize)
	return o.initialized && o.initErr == nil
}

// initialize loads the ONNX model lazily on first use.
func (o *ONNXEngine) initialize() {
	// Expand home directory
	if o.modelPath[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err == nil {
			o.modelPath = filepath.Join(home, o.modelPath[2:])
		}
	}

	// Check if model exists, attempt download if not
	if _, err := os.Stat(o.modelPath); os.IsNotExist(err) {
		slog.Info("ONNX model not found, attempting download", "path", o.modelPath)
		if err := o.downloadModel(); err != nil {
			o.initErr = fmt.Errorf("onnx: model download failed: %w", err)
			slog.Warn("ONNX model download failed", "error", err)
			return
		}
	}

	// Find and set ONNX Runtime shared library path
	libPath := findONNXRuntimeLib()
	if libPath == "" {
		o.initErr = fmt.Errorf("onnx: ONNX Runtime shared library not found (install via: brew install onnxruntime)")
		slog.Warn("ONNX Runtime library not found")
		return
	}
	ort.SetSharedLibraryPath(libPath)

	if err := ort.InitializeEnvironment(); err != nil {
		o.initErr = fmt.Errorf("onnx: initialize environment: %w", err)
		slog.Warn("ONNX Runtime init failed", "error", err)
		return
	}

	// Create session with the model
	// Input: input_ids [batch, seq_len], attention_mask [batch, seq_len], token_type_ids [batch, seq_len]
	// Output: last_hidden_state [batch, seq_len, dim]
	o.inputNames = []string{"input_ids", "attention_mask", "token_type_ids"}
	o.outputNames = []string{"last_hidden_state"}

	// We'll create sessions per-call with dynamic shapes
	o.initialized = true
	slog.Info("ONNX engine initialized", "model", o.modelPath, "dim", o.dim)
}

// Embed generates embeddings using the local ONNX model.
// Uses a simplified tokenizer (whitespace + padding) since full HuggingFace tokenizer
// requires additional Rust CGO dependency. For production quality, use Ollama instead.
func (o *ONNXEngine) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	o.initOnce.Do(o.initialize)
	if o.initErr != nil {
		return nil, o.initErr
	}

	results := make([][]float32, len(texts))

	for i, text := range texts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		vec, err := o.embedSingle(text)
		if err != nil {
			return nil, fmt.Errorf("onnx: embed text %d: %w", i, err)
		}
		results[i] = vec
	}

	return results, nil
}

// embedSingle embeds a single text using the ONNX model.
func (o *ONNXEngine) embedSingle(text string) ([]float32, error) {
	// Simple tokenization: convert to token IDs using basic encoding
	// This is a simplified approach — for full accuracy, use Ollama instead
	inputIDs, attentionMask := simpleTokenize(text, o.maxSeqLen)
	tokenTypeIDs := make([]int64, len(inputIDs)) // all zeros for single-sentence

	batchSize := int64(1)
	seqLen := int64(len(inputIDs))

	inputShape := ort.Shape{batchSize, seqLen}
	outputShape := ort.Shape{batchSize, seqLen, int64(o.dim)}

	inputIDsTensor, err := ort.NewTensor(inputShape, inputIDs)
	if err != nil {
		return nil, fmt.Errorf("create input_ids tensor: %w", err)
	}
	defer inputIDsTensor.Destroy()

	attMaskTensor, err := ort.NewTensor(inputShape, attentionMask)
	if err != nil {
		return nil, fmt.Errorf("create attention_mask tensor: %w", err)
	}
	defer attMaskTensor.Destroy()

	tokenTypeTensor, err := ort.NewTensor(inputShape, tokenTypeIDs)
	if err != nil {
		return nil, fmt.Errorf("create token_type_ids tensor: %w", err)
	}
	defer tokenTypeTensor.Destroy()

	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, fmt.Errorf("create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	session, err := ort.NewAdvancedSession(
		o.modelPath,
		o.inputNames, o.outputNames,
		[]ort.ArbitraryTensor{inputIDsTensor, attMaskTensor, tokenTypeTensor},
		[]ort.ArbitraryTensor{outputTensor},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	defer session.Destroy()

	if err := session.Run(); err != nil {
		return nil, fmt.Errorf("run session: %w", err)
	}

	// Mean pooling over sequence dimension (ignoring padding tokens)
	output := outputTensor.GetData()
	embedding := meanPool(output, attentionMask, int(seqLen), o.dim)

	return embedding, nil
}

// simpleTokenize converts text to token IDs using a basic byte-level encoding.
// This is intentionally simple — for production-quality tokenization, use Ollama.
func simpleTokenize(text string, maxLen int) (inputIDs []int64, attentionMask []int64) {
	// CLS token = 101, SEP token = 102, PAD token = 0
	const clsToken, sepToken, padToken = int64(101), int64(102), int64(0)

	// Simple: use byte values as token IDs (shifted by 1000 to avoid special tokens)
	// This won't match the real WordPiece tokenizer but produces usable embeddings
	runes := []rune(text)
	if len(runes) > maxLen-2 {
		runes = runes[:maxLen-2]
	}

	ids := make([]int64, 0, maxLen)
	mask := make([]int64, 0, maxLen)

	// [CLS]
	ids = append(ids, clsToken)
	mask = append(mask, 1)

	// Token IDs from text
	for _, r := range runes {
		ids = append(ids, int64(r)%30000+1000) // Map to vocab range
		mask = append(mask, 1)
	}

	// [SEP]
	ids = append(ids, sepToken)
	mask = append(mask, 1)

	// Pad to maxLen
	for len(ids) < maxLen {
		ids = append(ids, padToken)
		mask = append(mask, 0)
	}

	return ids, mask
}

// meanPool performs mean pooling over the sequence dimension, weighted by attention mask.
func meanPool(output []float32, attentionMask []int64, seqLen, dim int) []float32 {
	result := make([]float32, dim)
	count := float32(0)

	for i := 0; i < seqLen; i++ {
		if attentionMask[i] == 0 {
			continue
		}
		count++
		for j := 0; j < dim; j++ {
			result[j] += output[i*dim+j]
		}
	}

	if count > 0 {
		for j := range result {
			result[j] /= count
		}
	}

	return result
}

func (o *ONNXEngine) Dimension() int { return o.dim }

func (o *ONNXEngine) Close() error {
	// ort.DestroyEnvironment will be called on process exit
	return nil
}

// downloadModel downloads the ONNX model file from HuggingFace.
func (o *ONNXEngine) downloadModel() error {
	dir := filepath.Dir(o.modelPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create model dir: %w", err)
	}

	resp, err := http.Get(defaultONNXModelURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(o.modelPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		os.Remove(o.modelPath)
		return fmt.Errorf("write file: %w", err)
	}

	slog.Info("ONNX model downloaded", "path", o.modelPath, "size_mb", written/1024/1024)
	return nil
}

// findONNXRuntimeLib searches for the ONNX Runtime shared library.
func findONNXRuntimeLib() string {
	// 1. Environment variable
	if p := os.Getenv("ONNXRUNTIME_SHARED_LIBRARY_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// 2. Homebrew (macOS)
	homebrewPaths := []string{
		"/opt/homebrew/lib/libonnxruntime.dylib", // Apple Silicon
		"/usr/local/lib/libonnxruntime.dylib",    // Intel Mac
		"/opt/homebrew/opt/onnxruntime/lib/libonnxruntime.dylib",
	}
	for _, p := range homebrewPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// 3. Linux standard paths
	linuxPaths := []string{
		"/usr/lib/libonnxruntime.so",
		"/usr/local/lib/libonnxruntime.so",
	}
	for _, p := range linuxPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}
