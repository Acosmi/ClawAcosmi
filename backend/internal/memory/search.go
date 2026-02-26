package memory

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

// SearchRowResult is a single row from a vector or keyword search.
type SearchRowResult struct {
	ID        string
	Path      string
	StartLine int
	EndLine   int
	Score     float64
	Snippet   string
	Source    string
}

// vectorToBlob converts a float64 embedding to a little-endian float32 blob.
func vectorToBlob(embedding []float64) []byte {
	buf := make([]byte, len(embedding)*4)
	for i, v := range embedding {
		bits := math.Float32bits(float32(v))
		binary.LittleEndian.PutUint32(buf[i*4:], bits)
	}
	return buf
}

// truncateUTF8 truncates a string to at most maxChars characters,
// respecting UTF-8 boundaries.
func truncateUTF8(s string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	return string(runes[:maxChars])
}

// SourceFilter holds a SQL WHERE clause fragment and its bound parameters.
type SourceFilter struct {
	SQL    string
	Params []any
}

// SearchVectorParams controls a vector search.
type SearchVectorParams struct {
	DB                 *sql.DB
	VectorTable        string
	ProviderModel      string
	QueryVec           []float64
	Limit              int
	SnippetMaxChars    int
	EnsureVectorReady  func(ctx context.Context, dims int) (bool, error)
	SourceFilterVec    SourceFilter
	SourceFilterChunks SourceFilter
}

// SearchVector executes a vector-similarity search against chunks.
// If the sqlite-vec extension is available, it uses vec_distance_cosine;
// otherwise, it falls back to in-process cosine similarity.
func SearchVector(ctx context.Context, p SearchVectorParams) ([]SearchRowResult, error) {
	if len(p.QueryVec) == 0 || p.Limit <= 0 {
		return nil, nil
	}

	ready, err := p.EnsureVectorReady(ctx, len(p.QueryVec))
	if err != nil {
		return nil, err
	}

	if ready {
		return searchVectorNative(p)
	}
	return searchVectorFallback(p)
}

func searchVectorNative(p SearchVectorParams) ([]SearchRowResult, error) {
	querySQL := fmt.Sprintf(
		`SELECT c.id, c.path, c.start_line, c.end_line, c.text, c.source,
		        vec_distance_cosine(v.embedding, ?) AS dist
		   FROM %s v
		   JOIN chunks c ON c.id = v.id
		  WHERE c.model = ?%s
		  ORDER BY dist ASC
		  LIMIT ?`,
		p.VectorTable, p.SourceFilterVec.SQL,
	)

	args := []any{vectorToBlob(p.QueryVec), p.ProviderModel}
	args = append(args, p.SourceFilterVec.Params...)
	args = append(args, p.Limit)

	rows, err := p.DB.Query(querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	defer rows.Close()

	var results []SearchRowResult
	for rows.Next() {
		var (
			id, path, text, source string
			startLine, endLine     int
			dist                   float64
		)
		if err := rows.Scan(&id, &path, &startLine, &endLine, &text, &source, &dist); err != nil {
			continue
		}
		results = append(results, SearchRowResult{
			ID:        id,
			Path:      path,
			StartLine: startLine,
			EndLine:   endLine,
			Score:     1 - dist,
			Snippet:   truncateUTF8(text, p.SnippetMaxChars),
			Source:    source,
		})
	}
	return results, rows.Err()
}

func searchVectorFallback(p SearchVectorParams) ([]SearchRowResult, error) {
	chunks, err := listChunks(p.DB, p.ProviderModel, p.SourceFilterChunks)
	if err != nil {
		return nil, err
	}

	type scored struct {
		chunk chunkRow
		score float64
	}
	var candidates []scored
	for _, c := range chunks {
		s := CosineSimilarity(p.QueryVec, c.embedding)
		if !math.IsNaN(s) && !math.IsInf(s, 0) {
			candidates = append(candidates, scored{chunk: c, score: s})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	limit := p.Limit
	if limit > len(candidates) {
		limit = len(candidates)
	}

	results := make([]SearchRowResult, 0, limit)
	for _, c := range candidates[:limit] {
		results = append(results, SearchRowResult{
			ID:        c.chunk.id,
			Path:      c.chunk.path,
			StartLine: c.chunk.startLine,
			EndLine:   c.chunk.endLine,
			Score:     c.score,
			Snippet:   truncateUTF8(c.chunk.text, p.SnippetMaxChars),
			Source:    c.chunk.source,
		})
	}
	return results, nil
}

type chunkRow struct {
	id        string
	path      string
	startLine int
	endLine   int
	text      string
	embedding []float64
	source    string
}

func listChunks(db *sql.DB, providerModel string, filter SourceFilter) ([]chunkRow, error) {
	querySQL := fmt.Sprintf(
		`SELECT id, path, start_line, end_line, text, embedding, source
		   FROM chunks
		  WHERE model = ?%s`, filter.SQL,
	)
	args := []any{providerModel}
	args = append(args, filter.Params...)

	rows, err := db.Query(querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("list chunks: %w", err)
	}
	defer rows.Close()

	var result []chunkRow
	for rows.Next() {
		var (
			id, path, text, embeddingRaw, source string
			startLine, endLine                   int
		)
		if err := rows.Scan(&id, &path, &startLine, &endLine, &text, &embeddingRaw, &source); err != nil {
			continue
		}
		result = append(result, chunkRow{
			id:        id,
			path:      path,
			startLine: startLine,
			endLine:   endLine,
			text:      text,
			embedding: ParseEmbedding(embeddingRaw),
			source:    source,
		})
	}
	return result, rows.Err()
}

// SearchKeywordParams controls a FTS5 keyword search.
type SearchKeywordParams struct {
	DB              *sql.DB
	FTSTable        string
	ProviderModel   string
	Query           string
	Limit           int
	SnippetMaxChars int
	SourceFilter    SourceFilter
}

// SearchKeywordResult extends SearchRowResult with a TextScore field.
type SearchKeywordResult struct {
	SearchRowResult
	TextScore float64
}

// SearchKeyword executes a FTS5 keyword search against indexed chunks.
func SearchKeyword(_ context.Context, p SearchKeywordParams) ([]SearchKeywordResult, error) {
	if p.Limit <= 0 {
		return nil, nil
	}
	ftsQuery := BuildFtsQuery(p.Query)
	if ftsQuery == "" {
		return nil, nil
	}

	querySQL := fmt.Sprintf(
		`SELECT id, path, source, start_line, end_line, text,
		        bm25(%s) AS rank
		   FROM %s
		  WHERE %s MATCH ? AND model = ?%s
		  ORDER BY rank ASC
		  LIMIT ?`,
		p.FTSTable, p.FTSTable, p.FTSTable, p.SourceFilter.SQL,
	)

	args := []any{ftsQuery, p.ProviderModel}
	args = append(args, p.SourceFilter.Params...)
	args = append(args, p.Limit)

	rows, err := p.DB.Query(querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("keyword search: %w", err)
	}
	defer rows.Close()

	var results []SearchKeywordResult
	for rows.Next() {
		var (
			id, path, source, text string
			startLine, endLine     int
			rank                   float64
		)
		if err := rows.Scan(&id, &path, &source, &startLine, &endLine, &text, &rank); err != nil {
			continue
		}
		textScore := BM25RankToScore(rank)
		results = append(results, SearchKeywordResult{
			SearchRowResult: SearchRowResult{
				ID:        id,
				Path:      path,
				StartLine: startLine,
				EndLine:   endLine,
				Score:     textScore,
				Snippet:   truncateUTF8(text, p.SnippetMaxChars),
				Source:    source,
			},
			TextScore: textScore,
		})
	}
	return results, rows.Err()
}
