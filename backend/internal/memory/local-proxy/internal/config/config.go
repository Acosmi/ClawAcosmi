// Package config — local-proxy configuration.
package config

import (
	"os"
	"strconv"
)

// Config holds all configuration for the local proxy.
type Config struct {
	// CloudURL is the base URL of the cloud UHMS Gateway (e.g., "https://your-uhms.com/api/v1").
	CloudURL string

	// CloudAPIKey is the API key / Bearer token for authenticating to cloud algo API.
	CloudAPIKey string

	// DBPath is the path to the local SQLite database file.
	DBPath string

	// ListenAddr is the address to listen on for the local MCP server (default: "127.0.0.1:19002").
	ListenAddr string

	// UserID is the default user ID for local operations.
	UserID string

	// EmbedMode controls the embedding engine selection strategy.
	// "auto" (default) = Ollama → Cloud, "ollama" = Ollama only, "cloud" = Cloud only.
	EmbedMode string

	// OllamaURL is the Ollama API base URL (default: "http://localhost:11434").
	OllamaURL string

	// OllamaModel is the Ollama embedding model name (default: "all-minilm").
	OllamaModel string

	// ONNXModelPath is the path to the ONNX embedding model file.
	// Empty string disables ONNX engine. Requires build tag "onnx".
	ONNXModelPath string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	cfg := &Config{
		CloudURL:      envOr("UHMS_CLOUD_URL", "http://localhost:19001/api/v1"),
		CloudAPIKey:   envOr("UHMS_CLOUD_API_KEY", ""),
		DBPath:        envOr("UHMS_LOCAL_DB", "~/.uhms/local.db"),
		ListenAddr:    envOr("UHMS_LOCAL_LISTEN", "127.0.0.1:19002"),
		UserID:        envOr("UHMS_USER_ID", "default"),
		EmbedMode:     envOr("UHMS_EMBED_MODE", "auto"),
		OllamaURL:     envOr("UHMS_OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:   envOr("UHMS_OLLAMA_MODEL", "all-minilm"),
		ONNXModelPath: envOr("UHMS_ONNX_MODEL_PATH", ""),
	}
	return cfg
}

// StdioMode returns true if the proxy should run in stdio MCP mode (for Claude Desktop etc.).
func StdioMode() bool {
	v, _ := strconv.ParseBool(os.Getenv("UHMS_STDIO_MODE"))
	return v || (len(os.Args) > 1 && os.Args[1] == "mcp")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
