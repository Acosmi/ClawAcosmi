package gateway

// TS 对照: src/gateway/server/tls.ts + src/infra/tls/gateway.ts
// Gateway TLS 运行时配置 — 自签名证书加载/生成、指纹提取、TLS 选项构建。

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Acosmi/ClawAcosmi/internal/infra"
)

// GatewayTlsRuntime TLS 运行时配置。
// TS 对照: infra/tls/gateway.ts GatewayTlsRuntime (L13-22)
type GatewayTlsRuntime struct {
	Enabled           bool
	Required          bool
	CertPath          string
	KeyPath           string
	CAPath            string
	FingerprintSHA256 string
	TLSConfig         *tls.Config
	Error             string
}

// GatewayTlsConfig TLS 用户配置（从 config 文件读取）。
// TS 对照: config/types.gateway.ts GatewayTlsConfig
type GatewayTlsConfig struct {
	Enabled      bool   `json:"enabled"`
	AutoGenerate *bool  `json:"autoGenerate,omitempty"` // 默认 true
	CertPath     string `json:"certPath,omitempty"`
	KeyPath      string `json:"keyPath,omitempty"`
	CAPath       string `json:"caPath,omitempty"`
}

// LoadGatewayTlsRuntime 加载 TLS 运行时配置。
// 根据用户配置加载或生成自签名证书，提取 SHA-256 指纹，构建 tls.Config。
// TS 对照: infra/tls/gateway.ts loadGatewayTlsRuntime (L67-150)
func LoadGatewayTlsRuntime(cfg *GatewayTlsConfig, logger *slog.Logger) GatewayTlsRuntime {
	if cfg == nil || !cfg.Enabled {
		return GatewayTlsRuntime{Enabled: false, Required: false}
	}

	autoGenerate := cfg.AutoGenerate == nil || *cfg.AutoGenerate

	// 默认证书路径
	homeDir, _ := os.UserHomeDir()
	baseDir := filepath.Join(homeDir, ".config", "openacosmi", "gateway", "tls")
	certPath := cfg.CertPath
	keyPath := cfg.KeyPath
	if certPath == "" {
		certPath = filepath.Join(baseDir, "gateway-cert.pem")
	}
	if keyPath == "" {
		keyPath = filepath.Join(baseDir, "gateway-key.pem")
	}

	// 检测证书是否已存在
	hasCert := fileExists(certPath)
	hasKey := fileExists(keyPath)

	if !hasCert && !hasKey && autoGenerate {
		// 确保目录存在
		if err := os.MkdirAll(filepath.Dir(certPath), 0o700); err != nil {
			return GatewayTlsRuntime{
				Enabled:  false,
				Required: true,
				CertPath: certPath,
				KeyPath:  keyPath,
				Error:    fmt.Sprintf("gateway tls: failed to create cert dir: %v", err),
			}
		}
		if dir := filepath.Dir(keyPath); dir != filepath.Dir(certPath) {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return GatewayTlsRuntime{
					Enabled:  false,
					Required: true,
					CertPath: certPath,
					KeyPath:  keyPath,
					Error:    fmt.Sprintf("gateway tls: failed to create key dir: %v", err),
				}
			}
		}

		cert, err := infra.GenerateGatewayCert()
		if err != nil {
			return GatewayTlsRuntime{
				Enabled:  false,
				Required: true,
				CertPath: certPath,
				KeyPath:  keyPath,
				Error:    fmt.Sprintf("gateway tls: failed to generate cert: %v", err),
			}
		}
		if err := infra.SaveGatewayCert(cert, certPath, keyPath); err != nil {
			return GatewayTlsRuntime{
				Enabled:  false,
				Required: true,
				CertPath: certPath,
				KeyPath:  keyPath,
				Error:    fmt.Sprintf("gateway tls: failed to save cert: %v", err),
			}
		}
		if logger != nil {
			logger.Info("gateway tls: generated self-signed cert", "certPath", certPath)
		}
	}

	// 验证证书文件存在
	if !fileExists(certPath) || !fileExists(keyPath) {
		return GatewayTlsRuntime{
			Enabled:  false,
			Required: true,
			CertPath: certPath,
			KeyPath:  keyPath,
			Error:    "gateway tls: cert/key missing",
		}
	}

	// 加载证书
	tlsCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return GatewayTlsRuntime{
			Enabled:  false,
			Required: true,
			CertPath: certPath,
			KeyPath:  keyPath,
			Error:    fmt.Sprintf("gateway tls: failed to load cert: %v", err),
		}
	}

	// 提取指纹
	var fingerprint string
	if len(tlsCert.Certificate) > 0 {
		x509Cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
		if err == nil {
			fingerprint = infra.GetCertFingerprint(x509Cert)
		}
	}

	if fingerprint == "" {
		return GatewayTlsRuntime{
			Enabled:  false,
			Required: true,
			CertPath: certPath,
			KeyPath:  keyPath,
			Error:    "gateway tls: unable to compute certificate fingerprint",
		}
	}

	// 加载 CA（如果配置了）
	var caPath string
	if cfg.CAPath != "" {
		caPath = cfg.CAPath
	}

	// 构建 tls.Config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS13,
	}
	if caPath != "" {
		caPEM, err := os.ReadFile(caPath)
		if err == nil {
			pool := x509.NewCertPool()
			pool.AppendCertsFromPEM(caPEM)
			tlsConfig.ClientCAs = pool
		}
	}

	return GatewayTlsRuntime{
		Enabled:           true,
		Required:          true,
		CertPath:          certPath,
		KeyPath:           keyPath,
		CAPath:            caPath,
		FingerprintSHA256: fingerprint,
		TLSConfig:         tlsConfig,
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
