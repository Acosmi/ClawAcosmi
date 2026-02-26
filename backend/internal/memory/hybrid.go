package memory

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// HybridVectorResult is a search result from vector search.
type HybridVectorResult struct {
	ID          string
	Path        string
	StartLine   int
	EndLine     int
	Source      string
	Snippet     string
	VectorScore float64
}

// HybridKeywordResult is a search result from FTS keyword search.
type HybridKeywordResult struct {
	ID        string
	Path      string
	StartLine int
	EndLine   int
	Source    string
	Snippet   string
	TextScore float64
}

// HybridMergedResult is the combined result after merging vector+keyword.
type HybridMergedResult struct {
	Path      string  `json:"path"`
	StartLine int     `json:"startLine"`
	EndLine   int     `json:"endLine"`
	Score     float64 `json:"score"`
	Snippet   string  `json:"snippet"`
	Source    string  `json:"source"`
}

var tokenRe = regexp.MustCompile(`[A-Za-z0-9_]+`)

// BuildFtsQuery converts a raw query into an FTS5 MATCH expression.
// Returns empty string if no tokens can be extracted.
func BuildFtsQuery(raw string) string {
	tokens := tokenRe.FindAllString(raw, -1)
	if len(tokens) == 0 {
		return ""
	}
	var parts []string
	for _, t := range tokens {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		cleaned := strings.ReplaceAll(t, `"`, "")
		parts = append(parts, `"`+cleaned+`"`)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " AND ")
}

// BM25RankToScore converts a BM25 rank value to a 0–1 score.
func BM25RankToScore(rank float64) float64 {
	normalized := rank
	if !math.IsInf(rank, 0) && !math.IsNaN(rank) {
		if rank < 0 {
			normalized = 0
		}
	} else {
		normalized = 999
	}
	return 1.0 / (1.0 + normalized)
}

// MergeHybridResults combines vector and keyword search results using
// weighted scoring, returning results sorted by descending score.
func MergeHybridResults(
	vector []HybridVectorResult,
	keyword []HybridKeywordResult,
	vectorWeight, textWeight float64,
) []HybridMergedResult {
	type entry struct {
		id          string
		path        string
		startLine   int
		endLine     int
		source      string
		snippet     string
		vectorScore float64
		textScore   float64
	}

	byID := make(map[string]*entry)

	for _, r := range vector {
		byID[r.ID] = &entry{
			id: r.ID, path: r.Path,
			startLine: r.StartLine, endLine: r.EndLine,
			source: r.Source, snippet: r.Snippet,
			vectorScore: r.VectorScore,
		}
	}

	for _, r := range keyword {
		if e, ok := byID[r.ID]; ok {
			e.textScore = r.TextScore
			if r.Snippet != "" {
				e.snippet = r.Snippet
			}
		} else {
			byID[r.ID] = &entry{
				id: r.ID, path: r.Path,
				startLine: r.StartLine, endLine: r.EndLine,
				source: r.Source, snippet: r.Snippet,
				textScore: r.TextScore,
			}
		}
	}

	var merged []HybridMergedResult
	for _, e := range byID {
		score := vectorWeight*e.vectorScore + textWeight*e.textScore
		merged = append(merged, HybridMergedResult{
			Path:      e.path,
			StartLine: e.startLine,
			EndLine:   e.endLine,
			Score:     score,
			Snippet:   e.snippet,
			Source:    e.source,
		})
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})
	return merged
}
