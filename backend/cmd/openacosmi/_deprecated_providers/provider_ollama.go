package main

// provider_ollama.go — Ollama 本地 LLM provider 配置

import (
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 模型常量 ----------

const (
	OllamaBaseURL         = "http://localhost:11434/v1"
	OllamaDefaultModelID  = "llama3.2"
	OllamaDefaultModelRef = "ollama/llama3.2"
)

// ---------- Provider 配置 ----------

// ApplyOllamaProviderConfig 注册本地 Ollama provider 及常用模型。
func ApplyOllamaProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, OllamaDefaultModelRef, "Ollama")
	p := ensureProvider(cfg, "ollama")
	if p.BaseURL == "" {
		p.BaseURL = OllamaBaseURL
	}
	p.API = "openai-completions"
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "llama3.2",
		Name:          "Llama 3.2",
		ContextWindow: 128_000,
		MaxTokens:     8_192,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "llama3.1",
		Name:          "Llama 3.1",
		ContextWindow: 128_000,
		MaxTokens:     8_192,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "qwen2.5-coder",
		Name:          "Qwen 2.5 Coder",
		ContextWindow: 128_000,
		MaxTokens:     8_192,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "deepseek-r1",
		Name:          "DeepSeek R1",
		Reasoning:     true,
		ContextWindow: 128_000,
		MaxTokens:     8_192,
	})
}

// ApplyOllamaConfig 注册 Ollama 并设为默认。
func ApplyOllamaConfig(cfg *types.OpenAcosmiConfig) {
	ApplyOllamaProviderConfig(cfg)
	setDefaultModel(cfg, OllamaDefaultModelRef)
}
