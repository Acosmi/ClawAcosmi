package memory

import (
	"bufio"
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

	"github.com/openacosmi/claw-acismi/pkg/retry"
)

// ── Types ──

// VoyageBatchRequest is a single request for Voyage batch embedding.
type VoyageBatchRequest struct {
	CustomID string             `json:"custom_id"`
	Body     VoyageBatchReqBody `json:"body"`
}

// VoyageBatchReqBody contains the input text(s).
type VoyageBatchReqBody struct {
	Input any `json:"input"` // string or []string
}

// VoyageBatchStatus is the batch job status from Voyage.
type VoyageBatchStatus struct {
	ID           string  `json:"id,omitempty"`
	Status       string  `json:"status,omitempty"`
	OutputFileID *string `json:"output_file_id,omitempty"`
	ErrorFileID  *string `json:"error_file_id,omitempty"`
}

// VoyageBatchOutputLine is a single line in Voyage batch output.
type VoyageBatchOutputLine struct {
	CustomID string                     `json:"custom_id,omitempty"`
	Response *VoyageBatchOutputResponse `json:"response,omitempty"`
	Error    *BatchErrorDetail          `json:"error,omitempty"`
}

// VoyageBatchOutputResponse is the response part.
type VoyageBatchOutputResponse struct {
	StatusCode int                        `json:"status_code,omitempty"`
	Body       *VoyageBatchOutputRespBody `json:"body,omitempty"`
}

// VoyageBatchOutputRespBody is the body of a Voyage batch response.
type VoyageBatchOutputRespBody struct {
	Data  []VoyageBatchEmbedEntry `json:"data,omitempty"`
	Error *BatchErrorDetail       `json:"error,omitempty"`
}

// VoyageBatchEmbedEntry is a single embedding entry.
type VoyageBatchEmbedEntry struct {
	Embedding []float64 `json:"embedding,omitempty"`
	Index     int       `json:"index,omitempty"`
}

// VoyageBatchClientConfig extends BatchClientConfig with Voyage-specific fields.
type VoyageBatchClientConfig struct {
	BatchClientConfig
	Model string
}

// ── Constants ──

const (
	voyageBatchEndpoint         = "/v1/embeddings"
	voyageBatchCompletionWindow = "12h"
	voyageBatchMaxRequests      = 50000
)

// ── Functions ──

// SplitVoyageBatchRequests splits requests into groups.
func SplitVoyageBatchRequests(requests []VoyageBatchRequest) [][]VoyageBatchRequest {
	if len(requests) <= voyageBatchMaxRequests {
		return [][]VoyageBatchRequest{requests}
	}
	var groups [][]VoyageBatchRequest
	for i := 0; i < len(requests); i += voyageBatchMaxRequests {
		end := i + voyageBatchMaxRequests
		if end > len(requests) {
			end = len(requests)
		}
		groups = append(groups, requests[i:end])
	}
	return groups
}

// submitVoyageBatch uploads JSONL and creates a Voyage batch job.
func submitVoyageBatch(ctx context.Context, cfg VoyageBatchClientConfig, requests []VoyageBatchRequest, agentID string) (*VoyageBatchStatus, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")

	// Build JSONL
	var jsonl bytes.Buffer
	for i, req := range requests {
		if i > 0 {
			jsonl.WriteByte('\n')
		}
		line, _ := json.Marshal(req)
		jsonl.Write(line)
	}

	// Upload as multipart form
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("purpose", "batch")
	filename := fmt.Sprintf("memory-embeddings.%s.jsonl", HashText(fmt.Sprintf("%d", time.Now().UnixMilli())))
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	part.Write(jsonl.Bytes())
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
		return nil, fmt.Errorf("voyage batch file upload: %w", err)
	}
	defer uploadResp.Body.Close()
	if uploadResp.StatusCode >= 400 {
		text, _ := io.ReadAll(uploadResp.Body)
		return nil, fmt.Errorf("voyage batch file upload failed: %d %s", uploadResp.StatusCode, text)
	}
	var filePayload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(uploadResp.Body).Decode(&filePayload); err != nil {
		return nil, err
	}
	if filePayload.ID == "" {
		return nil, fmt.Errorf("voyage batch file upload failed: missing file id")
	}

	// Create batch with retry
	batchStatus, err := retry.DoWithResult(ctx, retry.Config{
		MaxAttempts:  3,
		InitialDelay: 300 * time.Millisecond,
		MaxDelay:     2 * time.Second,
		JitterFactor: 0.1,
		ShouldRetry: func(err error, _ int) bool {
			msg := err.Error()
			return strings.Contains(msg, "429") || strings.Contains(msg, "5")
		},
	}, func(_ int) (*VoyageBatchStatus, error) {
		batchBody, _ := json.Marshal(map[string]any{
			"input_file_id":     filePayload.ID,
			"endpoint":          voyageBatchEndpoint,
			"completion_window": voyageBatchCompletionWindow,
			"request_params": map[string]string{
				"model":      cfg.Model,
				"input_type": "document",
			},
			"metadata": map[string]string{
				"source": "clawdbot-memory",
				"agent":  agentID,
			},
		})
		req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/batches", bytes.NewReader(batchBody))
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
			return nil, fmt.Errorf("voyage batch create failed: %d %s", resp.StatusCode, text)
		}
		var status VoyageBatchStatus
		json.NewDecoder(resp.Body).Decode(&status)
		return &status, nil
	})
	if err != nil {
		return nil, err
	}
	return batchStatus, nil
}

// fetchVoyageBatchStatus polls the Voyage batch status.
func fetchVoyageBatchStatus(ctx context.Context, cfg VoyageBatchClientConfig, batchID string) (*VoyageBatchStatus, error) {
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
		return nil, fmt.Errorf("voyage batch status failed: %d %s", resp.StatusCode, text)
	}
	var status VoyageBatchStatus
	json.NewDecoder(resp.Body).Decode(&status)
	return &status, nil
}

// waitForVoyageBatch polls until the Voyage batch completes or fails.
func waitForVoyageBatch(ctx context.Context, cfg VoyageBatchClientConfig, batchID string, wait bool, pollInterval, timeout time.Duration, logger *slog.Logger) (string, string, error) {
	start := time.Now()
	for {
		status, err := fetchVoyageBatchStatus(ctx, cfg, batchID)
		if err != nil {
			return "", "", err
		}
		state := status.Status
		if state == "completed" {
			if status.OutputFileID == nil || *status.OutputFileID == "" {
				return "", "", fmt.Errorf("voyage batch %s completed without output file", batchID)
			}
			eid := ""
			if status.ErrorFileID != nil {
				eid = *status.ErrorFileID
			}
			return *status.OutputFileID, eid, nil
		}
		if state == "failed" || state == "expired" || state == "cancelled" || state == "canceled" {
			return "", "", fmt.Errorf("voyage batch %s %s", batchID, state)
		}
		if !wait {
			return "", "", fmt.Errorf("voyage batch %s still %s; wait disabled", batchID, state)
		}
		if time.Since(start) > timeout {
			return "", "", fmt.Errorf("voyage batch %s timed out after %v", batchID, timeout)
		}
		if logger != nil {
			logger.Debug("voyage batch polling", "batchId", batchID, "status", state)
		}
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// RunVoyageEmbeddingBatches is the top-level entry point for Voyage batch embeddings.
// Note: Voyage output uses streaming JSONL parsing (line-by-line) matching the TS version.
func RunVoyageEmbeddingBatches(ctx context.Context, params VoyageBatchParams) (map[string][]float64, error) {
	if len(params.Requests) == 0 {
		return map[string][]float64{}, nil
	}
	groups := SplitVoyageBatchRequests(params.Requests)
	result := make(map[string][]float64)

	type groupResult struct{}
	tasks := make([]func() (groupResult, error), len(groups))
	for gi, group := range groups {
		gi, group := gi, group
		tasks[gi] = func() (groupResult, error) {
			batchInfo, err := submitVoyageBatch(ctx, params.Client, group, params.AgentID)
			if err != nil {
				return groupResult{}, err
			}
			if batchInfo.ID == "" {
				return groupResult{}, fmt.Errorf("voyage batch create failed: missing batch id")
			}
			if !params.Wait && batchInfo.Status != "completed" {
				return groupResult{}, fmt.Errorf("voyage batch %s submitted; enable remote.batch.wait to await completion", batchInfo.ID)
			}

			outputFileID, _, err := waitForVoyageBatch(ctx, params.Client, batchInfo.ID, params.Wait, params.PollInterval, params.Timeout, params.Logger)
			if err != nil {
				return groupResult{}, err
			}

			// Stream-parse output (matching TS readline approach)
			baseURL := strings.TrimRight(params.Client.BaseURL, "/")
			contentReq, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/files/"+outputFileID+"/content", nil)
			if err != nil {
				return groupResult{}, err
			}
			for k, v := range params.Client.Headers {
				contentReq.Header.Set(k, v)
			}
			contentResp, err := http.DefaultClient.Do(contentReq)
			if err != nil {
				return groupResult{}, err
			}
			defer contentResp.Body.Close()
			if contentResp.StatusCode >= 400 {
				text, _ := io.ReadAll(contentResp.Body)
				return groupResult{}, fmt.Errorf("voyage batch file content failed: %d %s", contentResp.StatusCode, text)
			}

			remaining := make(map[string]struct{}, len(group))
			for _, r := range group {
				remaining[r.CustomID] = struct{}{}
			}
			var errors []string

			scanner := bufio.NewScanner(contentResp.Body)
			for scanner.Scan() {
				raw := strings.TrimSpace(scanner.Text())
				if raw == "" {
					continue
				}
				var line VoyageBatchOutputLine
				if err := json.Unmarshal([]byte(raw), &line); err != nil {
					continue
				}
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
				return groupResult{}, fmt.Errorf("voyage batch %s failed: %s", batchInfo.ID, strings.Join(errors, "; "))
			}
			if len(remaining) > 0 {
				return groupResult{}, fmt.Errorf("voyage batch %s missing %d embedding responses", batchInfo.ID, len(remaining))
			}
			_ = gi
			return groupResult{}, nil
		}
	}

	if _, err := RunWithConcurrency(ctx, tasks, params.Concurrency); err != nil {
		return nil, err
	}
	return result, nil
}

// VoyageBatchParams holds parameters for RunVoyageEmbeddingBatches.
type VoyageBatchParams struct {
	Client       VoyageBatchClientConfig
	AgentID      string
	Requests     []VoyageBatchRequest
	Wait         bool
	PollInterval time.Duration
	Timeout      time.Duration
	Concurrency  int
	Logger       *slog.Logger
}
