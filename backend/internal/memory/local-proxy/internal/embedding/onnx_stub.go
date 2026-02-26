//go:build !onnx

package embedding

// NewONNXEngineFromConfig returns nil when built without the "onnx" build tag.
// To enable ONNX support, build with: go build -tags onnx
func NewONNXEngineFromConfig(modelPath string) EmbedEngine {
	return nil
}
