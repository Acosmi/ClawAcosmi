//go:build cgo

package uhms

// CGO 绑定：openviking-ffi tokenizer API — 精确 BPE token 计数。
// 使用 tiktoken-rs cl100k_base 编码 (兼容 GPT-4/Claude)。

/*
#cgo LDFLAGS: -L${SRCDIR}/../../../../cli-rust/libs/openviking-rs/target/release -lopenviking_ffi -lm -ldl
#cgo darwin LDFLAGS: -framework Security -framework CoreFoundation

extern int ovk_token_count(const unsigned char* text, unsigned long text_len);
extern int ovk_token_truncate(const unsigned char* text, unsigned long text_len, unsigned long max_tokens, unsigned long* out_byte_len);
*/
import "C"

import "unsafe"

// countTokensBPE returns exact BPE token count via Rust FFI (cl100k_base).
// Falls back to rune-based estimation if FFI returns error.
func countTokensBPE(text string) int {
	if len(text) == 0 {
		return 0
	}
	b := []byte(text)
	n := C.ovk_token_count((*C.uchar)(unsafe.Pointer(&b[0])), C.ulong(len(b)))
	if n < 0 {
		// FFI error (e.g. invalid UTF-8): fall back to rune estimate
		return countTokensRune(text)
	}
	return int(n)
}

// truncateToTokensBPE truncates text to fit within maxTokens via Rust FFI.
func truncateToTokensBPE(text string, maxTokens int) string {
	if len(text) == 0 || maxTokens <= 0 {
		return ""
	}
	b := []byte(text)
	var outLen C.ulong
	rc := C.ovk_token_truncate(
		(*C.uchar)(unsafe.Pointer(&b[0])),
		C.ulong(len(b)),
		C.ulong(maxTokens),
		&outLen,
	)
	if rc != 0 {
		return truncateToTokensRune(text, maxTokens)
	}
	n := int(outLen)
	if n > len(b) {
		n = len(b) // defensive: Rust should never return > text_len
	}
	return string(b[:n])
}
