// onboard/oauth_tls_preflight.go — TLS 证书预检
// 对应 TS 文件: src/commands/oauth-tls-preflight.ts
// OpenAI OAuth TLS 证书问题检测与修复建议格式化。
package onboard

import (
	"crypto/tls"
	"fmt"
	"net"
	"runtime"
	"strings"
	"time"
)

// TLSErrorCode TLS 错误代码。
type TLSErrorCode string

const (
	TLSErrorCodeSelfSignedCert TLSErrorCode = "DEPTH_ZERO_SELF_SIGNED_CERT"
	TLSErrorCodeCertExpired    TLSErrorCode = "CERT_HAS_EXPIRED"
	TLSErrorCodeUnableToVerify TLSErrorCode = "UNABLE_TO_VERIFY_LEAF_SIGNATURE"
	TLSErrorCodeNotYetValid    TLSErrorCode = "CERT_NOT_YET_VALID"
)

// TLSPreflightResult TLS 预检结果。
type TLSPreflightResult struct {
	OK      bool
	Code    TLSErrorCode
	Message string
}

// oauthPreflightHost OpenAI OAuth 预检主机。
const oauthPreflightHost = "auth0.openai.com"

// tlsErrorPatterns TLS 错误模式与代码映射。
var tlsErrorPatterns = map[string]TLSErrorCode{
	"certificate is not trusted":    TLSErrorCodeSelfSignedCert,
	"x509: certificate has expired": TLSErrorCodeCertExpired,
	"unknown authority":             TLSErrorCodeUnableToVerify,
	"certificate is not valid":      TLSErrorCodeNotYetValid,
}

// RunOpenAIOAuthTlsPreflight 运行 OpenAI OAuth TLS 预检。
// 对应 TS: runOpenaiOAuthTlsPreflight()
func RunOpenAIOAuthTlsPreflight() TLSPreflightResult {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", oauthPreflightHost+":443", &tls.Config{
		ServerName: oauthPreflightHost,
	})
	if err != nil {
		errMsg := err.Error()
		for pattern, code := range tlsErrorPatterns {
			if containsIgnoreCase(errMsg, pattern) {
				return TLSPreflightResult{
					OK:      false,
					Code:    code,
					Message: errMsg,
				}
			}
		}
		return TLSPreflightResult{
			OK:      false,
			Code:    TLSErrorCodeUnableToVerify,
			Message: errMsg,
		}
	}
	defer conn.Close()
	return TLSPreflightResult{OK: true}
}

// containsIgnoreCase 不区分大小写的字符串包含检查。
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// FormatOpenAIOAuthTlsPreflightFix 格式化 TLS 预检修复建议消息。
// 对应 TS: formatOpenaiOAuthTlsPreflightFix()
func FormatOpenAIOAuthTlsPreflightFix(result TLSPreflightResult) string {
	if result.OK {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("⚠ OpenAI OAuth TLS Certificate Error\n\n")
	sb.WriteString(fmt.Sprintf("Code: %s\n", result.Code))
	sb.WriteString(fmt.Sprintf("Error: %s\n\n", result.Message))

	switch result.Code {
	case TLSErrorCodeSelfSignedCert, TLSErrorCodeUnableToVerify:
		sb.WriteString("Your system's CA certificate store may be incomplete or overridden.\n\n")

		if runtime.GOOS == "darwin" {
			sb.WriteString("If you installed Go/Node via Homebrew, try:\n")
			sb.WriteString("  export SSL_CERT_FILE=$(brew --prefix)/etc/openssl@3/cert.pem\n\n")
			sb.WriteString("Or install the Homebrew CA certs:\n")
			sb.WriteString("  brew install ca-certificates\n")
		} else if runtime.GOOS == "linux" {
			sb.WriteString("Try updating your CA certificates:\n")
			sb.WriteString("  sudo update-ca-certificates\n\n")
			sb.WriteString("Or manually set the cert file:\n")
			sb.WriteString("  export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt\n")
		} else {
			sb.WriteString("Ensure your system's CA certificate store is up to date.\n")
		}

	case TLSErrorCodeCertExpired:
		sb.WriteString("The server certificate has expired.\n")
		sb.WriteString("This is usually a server-side issue. Try again later.\n")

	case TLSErrorCodeNotYetValid:
		sb.WriteString("The server certificate is not yet valid.\n")
		sb.WriteString("Check your system clock is set correctly.\n")
	}

	return sb.String()
}
