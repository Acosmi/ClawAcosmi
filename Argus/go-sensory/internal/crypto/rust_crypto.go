package crypto

/*
#cgo CFLAGS: -I${SRCDIR}/../../../rust-core/include
#cgo LDFLAGS: -L${SRCDIR}/../../../rust-core/target/release -largus_core -Wl,-rpath,/usr/lib/swift
#include "argus_core.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// RustAESEncrypt encrypts plaintext using AES-256-GCM.
// Returns ciphertext and the 12-byte nonce needed for decryption.
func RustAESEncrypt(key, plaintext []byte) (ciphertext, nonce []byte, err error) {
	if len(key) != 32 {
		return nil, nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	if len(plaintext) == 0 {
		return nil, nil, fmt.Errorf("plaintext cannot be empty")
	}

	var noncePtr *C.uint8_t
	var nonceLen C.size_t
	var cipherPtr *C.uint8_t
	var cipherLen C.size_t

	rc := C.argus_aes_encrypt(
		(*C.uint8_t)(unsafe.Pointer(&key[0])),
		(*C.uint8_t)(unsafe.Pointer(&plaintext[0])),
		C.size_t(len(plaintext)),
		&noncePtr, &nonceLen,
		&cipherPtr, &cipherLen,
	)
	if rc != 0 {
		return nil, nil, fmt.Errorf("argus_aes_encrypt failed: rc=%d", rc)
	}

	nonce = C.GoBytes(unsafe.Pointer(noncePtr), C.int(nonceLen))
	ciphertext = C.GoBytes(unsafe.Pointer(cipherPtr), C.int(cipherLen))

	C.argus_free_buffer(noncePtr, nonceLen)
	C.argus_free_buffer(cipherPtr, cipherLen)

	return ciphertext, nonce, nil
}

// RustAESDecrypt decrypts ciphertext using AES-256-GCM.
func RustAESDecrypt(key, nonce, ciphertext []byte) (plaintext []byte, err error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	if len(nonce) != 12 {
		return nil, fmt.Errorf("nonce must be 12 bytes, got %d", len(nonce))
	}
	if len(ciphertext) == 0 {
		return nil, fmt.Errorf("ciphertext cannot be empty")
	}

	var plainPtr *C.uint8_t
	var plainLen C.size_t

	rc := C.argus_aes_decrypt(
		(*C.uint8_t)(unsafe.Pointer(&key[0])),
		(*C.uint8_t)(unsafe.Pointer(&nonce[0])),
		(*C.uint8_t)(unsafe.Pointer(&ciphertext[0])),
		C.size_t(len(ciphertext)),
		&plainPtr, &plainLen,
	)
	if rc != 0 {
		return nil, fmt.Errorf("argus_aes_decrypt failed: rc=%d", rc)
	}

	plaintext = C.GoBytes(unsafe.Pointer(plainPtr), C.int(plainLen))
	C.argus_free_buffer(plainPtr, plainLen)

	return plaintext, nil
}
