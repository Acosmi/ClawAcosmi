// Package algo — Algorithm service implementation.
// Wraps existing EmbeddingService, LLMProvider, and RerankService
// to expose stateless computation endpoints. No database access.
package algo

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uhms/go-api/internal/services"
)

// Service orchestrates pure algorithm computations.
// It holds references to the existing service singletons but NEVER
// accesses any database. All methods are stateless and idempotent.
type Service struct {
	embedSvc  services.EmbeddingService
	llm       services.LLMProvider
	rerankSvc *services.RerankService
}

// NewService creates an Algorithm service from existing service dependencies.
func NewService(
	embedSvc services.EmbeddingService,
	llm services.LLMProvider,
	rerankSvc *services.RerankService,
) *Service {
	return &Service{
		embedSvc:  embedSvc,
		llm:       llm,
		rerankSvc: rerankSvc,
	}
}

// Embed generates vector embeddings for the given texts.
func (s *Service) Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error) {
	if s.embedSvc == nil {
		return nil, fmt.Errorf("algo: embedding service not available")
	}

	embeddings, err := s.embedSvc.EmbedDocuments(ctx, req.Texts)
	if err != nil {
		slog.Error("algo.Embed failed", "error", err, "count", len(req.Texts))
		return nil, fmt.Errorf("algo: embed failed: %w", err)
	}

	return &EmbedResponse{
		Embeddings: embeddings,
		Dimension:  s.embedSvc.Dimension(),
	}, nil
}

// Classify performs NLP classification and importance scoring on the given text.
func (s *Service) Classify(ctx context.Context, req *ClassifyRequest) (*ClassifyResponse, error) {
	if s.llm == nil {
		return nil, fmt.Errorf("algo: LLM service not available")
	}

	// Score importance
	importance, err := s.llm.ScoreImportance(ctx, req.Content)
	if err != nil {
		slog.Error("algo.Classify importance scoring failed", "error", err)
		return nil, fmt.Errorf("algo: classify failed: %w", err)
	}

	// Use the LLM to classify category
	categoryPrompt := fmt.Sprintf(
		`Classify the following text into exactly ONE category from: preference, habit, profile, skill, relationship, event, opinion, fact, goal, task, reminder, insight.

Text: %s

Return a JSON object with a single "category" field. Example: {"category": "preference"}
Return ONLY valid JSON, no markdown formatting.`, req.Content)

	categoryResult, err := s.llm.Generate(ctx, categoryPrompt)
	if err != nil {
		slog.Warn("algo.Classify category detection failed, using default", "error", err)
		return &ClassifyResponse{
			Category:        "fact",
			ImportanceScore: importance.Score,
			Reasoning:       importance.Reasoning,
		}, nil
	}

	category := extractCategoryFromJSON(categoryResult)

	return &ClassifyResponse{
		Category:        category,
		ImportanceScore: importance.Score,
		Reasoning:       importance.Reasoning,
	}, nil
}

// Rank performs semantic reranking of candidate documents against a query.
func (s *Service) Rank(ctx context.Context, req *RankRequest) (*RankResponse, error) {
	if s.rerankSvc == nil {
		return nil, fmt.Errorf("algo: rerank service not available")
	}

	topN := req.TopN
	if topN <= 0 {
		topN = len(req.Documents)
	}

	results, err := s.rerankSvc.Rerank(ctx, req.Query, req.Documents, topN)
	if err != nil {
		slog.Error("algo.Rank failed", "error", err)
		return nil, fmt.Errorf("algo: rank failed: %w", err)
	}

	rankResults := make([]RankResult, len(results))
	for i, r := range results {
		rankResults[i] = RankResult{
			Index: r.Index,
			Score: r.Score,
		}
	}

	return &RankResponse{Results: rankResults}, nil
}

// Reflect generates a reflection from a set of memory summaries.
func (s *Service) Reflect(ctx context.Context, req *ReflectRequest) (*ReflectResponse, error) {
	if s.llm == nil {
		return nil, fmt.Errorf("algo: LLM service not available")
	}

	reflection, err := s.llm.GenerateReflection(ctx, req.Memories, req.CoreMemoryContext)
	if err != nil {
		slog.Error("algo.Reflect failed", "error", err)
		return nil, fmt.Errorf("algo: reflect failed: %w", err)
	}

	// Parse the reflection response for core memory edits
	resp := &ReflectResponse{
		Reflection: reflection,
	}

	// Try to extract core memory edits from the structured response
	edits := extractCoreMemoryEdits(reflection)
	if edits != nil {
		resp.CoreMemoryEdits = edits
		resp.Reflection = extractReflectionText(reflection)
	}

	return resp, nil
}

// Extract performs entity and relation extraction for knowledge graph construction.
func (s *Service) Extract(ctx context.Context, req *ExtractRequest) (*ExtractResponse, error) {
	if s.llm == nil {
		return nil, fmt.Errorf("algo: LLM service not available")
	}

	result, err := s.llm.ExtractEntities(ctx, req.Content)
	if err != nil {
		slog.Error("algo.Extract failed", "error", err)
		return nil, fmt.Errorf("algo: extract failed: %w", err)
	}

	entities := make([]ExtractedEntity, len(result.Entities))
	for i, e := range result.Entities {
		entities[i] = ExtractedEntity{
			Name:        e.Name,
			EntityType:  e.EntityType,
			Description: e.Description,
		}
	}

	relations := make([]ExtractedRelation, len(result.Relations))
	for i, r := range result.Relations {
		relations[i] = ExtractedRelation{
			Source:       r.Source,
			Target:       r.Target,
			RelationType: r.RelationType,
		}
	}

	return &ExtractResponse{
		Entities:  entities,
		Relations: relations,
	}, nil
}

// Health returns the health status of algorithm subsystems.
func (s *Service) Health() *HealthResponse {
	resp := &HealthResponse{Status: "ok"}

	resp.Embedding = s.embedSvc != nil
	resp.LLM = s.llm != nil
	resp.Rerank = s.rerankSvc != nil

	if s.embedSvc != nil {
		resp.EmbedDimension = s.embedSvc.Dimension()
	}

	if !resp.Embedding || !resp.LLM {
		resp.Status = "degraded"
	}

	return resp
}
