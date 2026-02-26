//go:build !cgo

package ffi

import (
	"strings"
	"unicode"
)

// Tokenize 纯 Go 分词 fallback（按 Unicode 边界拆分）。
func Tokenize(text string) []string {
	if text == "" {
		return nil
	}
	var tokens []string
	var cur strings.Builder
	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		} else {
			// CJK 字符逐字拆分
			if unicode.Is(unicode.Han, r) {
				if cur.Len() > 0 {
					tokens = append(tokens, cur.String())
					cur.Reset()
				}
				tokens = append(tokens, string(r))
			} else {
				cur.WriteRune(r)
			}
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// BM25Score 纯 Go BM25 评分 fallback。
func BM25Score(query, doc string, avgDocLen float64) float64 {
	const k1 = 1.2
	const b = 0.75

	qTokens := Tokenize(query)
	dTokens := Tokenize(doc)
	dl := float64(len(dTokens))
	if dl == 0 || avgDocLen <= 0 {
		return 0
	}
	tf := make(map[string]float64)
	for _, t := range dTokens {
		tf[t]++
	}
	score := 0.0
	for _, qt := range qTokens {
		freq := tf[qt]
		if freq > 0 {
			num := freq * (k1 + 1)
			den := freq + k1*(1-b+b*dl/avgDocLen)
			score += num / den
		}
	}
	return score
}

// TokenizeJoin 分词后用空格连接。
func TokenizeJoin(text string) string {
	return strings.Join(Tokenize(text), " ")
}

// SetIDFTable 纯 Go 版（no-op，IDF 仅在 Rust 实现中生效）。
func SetIDFTable(_ map[string]float64, _ float64) error {
	return nil // PureGo 模式下 IDF 不生效
}
