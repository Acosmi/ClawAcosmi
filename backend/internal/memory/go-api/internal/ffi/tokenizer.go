//go:build cgo

package ffi

// CGO 绑定：nexus-tokenizer — 中文分词 + BM25 评分。
//
// 替换 Go 侧 vector_store.go 中简陋的 tokenizeBM25 字符级拆分。

/*
#cgo LDFLAGS: -L${SRCDIR}/../../../../libs/nexus-core/target/release -lnexus_unified -lm -ldl -framework Security
#cgo CFLAGS: -I${SRCDIR}/../../../../libs/nexus-core/include
#include <stdlib.h>
#include "nexus_tokenizer.h"
*/
import "C"

import (
	"fmt"
	"strings"
	"unsafe"
)

// Tokenize 使用 jieba-rs 对文本进行中文分词。
func Tokenize(text string) []string {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	var count C.uint
	result := C.nexus_tokenize(cText, &count)
	if result == nil || count == 0 {
		return nil
	}
	defer C.nexus_tokenizer_free(result)

	// 结果以 \0 分隔
	raw := C.GoString(result)
	// GoString 只读到第一个 \0，需用 GoStringN 获取完整数据
	fullLen := 0
	for i := 0; i < int(count); i++ {
		p := unsafe.Pointer(uintptr(unsafe.Pointer(result)) + uintptr(fullLen))
		s := C.GoString((*C.char)(p))
		fullLen += len(s) + 1
	}
	// 按 \0 分隔提取所有 token
	_ = raw
	tokens := make([]string, 0, int(count))
	offset := 0
	for i := 0; i < int(count); i++ {
		p := unsafe.Pointer(uintptr(unsafe.Pointer(result)) + uintptr(offset))
		s := C.GoString((*C.char)(p))
		if s != "" {
			tokens = append(tokens, s)
		}
		offset += len(s) + 1
	}
	return tokens
}

// BM25Score 计算 query 与 doc 之间的 BM25 相关性得分。
func BM25Score(query, doc string, avgDocLen float64) float64 {
	cQuery := C.CString(query)
	cDoc := C.CString(doc)
	defer C.free(unsafe.Pointer(cQuery))
	defer C.free(unsafe.Pointer(cDoc))

	return float64(C.nexus_bm25_score(cQuery, cDoc, C.double(avgDocLen)))
}

// TokenizeJoin 分词后用空格连接，方便调试。
func TokenizeJoin(text string) string {
	return strings.Join(Tokenize(text), " ")
}

// SetIDFTable 设置 BM25 的真实 IDF 表。
// idfMap: 词→IDF 值，docCount: 文档总数 N。
func SetIDFTable(idfMap map[string]float64, docCount float64) error {
	if len(idfMap) == 0 {
		return fmt.Errorf("idfMap is empty")
	}
	// 序列化为 "word\x00idf_value\x00" 格式
	var buf strings.Builder
	for word, idf := range idfMap {
		buf.WriteString(word)
		buf.WriteByte(0)
		buf.WriteString(fmt.Sprintf("%f", idf))
		buf.WriteByte(0)
	}
	data := buf.String()
	cData := C.CString(data)
	defer C.free(unsafe.Pointer(cData))

	ret := C.nexus_set_idf_table(cData, C.uint(len(data)), C.double(docCount))
	if ret != 0 {
		return fmt.Errorf("nexus_set_idf_table failed (code %d)", ret)
	}
	return nil
}
