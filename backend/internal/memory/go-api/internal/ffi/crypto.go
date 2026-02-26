//go:build cgo

package ffi

// CGO 绑定：nexus-crypto — AES-256-GCM 加解密 + bcrypt 哈希验证。
//
// 新增加密能力，不替换现有 Fernet 逻辑。

/*
#cgo LDFLAGS: -L${SRCDIR}/../../../../libs/nexus-core/target/release -lnexus_unified -lm -framework Security
#cgo CFLAGS: -I${SRCDIR}/../../../../libs/nexus-core/include
#include <stdlib.h>
#include "nexus_crypto.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// AES256GCMEncrypt 使用 AES-256-GCM 加密。key 必须为 32 字节。
// 返回 nonce(12B) + ciphertext + tag(16B)。
func AES256GCMEncrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	// 输出缓冲区：12(nonce) + len(pt) + 16(tag)
	outCap := 12 + len(plaintext) + 16
	out := make([]byte, outCap)
	var outLen C.size_t

	ret := C.nexus_aes256gcm_encrypt(
		(*C.uchar)(unsafe.Pointer(&key[0])),
		(*C.uchar)(unsafe.Pointer(&plaintext[0])),
		C.size_t(len(plaintext)),
		(*C.uchar)(unsafe.Pointer(&out[0])),
		C.size_t(outCap),
		&outLen,
	)
	if ret != 0 {
		return nil, fmt.Errorf("AES-256-GCM encrypt failed (code %d)", ret)
	}
	return out[:outLen], nil
}

// AES256GCMDecrypt 使用 AES-256-GCM 解密。key 必须为 32 字节。
// data 格式：nonce(12B) + ciphertext + tag(16B)。
func AES256GCMDecrypt(key, data []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	if len(data) < 28 {
		return nil, fmt.Errorf("data too short for AES-256-GCM")
	}
	outCap := len(data)
	out := make([]byte, outCap)
	var outLen C.size_t

	ret := C.nexus_aes256gcm_decrypt(
		(*C.uchar)(unsafe.Pointer(&key[0])),
		(*C.uchar)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
		(*C.uchar)(unsafe.Pointer(&out[0])),
		C.size_t(outCap),
		&outLen,
	)
	if ret != 0 {
		return nil, fmt.Errorf("AES-256-GCM decrypt failed")
	}
	return out[:outLen], nil
}

// BcryptHash 使用 bcrypt 哈希密码。cost 范围 [4, 31]。
func BcryptHash(password string, cost int) (string, error) {
	cPw := C.CString(password)
	defer C.free(unsafe.Pointer(cPw))

	result := C.nexus_bcrypt_hash(cPw, C.int(cost))
	if result == nil {
		return "", fmt.Errorf("bcrypt hash failed")
	}
	defer C.nexus_crypto_free(result)
	return C.GoString(result), nil
}

// BcryptVerify 验证密码与 bcrypt 哈希是否匹配。
func BcryptVerify(password, hash string) (bool, error) {
	cPw := C.CString(password)
	cHash := C.CString(hash)
	defer C.free(unsafe.Pointer(cPw))
	defer C.free(unsafe.Pointer(cHash))

	ret := C.nexus_bcrypt_verify(cPw, cHash)
	switch ret {
	case 1:
		return true, nil
	case 0:
		return false, nil
	default:
		return false, fmt.Errorf("bcrypt verify error")
	}
}
