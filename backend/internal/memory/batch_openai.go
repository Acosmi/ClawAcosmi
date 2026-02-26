package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/anthropic/open-acosmi/pkg/retry"
)

// ── Types ──

// OpenAiBatchRequest is a single request line in the JSONL upload.
type OpenAiBatchRequest struct {
	CustomID string             `json:"custom_id"`
	Method   string             `json:"method"`
	URL      string             `json:"url"`
	Body     OpenAiBatchReqBody `json:"body"`
}

// OpenAiBatchReqBody is the body of a single OpenAI batch request.
type OpenAiBatchReqBody struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// OpenAiBatchStatus is the batch job status from OpenAI.
type OpenAiBatchStatus struct {
	ID           string  `json:"id,omitempty"`
	Status       string  `json:"status,omitempty"`
	OutputFileID *string `json:"output_file_id,omitempty"`
	ErrorFileID  *string `json:"error_file_id,omitempty"`
}

// OpenAiBatchOutputLine is a single line in the batch output JSONL.
type OpenAiBatchOutputLine struct {
	CustomID string                     `json:"custom_id,omitempty"`
	Response *OpenAiBatchOutputResponse `json:"response,omitempty"`
	Error    *BatchErrorDetail          `json:"error,omitempty"`
}

// OpenAiBatchOutputResponse is the response part of an output line.
type OpenAiBatchOutputResponse struct {
	StatusCode int                        `json:"status_code,omitempty"`
	Body       *OpenAiBatchOutputRespBody `json:"body,omitempty"`
}

// OpenAiBatchOutputRespBody is the body of a batch response.
type OpenAiBatchOutputRespBody struct {
	Data  []OpenAiBatchEmbedEntry `json:"data,omitempty"`
	Error *BatchErrorDetail       `json:"error,omitempty"`
}

// OpenAiBatchEmbedEntry is a single embedding in the response.
type OpenAiBatchEmbedEntry struct {
	Embedding []float64 `json:"embedding,omitempty"`
	Index     int       `json:"index,omitempty"`
}

// BatchErrorDetail is a reusable error detail struct.
type BatchErrorDetail struct {
	Message string `json:"message,omitempty"`
}

// BatchClientConfig carries HTTP client info for batch operations.
type BatchClientConfig struct {
	BaseURL string
	Headers map[string]string
}

// ── Constants ──

const (
	openAiBatchEndpoint         = "/v1/embeddings"
	openAiBatchCompletionWindow = "24h"
	openAiBatchMaxRequests      = 50000
)

// ── Functions ──

// SplitOpenAiBatchRequests splits requests into groups of at most 50000.
func SplitOpenAiBatchRequests(requests []OpenAiBatchRequest) [][]OpenAiBatchRequest {
	if len(requests) <= openAiBatchMaxRequests {
		return [][]OpenAiBatchRequest{requests}
	}
	var groups [][]OpenAiBatchRequest
	for i := 0; i < len(requests); i += openAiBatchMaxRequests {
		end := i + openAiBatchMaxRequests
		if end > len(requests) {
			end = len(requests)
		}
		groups = append(groups, requests[i:end])
	}
	return groups
}

// submitOpenAiBatch uploads JSONL and creates a batch job.
func submitOpenAiBatch(ctx context.Context, cfg BatchClientConfig, requests []OpenAiBatchRequest, agentID string) (*OpenAiBatchStatus, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")

	// Build JSONL payload
	var jsonl bytes.Buffer
	for i, req := range requests {
		if i > 0 {
			jsonl.WriteByte('\n')
		}
		line, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("openai batch: marshal request: %w", err)
		}
		jsonl.Write(line)
	}

	// Upload as multipart form
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("purpose", "batch")

	filename := fmt.Sprintf("memory-embeddings.%s.jsonl", HashText(fmt.Sprintf("%d", time.Now().UnixMilli())))
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("openai batch: create form file: %w", err)
	}
	if _, err := part.Write(jsonl.Bytes()); err != nil {
		return nil, fmt.Errorf("openai batch: write form file: %w", err)
	}
	w.Close()

	uploadReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/files", &body)
	if err != nil {
		return nil, err
	}
	for k, v := range cfg.Headers {
		if !strings.EqualFold(k, "content-type") {
			uploadReq.Header.Set(k, v)
		}
	}
	uploadReq.Header.Set("Content-Type", w.FormDataContentType())

	uploadResp, err := http.DefaultClient.Do(uploadReq)
	if err != nil {
		return nil, fmt.Errorf("openai batch file upload: %w", err)
	}
	defer uploadResp.Body.Close()
	if uploadResp.StatusCode >= 400 {
		text, _ := io.ReadAll(uploadResp.Body)
		return nil, fmt.Errorf("openai batch file upload failed: %d %s", uploadResp.StatusCode, text)
	}
	var filePayload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(uploadResp.Body).Decode(&filePayload); err != nil {
		return nil, fmt.Errorf("openai batch file upload: decode: %w", err)
	}
	if filePayload.ID == "" {
		return nil, fmt.Errorf("openai batch file upload failed: missing file id")
	}

	// Create batch with retry (429 / 5xx)
	batchStatus, err := retry.DoWithResult(ctx, retry.Config{
		MaxAttempts:  3,
		InitialDelay: 300 * time.Millisecond,
		MaxDelay:     2 * time.Second,
		JitterFactor: 0.1,
		ShouldRetry: func(err error, _ int) bool {
			msg := err.Error()
			return strings.Contains(msg, "429") || strings.Contains(msg, "5")
		},
	}, func(_ int) (*OpenAiBatchStatus, error) {
		batchBody := map[string]any{
			"input_file_id":     filePayload.ID,
			"endpoint":          openAiBatchEndpoint,
			"completion_window": openAiBatchCompletionWindow,
			"metadata": map[string]string{
				"source": "openacosmi-memory",
				"agent":  agentID,
			},
		}
		payload, _ := json.Marshal(batchBody)
		req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/batches", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		for k, v := range cfg.Headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			text, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("openai batch create failed: %d %s", resp.StatusCode, text)
		}
		var status OpenAiBatchStatus
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			return nil, fmt.Errorf("openai batch create: decode: %w", err)
		}
		return &status, nil
	})
	if err != nil {
		return nil, err
	}
	return batchStatus, nil
}

// fetchOpenAiBatchStatus polls the status of a batch job.
func fetchOpenAiBatchStatus(ctx context.Context, cfg BatchClientConfig, batchID string) (*OpenAiBatchStatus, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/batches/"+batchID, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		text, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai batch status failed: %d %s", resp.StatusCode, text)
	}
	var status OpenAiBatchStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

// fetchOpenAiFileContent downloads batch output file content.
func fetchOpenAiFileContent(ctx context.Context, cfg BatchClientConfig, fileID string) (string, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/files/"+fileID+"/content", nil)
	if err != nil {
		return "", err
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		text, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai batch file content failed: %d %s", resp.StatusCode, text)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParseOpenAiBatchOutput parses JSONL output into structured lines.
func ParseOpenAiBatchOutput(text string) []OpenAiBatchOutputLine {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var lines []OpenAiBatchOutputLine
	for _, raw := range strings.Split(text, "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var line OpenAiBatchOutputLine
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// waitForOpenAiBatch polls until the batch completes or fails.
func waitForOpenAiBatch(ctx context.Context, cfg BatchClientConfig, batchID string, wait bool, pollInterval, timeout time.Duration, logger *slog.Logger) (outputFileID string, errorFileID string, err error) {
	start := time.Now()
	for {
		status, fetchErr := fetchOpenAiBatchStatus(ctx, cfg, batchID)
		if fetchErr != nil {
			return "", "", fetchErr
		}
		state := status.Status
		if state == "completed" {
			if status.OutputFileID == nil || *status.OutputFileID == "" {
				return "", "", fmt.Errorf("openai batch %s completed without output file", batchID)
			}
			eid := ""
			if status.ErrorFileID != nil {
				eid = *status.ErrorFileID
			}
			return *status.OutputFileID, eid, nil
		}
		if state == "failed" || state == "expired" || state == "cancelled" || state == "canceled" {
			return "", "", fmt.Errorf("openai batch %s %s", batchID, state)
		}
		if !wait {
			return "", "", fmt.Errorf("openai batch %s still %s; wait disabled", batchID, state)
		}
		if time.Since(start) > timeout {
			return "", "", fmt.Errorf("openai batch %s timed out after %v", batchID, timeout)
		}
		if logger != nil {
			logger.Debug("openai batch polling", "batchId", batchID, "status", state)
		}
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// RunOpenAiEmbeddingBatches is the top-level entry point for OpenAI batch embeddings.
func RunOpenAiEmbeddingBatches(ctx context.Context, params OpenAiBatchParams) (map[string][]float64, error) {
	if len(params.Requests) == 0 {
		return map[string][]float64{}, nil
	}
	groups := SplitOpenAiBatchRequests(params.Requests)
	result := make(map[string][]float64)

	type groupResult struct{}
	tasks := make([]func() (groupResult, error), len(groups))
	for gi, group := range groups {
		gi, group := gi, group
		tasks[gi] = func() (groupResult, error) {
			batchInfo, err := submitOpenAiBatch(ctx, params.Client, group, params.AgentID)
			if err != nil {
				return groupResult{}, err
			}
			if batchInfo.ID == "" {
				return groupResult{}, fmt.Errorf("openai batch create failed: missing batch id")
			}
			if !params.Wait && batchInfo.Status != "completed" {
				return groupResult{}, fmt.Errorf("openai batch %s submitted; enable remote.batch.wait to await completion", batchInfo.ID)
			}

			outputFileID, _, err := waitForOpenAiBatch(ctx, params.Client, batchInfo.ID, params.Wait, params.PollInterval, params.Timeout, params.Logger)
			if err != nil {
				return groupResult{}, err
			}

			content, err := fetchOpenAiFileContent(ctx, params.Client, outputFileID)
			if err != nil {
				return groupResult{}, err
			}
			outputLines := ParseOpenAiBatchOutput(content)
			remaining := make(map[string]struct{}, len(group))
			for _, r := range group {
				remaining[r.CustomID] = struct{}{}
			}

			var errors []string
			for _, line := range outputLines {
				if line.CustomID == "" {
					continue
				}
				delete(remaining, line.CustomID)
				if line.Error != nil && line.Error.Message != "" {
					errors = append(errors, line.CustomID+": "+line.Error.Message)
					continue
				}
				if line.Response != nil && line.Response.StatusCode >= 400 {
					msg := "unknown error"
					if line.Response.Body != nil && line.Response.Body.Error != nil {
						msg = line.Response.Body.Error.Message
					}
					errors = append(errors, line.CustomID+": "+msg)
					continue
				}
				if line.Response != nil && line.Response.Body != nil && len(line.Response.Body.Data) > 0 {
					emb := line.Response.Body.Data[0].Embedding
					if len(emb) == 0 {
						errors = append(errors, line.CustomID+": empty embedding")
						continue
					}
					result[line.CustomID] = emb
				}
			}
			if len(errors) > 0 {
				return groupResult{}, fmt.Errorf("openai batch %s failed: %s", batchInfo.ID, strings.Join(errors, "; "))
			}
			if len(remaining) > 0 {
				return groupResult{}, fmt.Errorf("openai batch %s missing %d embedding responses", batchInfo.ID, len(remaining))
			}
			return groupResult{}, nil
		}
	}

	if _, err := RunWithConcurrency(ctx, tasks, params.Concurrency); err != nil {
		return nil, err
	}
	return result, nil
}

// OpenAiBatchParams holds parameters for RunOpenAiEmbeddingBatches.
type OpenAiBatchParams struct {
	Client       BatchClientConfig
	AgentID      string
	Requests     []OpenAiBatchRequest
	Wait         bool
	PollInterval time.Duration
	Timeout      time.Duration
	Concurrency  int
	Logger       *slog.Logger
}
