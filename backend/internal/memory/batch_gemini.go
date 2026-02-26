package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ── Types ──

// GeminiBatchRequest is a single request for Gemini batch embedding.
type GeminiBatchRequest struct {
	CustomID string `json:"custom_id"`
	Content  struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"content"`
	TaskType string `json:"taskType"` // "RETRIEVAL_DOCUMENT" | "RETRIEVAL_QUERY"
}

// GeminiBatchStatus is the batch job status from Gemini.
type GeminiBatchStatus struct {
	Name         string                   `json:"name,omitempty"`
	State        string                   `json:"state,omitempty"`
	OutputConfig *GeminiBatchOutputConfig `json:"outputConfig,omitempty"`
	Metadata     *GeminiBatchMetadata     `json:"metadata,omitempty"`
	Error        *BatchErrorDetail        `json:"error,omitempty"`
}

// GeminiBatchOutputConfig holds output file references.
type GeminiBatchOutputConfig struct {
	File   string `json:"file,omitempty"`
	FileID string `json:"fileId,omitempty"`
}

// GeminiBatchMetadata holds metadata from Gemini batch response.
type GeminiBatchMetadata struct {
	Output *GeminiBatchMetadataOutput `json:"output,omitempty"`
}

// GeminiBatchMetadataOutput holds the output file path.
type GeminiBatchMetadataOutput struct {
	ResponsesFile string `json:"responsesFile,omitempty"`
}

// GeminiBatchOutputLine is a single line in Gemini batch output.
type GeminiBatchOutputLine struct {
	Key       string                     `json:"key,omitempty"`
	CustomID  string                     `json:"custom_id,omitempty"`
	RequestID string                     `json:"request_id,omitempty"`
	Embedding *GeminiBatchEmbedding      `json:"embedding,omitempty"`
	Response  *GeminiBatchOutputResponse `json:"response,omitempty"`
	Error     *BatchErrorDetail          `json:"error,omitempty"`
}

// GeminiBatchEmbedding holds embedding values.
type GeminiBatchEmbedding struct {
	Values []float64 `json:"values,omitempty"`
}

// GeminiBatchOutputResponse is the response wrapper.
type GeminiBatchOutputResponse struct {
	Embedding *GeminiBatchEmbedding `json:"embedding,omitempty"`
	Error     *BatchErrorDetail     `json:"error,omitempty"`
}

// GeminiBatchClientConfig extends BatchClientConfig with Gemini-specific fields.
type GeminiBatchClientConfig struct {
	BatchClientConfig
	ModelPath string // e.g. "models/text-embedding-004"
}

// ── Constants ──

const geminiBatchMaxRequests = 50000

// ── Functions ──

// SplitGeminiBatchRequests splits requests into groups.
func SplitGeminiBatchRequests(requests []GeminiBatchRequest) [][]GeminiBatchRequest {
	if len(requests) <= geminiBatchMaxRequests {
		return [][]GeminiBatchRequest{requests}
	}
	var groups [][]GeminiBatchRequest
	for i := 0; i < len(requests); i += geminiBatchMaxRequests {
		end := i + geminiBatchMaxRequests
		if end > len(requests) {
			end = len(requests)
		}
		groups = append(groups, requests[i:end])
	}
	return groups
}

// getGeminiUploadURL derives the upload URL from the base URL.
func getGeminiUploadURL(baseURL string) string {
	if strings.Contains(baseURL, "/v1beta") {
		return strings.Replace(baseURL, "/v1beta", "/upload/v1beta", 1)
	}
	return strings.TrimRight(baseURL, "/") + "/upload"
}

// buildGeminiMultipartBody creates the multipart/related body for file upload.
func buildGeminiMultipartBody(jsonlContent, displayName string) ([]byte, string) {
	boundary := "openacosmi-" + HashText(displayName)
	var buf bytes.Buffer

	jsonMeta, _ := json.Marshal(map[string]any{
		"file": map[string]string{
			"displayName": displayName,
			"mimeType":    "application/jsonl",
		},
	})

	fmt.Fprintf(&buf, "--%s\r\nContent-Type: application/json; charset=UTF-8\r\n\r\n%s\r\n", boundary, jsonMeta)
	fmt.Fprintf(&buf, "--%s\r\nContent-Type: application/jsonl; charset=UTF-8\r\n\r\n%s\r\n", boundary, jsonlContent)
	fmt.Fprintf(&buf, "--%s--\r\n", boundary)

	contentType := "multipart/related; boundary=" + boundary
	return buf.Bytes(), contentType
}

// submitGeminiBatch uploads file and creates a batch job.
func submitGeminiBatch(ctx context.Context, cfg GeminiBatchClientConfig, requests []GeminiBatchRequest, agentID string) (*GeminiBatchStatus, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")

	// Build JSONL
	var jsonl bytes.Buffer
	for i, req := range requests {
		if i > 0 {
			jsonl.WriteByte('\n')
		}
		line, _ := json.Marshal(map[string]any{
			"key":     req.CustomID,
			"request": map[string]any{"content": req.Content, "task_type": req.TaskType},
		})
		jsonl.Write(line)
	}

	displayName := fmt.Sprintf("memory-embeddings-%s", HashText(fmt.Sprintf("%d", time.Now().UnixMilli())))
	body, contentType := buildGeminiMultipartBody(jsonl.String(), displayName)

	// Upload file
	uploadURL := getGeminiUploadURL(baseURL) + "/files?uploadType=multipart"
	uploadReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range cfg.Headers {
		if !strings.EqualFold(k, "content-type") {
			uploadReq.Header.Set(k, v)
		}
	}
	uploadReq.Header.Set("Content-Type", contentType)

	uploadResp, err := http.DefaultClient.Do(uploadReq)
	if err != nil {
		return nil, fmt.Errorf("gemini batch file upload: %w", err)
	}
	defer uploadResp.Body.Close()
	if uploadResp.StatusCode >= 400 {
		text, _ := io.ReadAll(uploadResp.Body)
		return nil, fmt.Errorf("gemini batch file upload failed: %d %s", uploadResp.StatusCode, text)
	}
	var filePayload struct {
		Name string `json:"name"`
		File *struct {
			Name string `json:"name"`
		} `json:"file"`
	}
	if err := json.NewDecoder(uploadResp.Body).Decode(&filePayload); err != nil {
		return nil, fmt.Errorf("gemini batch file upload: decode: %w", err)
	}
	fileID := filePayload.Name
	if fileID == "" && filePayload.File != nil {
		fileID = filePayload.File.Name
	}
	if fileID == "" {
		return nil, fmt.Errorf("gemini batch file upload failed: missing file id")
	}

	// Create batch
	batchBody, _ := json.Marshal(map[string]any{
		"batch": map[string]any{
			"displayName": "memory-embeddings-" + agentID,
			"inputConfig": map[string]string{"file_name": fileID},
		},
	})
	batchEndpoint := fmt.Sprintf("%s/%s:asyncBatchEmbedContent", baseURL, cfg.ModelPath)
	batchReq, err := http.NewRequestWithContext(ctx, "POST", batchEndpoint, bytes.NewReader(batchBody))
	if err != nil {
		return nil, err
	}
	for k, v := range cfg.Headers {
		batchReq.Header.Set(k, v)
	}
	batchReq.Header.Set("Content-Type", "application/json")

	batchResp, err := http.DefaultClient.Do(batchReq)
	if err != nil {
		return nil, err
	}
	defer batchResp.Body.Close()
	if batchResp.StatusCode >= 400 {
		text, _ := io.ReadAll(batchResp.Body)
		if batchResp.StatusCode == 404 {
			return nil, fmt.Errorf("gemini batch create failed: 404 (asyncBatchEmbedContent not available)")
		}
		return nil, fmt.Errorf("gemini batch create failed: %d %s", batchResp.StatusCode, text)
	}
	var status GeminiBatchStatus
	if err := json.NewDecoder(batchResp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

// fetchGeminiBatchStatus polls the status of a Gemini batch job.
func fetchGeminiBatchStatus(ctx context.Context, cfg GeminiBatchClientConfig, batchName string) (*GeminiBatchStatus, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	name := batchName
	if !strings.HasPrefix(name, "batches/") {
		name = "batches/" + name
	}
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/"+name, nil)
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
		return nil, fmt.Errorf("gemini batch status failed: %d %s", resp.StatusCode, text)
	}
	var status GeminiBatchStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

// fetchGeminiFileContent downloads batch output file content.
func fetchGeminiFileContent(ctx context.Context, cfg GeminiBatchClientConfig, fileID string) (string, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	file := fileID
	if !strings.HasPrefix(file, "files/") {
		file = "files/" + file
	}
	downloadURL := baseURL + "/" + file + ":download"
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
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
		return "", fmt.Errorf("gemini batch file content failed: %d %s", resp.StatusCode, text)
	}
	data, err := io.ReadAll(resp.Body)
	return string(data), err
}

// ParseGeminiBatchOutput parses JSONL output into structured lines.
func ParseGeminiBatchOutput(text string) []GeminiBatchOutputLine {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var lines []GeminiBatchOutputLine
	for _, raw := range strings.Split(text, "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var line GeminiBatchOutputLine
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// isGeminiCompleted checks if the state indicates completion.
func isGeminiCompleted(state string) bool {
	return state == "SUCCEEDED" || state == "COMPLETED" || state == "DONE"
}

// isGeminiFailed checks if the state indicates failure.
func isGeminiFailed(state string) bool {
	return state == "FAILED" || state == "CANCELLED" || state == "CANCELED" || state == "EXPIRED"
}

// resolveGeminiOutputFileID extracts the output file ID from status.
func resolveGeminiOutputFileID(s *GeminiBatchStatus) string {
	if s.OutputConfig != nil {
		if s.OutputConfig.File != "" {
			return s.OutputConfig.File
		}
		if s.OutputConfig.FileID != "" {
			return s.OutputConfig.FileID
		}
	}
	if s.Metadata != nil && s.Metadata.Output != nil {
		return s.Metadata.Output.ResponsesFile
	}
	return ""
}

// waitForGeminiBatch polls until the batch completes or fails.
func waitForGeminiBatch(ctx context.Context, cfg GeminiBatchClientConfig, batchName string, wait bool, pollInterval, timeout time.Duration, logger *slog.Logger) (string, error) {
	start := time.Now()
	for {
		status, err := fetchGeminiBatchStatus(ctx, cfg, batchName)
		if err != nil {
			return "", err
		}
		state := status.State
		if isGeminiCompleted(state) {
			fid := resolveGeminiOutputFileID(status)
			if fid == "" {
				return "", fmt.Errorf("gemini batch %s completed without output file", batchName)
			}
			return fid, nil
		}
		if isGeminiFailed(state) {
			msg := "unknown error"
			if status.Error != nil {
				msg = status.Error.Message
			}
			return "", fmt.Errorf("gemini batch %s %s: %s", batchName, state, msg)
		}
		if !wait {
			return "", fmt.Errorf("gemini batch %s still %s; wait disabled", batchName, state)
		}
		if time.Since(start) > timeout {
			return "", fmt.Errorf("gemini batch %s timed out after %v", batchName, timeout)
		}
		if logger != nil {
			logger.Debug("gemini batch polling", "name", batchName, "state", state)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// RunGeminiEmbeddingBatches is the top-level entry point for Gemini batch embeddings.
func RunGeminiEmbeddingBatches(ctx context.Context, params GeminiBatchParams) (map[string][]float64, error) {
	if len(params.Requests) == 0 {
		return map[string][]float64{}, nil
	}
	groups := SplitGeminiBatchRequests(params.Requests)
	result := make(map[string][]float64)

	type groupResult struct{}
	tasks := make([]func() (groupResult, error), len(groups))
	for gi, group := range groups {
		gi, group := gi, group
		tasks[gi] = func() (groupResult, error) {
			batchInfo, err := submitGeminiBatch(ctx, params.Client, group, params.AgentID)
			if err != nil {
				return groupResult{}, err
			}
			batchName := batchInfo.Name
			if batchName == "" {
				return groupResult{}, fmt.Errorf("gemini batch create failed: missing batch name")
			}
			if !params.Wait && !isGeminiCompleted(batchInfo.State) {
				return groupResult{}, fmt.Errorf("gemini batch %s submitted; enable remote.batch.wait to await completion", batchName)
			}

			outputFileID := resolveGeminiOutputFileID(batchInfo)
			if !isGeminiCompleted(batchInfo.State) || outputFileID == "" {
				var waitErr error
				outputFileID, waitErr = waitForGeminiBatch(ctx, params.Client, batchName, params.Wait, params.PollInterval, params.Timeout, params.Logger)
				if waitErr != nil {
					return groupResult{}, waitErr
				}
			}

			content, err := fetchGeminiFileContent(ctx, params.Client, outputFileID)
			if err != nil {
				return groupResult{}, err
			}
			outputLines := ParseGeminiBatchOutput(content)
			remaining := make(map[string]struct{}, len(group))
			for _, r := range group {
				remaining[r.CustomID] = struct{}{}
			}

			var errors []string
			for _, line := range outputLines {
				customID := line.Key
				if customID == "" {
					customID = line.CustomID
				}
				if customID == "" {
					customID = line.RequestID
				}
				if customID == "" {
					continue
				}
				delete(remaining, customID)
				if line.Error != nil && line.Error.Message != "" {
					errors = append(errors, customID+": "+line.Error.Message)
					continue
				}
				if line.Response != nil && line.Response.Error != nil && line.Response.Error.Message != "" {
					errors = append(errors, customID+": "+line.Response.Error.Message)
					continue
				}
				var embedding []float64
				if line.Embedding != nil {
					embedding = line.Embedding.Values
				} else if line.Response != nil && line.Response.Embedding != nil {
					embedding = line.Response.Embedding.Values
				}
				if len(embedding) == 0 {
					errors = append(errors, customID+": empty embedding")
					continue
				}
				result[customID] = embedding
			}
			if len(errors) > 0 {
				return groupResult{}, fmt.Errorf("gemini batch %s failed: %s", batchName, strings.Join(errors, "; "))
			}
			if len(remaining) > 0 {
				return groupResult{}, fmt.Errorf("gemini batch %s missing %d embedding responses", batchName, len(remaining))
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

// GeminiBatchParams holds parameters for RunGeminiEmbeddingBatches.
type GeminiBatchParams struct {
	Client       GeminiBatchClientConfig
	AgentID      string
	Requests     []GeminiBatchRequest
	Wait         bool
	PollInterval time.Duration
	Timeout      time.Duration
	Concurrency  int
	Logger       *slog.Logger
}
