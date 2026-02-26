package embedding

import (
	"context"

	"github.com/uhms/local-proxy/internal/cloud"
)

// CloudEngine wraps the existing cloud.Client.Embed() as an EmbedEngine.
// This is the fallback engine when local engines (Ollama, ONNX) are unavailable.
type CloudEngine struct {
	client  *cloud.Client
	monitor *cloud.Monitor // may be nil
}

// NewCloudEngine creates a cloud-backed embedding engine.
// The monitor parameter is optional — if nil, availability is based solely on client configuration.
func NewCloudEngine(client *cloud.Client, monitor *cloud.Monitor) *CloudEngine {
	return &CloudEngine{
		client:  client,
		monitor: monitor,
	}
}

func (c *CloudEngine) Name() string { return "cloud" }

// Available returns true if the cloud client is configured and the connection is online.
func (c *CloudEngine) Available() bool {
	if !c.client.IsConfigured() {
		return false
	}
	if c.monitor != nil {
		return c.monitor.IsOnline()
	}
	return true
}

// Embed delegates to cloud.Client.Embed() and unwraps the response.
func (c *CloudEngine) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := c.client.Embed(ctx, texts)
	if err != nil {
		return nil, err
	}
	return resp.Embeddings, nil
}

// Dimension returns 0 since the cloud dimension depends on the server-side model.
func (c *CloudEngine) Dimension() int { return 0 }

// Close is a no-op — the cloud.Client lifecycle is managed externally.
func (c *CloudEngine) Close() error { return nil }
