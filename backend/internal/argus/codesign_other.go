//go:build !darwin

package argus

// codesign_other.go — 非 macOS 平台的 no-op 存根
//
// 仅 macOS 有 TCC 权限系统和 codesign 需求，其他平台无需签名。

// findAppBundleBinary 非 macOS 平台不存在 .app bundle。
func FindAppBundleBinary() string {
	return ""
}

// ensureCodeSigned 非 macOS 平台无需签名。
func EnsureCodeSigned(binaryPath string) error {
	return nil
}
