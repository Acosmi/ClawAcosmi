// Package memory implements a ChromaDB-backed vector store for
// keyframe embedding and semantic retrieval.
//
// Uses direct HTTP REST calls instead of SDK libraries.
package memory

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// KeyframeVectorStore stores keyframe metadata and descriptions
// as vector embeddings in ChromaDB for semantic retrieval.
type KeyframeVectorStore struct {
	baseURL        string
	collectionName string
	collectionID   string // ChromaDB internal UUID
	client         *http.Client
	connected      bool
	mu             sync.RWMutex
}

// NewKeyframeVectorStore creates a store pointing at the given ChromaDB server.
func NewKeyframeVectorStore(host string, port int, collectionName string) *KeyframeVectorStore {
	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 8000
	}
	if collectionName == "" {
		collectionName = "argus_keyframes"
	}
	return &KeyframeVectorStore{
		baseURL:        fmt.Sprintf("http://%s:%d", host, port),
		collectionName: collectionName,
		client:         &http.Client{Timeout: 10 * time.Second},
	}
}

// Connect establishes connection to ChromaDB and creates/gets the collection.
func (s *KeyframeVectorStore) Connect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create collection
	body := map[string]any{
		"name":     s.collectionName,
		"metadata": map[string]string{"hnsw:space": "cosine"},
	}
	jsonBody, _ := json.Marshal(body)

	resp, err := s.client.Post(s.baseURL+"/api/v1/collections", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		log.Printf("[VectorStore] ChromaDB connection failed: %v", err)
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 409 {
		// Collection already exists, get it
		getResp, err := s.client.Get(fmt.Sprintf("%s/api/v1/collections/%s", s.baseURL, s.collectionName))
		if err != nil {
			return err
		}
		defer getResp.Body.Close()
		respBody, _ = io.ReadAll(getResp.Body)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parse collection response: %w", err)
	}
	s.collectionID = result.ID
	s.connected = true

	// Log with inline count (avoid calling Count() which would RLock → deadlock)
	countResp, err := s.client.Get(fmt.Sprintf("%s/api/v1/collections/%s/count", s.baseURL, s.collectionID))
	itemCount := 0
	if err == nil {
		defer countResp.Body.Close()
		json.NewDecoder(countResp.Body).Decode(&itemCount)
	}
	log.Printf("[VectorStore] Connected to ChromaDB at %s, collection=%s (%d items)",
		s.baseURL, s.collectionName, itemCount)
	return nil
}

// IsConnected returns true if the store is connected.
func (s *KeyframeVectorStore) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

// AddKeyframe stores a keyframe's description as a vector embedding.
func (s *KeyframeVectorStore) AddKeyframe(
	frameNo int,
	timestamp float64,
	description string,
	triggerReason string,
	thumbnailJPEG []byte,
	metadata map[string]any,
) string {
	if !s.IsConnected() {
		return ""
	}

	docID := fmt.Sprintf("kf_%d_%d", frameNo, int(timestamp*1000))

	meta := map[string]any{
		"frame_no":       frameNo,
		"timestamp":      timestamp,
		"trigger_reason": triggerReason,
		"stored_at":      float64(time.Now().Unix()),
	}
	if len(thumbnailJPEG) > 0 {
		hash := md5.Sum(thumbnailJPEG)
		meta["thumbnail_hash"] = hex.EncodeToString(hash[:])
		meta["thumbnail_size"] = len(thumbnailJPEG)
	}
	// Flatten metadata (ChromaDB only supports scalar values)
	for k, v := range metadata {
		switch v := v.(type) {
		case string, int, float64, bool:
			meta["meta_"+k] = v
		}
	}

	body := map[string]any{
		"ids":       []string{docID},
		"documents": []string{description},
		"metadatas": []map[string]any{meta},
	}
	jsonBody, _ := json.Marshal(body)

	url := fmt.Sprintf("%s/api/v1/collections/%s/upsert", s.baseURL, s.collectionID)
	resp, err := s.client.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		log.Printf("[VectorStore] Failed to store keyframe: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[VectorStore] Upsert failed (status %d): %s", resp.StatusCode, string(body))
		return ""
	}

	return docID
}

// QueryResult holds a single semantic search hit.
type QueryResult struct {
	ID          string         `json:"id"`
	Description string         `json:"description"`
	Distance    float64        `json:"distance"`
	Metadata    map[string]any `json:"metadata"`
}

// Query performs semantic search over stored keyframes.
func (s *KeyframeVectorStore) Query(text string, nResults int, timeRange *[2]float64) []QueryResult {
	if !s.IsConnected() {
		return nil
	}
	if nResults <= 0 {
		nResults = 5
	}

	body := map[string]any{
		"query_texts": []string{text},
		"n_results":   nResults,
	}
	if timeRange != nil {
		body["where"] = map[string]any{
			"$and": []map[string]any{
				{"timestamp": map[string]any{"$gte": timeRange[0]}},
				{"timestamp": map[string]any{"$lte": timeRange[1]}},
			},
		}
	}

	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/v1/collections/%s/query", s.baseURL, s.collectionID)
	resp, err := s.client.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		log.Printf("[VectorStore] Query failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		IDs       [][]string         `json:"ids"`
		Documents [][]string         `json:"documents"`
		Distances [][]float64        `json:"distances"`
		Metadatas [][]map[string]any `json:"metadatas"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[VectorStore] Query response parse failed: %v", err)
		return nil
	}

	if len(result.IDs) == 0 || len(result.IDs[0]) == 0 {
		return nil
	}

	hits := make([]QueryResult, 0, len(result.IDs[0]))
	for i, id := range result.IDs[0] {
		qr := QueryResult{ID: id}
		if len(result.Documents) > 0 && len(result.Documents[0]) > i {
			qr.Description = result.Documents[0][i]
		}
		if len(result.Distances) > 0 && len(result.Distances[0]) > i {
			qr.Distance = result.Distances[0][i]
		}
		if len(result.Metadatas) > 0 && len(result.Metadatas[0]) > i {
			qr.Metadata = result.Metadatas[0][i]
		}
		hits = append(hits, qr)
	}
	return hits
}

// Count returns the number of stored keyframes.
func (s *KeyframeVectorStore) Count() int {
	if !s.IsConnected() {
		return 0
	}

	resp, err := s.client.Get(fmt.Sprintf("%s/api/v1/collections/%s/count", s.baseURL, s.collectionID))
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var count int
	json.NewDecoder(resp.Body).Decode(&count)
	return count
}

// Clear deletes all stored keyframes and recreates the collection.
func (s *KeyframeVectorStore) Clear() error {
	if !s.IsConnected() {
		return nil
	}

	// Delete collection
	req, _ := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/api/v1/collections/%s", s.baseURL, s.collectionName), nil)
	s.client.Do(req)

	// Recreate
	s.connected = false
	return s.Connect()
}

// Close releases resources.
func (s *KeyframeVectorStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connected = false
	s.collectionID = ""
	log.Printf("[VectorStore] Closed")
}
