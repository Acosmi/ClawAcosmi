package line

// TS 对照: src/line/accounts.ts + src/line/probe.ts + src/line/signature.ts
// 审计补全: 帐号解析、bot 探测、签名验证

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// ---------- Accounts (src/line/accounts.ts) ----------

// ResolveLineAccount 解析 LINE 帐号凭证。
// 优先级: 显式传入 > config > env > file
func ResolveLineAccount(config *LineConfig, accountID string) (*ResolvedLineAccount, error) {
	if config == nil {
		return nil, fmt.Errorf("LINE config is nil")
	}

	// 多帐号: 先查 accounts map
	if accountID != "" && config.Accounts != nil {
		if acct, ok := config.Accounts[accountID]; ok {
			token, tokenSource := resolveToken(acct.ChannelAccessToken, acct.TokenFile, config.ChannelAccessToken)
			secret := firstNonEmpty(acct.ChannelSecret, config.ChannelSecret)
			if token == "" || secret == "" {
				return nil, fmt.Errorf("LINE account %q: missing token or secret", accountID)
			}
			return &ResolvedLineAccount{
				AccountID:          accountID,
				Name:               firstNonEmpty(acct.Name, config.Name),
				Enabled:            acct.Enabled,
				ChannelAccessToken: token,
				ChannelSecret:      secret,
				TokenSource:        tokenSource,
			}, nil
		}
	}

	// 默认帐号: 从顶层 config
	token, tokenSource := resolveToken(config.ChannelAccessToken, config.TokenFile, "")
	if token == "" {
		// fallback: env
		token = os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
		if token != "" {
			tokenSource = TokenSourceEnv
		}
	}
	secret := config.ChannelSecret
	if secret == "" {
		secret = os.Getenv("LINE_CHANNEL_SECRET")
	}
	if token == "" || secret == "" {
		return nil, fmt.Errorf("LINE: missing channel access token or secret")
	}

	id := accountID
	if id == "" {
		id = "default"
	}
	return &ResolvedLineAccount{
		AccountID:          id,
		Name:               config.Name,
		Enabled:            config.Enabled,
		ChannelAccessToken: token,
		ChannelSecret:      secret,
		TokenSource:        tokenSource,
	}, nil
}

func resolveToken(configToken, tokenFile, fallback string) (string, LineTokenSource) {
	if configToken != "" {
		return configToken, TokenSourceConfig
	}
	if tokenFile != "" {
		data, err := os.ReadFile(tokenFile)
		if err == nil {
			return strings.TrimSpace(string(data)), TokenSourceFile
		}
	}
	if fallback != "" {
		return fallback, TokenSourceConfig
	}
	return "", TokenSourceNone
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ---------- Probe (src/line/probe.ts) ----------

// ProbeBot 查询 bot 资料以验证凭证。
func ProbeBot(ctx context.Context, token string) (*LineProbeResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.line.me/v2/bot/info", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return &LineProbeResult{OK: false, Error: err.Error()}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return &LineProbeResult{
			OK:    false,
			Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
		}, nil
	}

	var bot LineBotProfile
	if err := json.NewDecoder(resp.Body).Decode(&bot); err != nil {
		return &LineProbeResult{OK: false, Error: err.Error()}, nil
	}

	return &LineProbeResult{OK: true, Bot: &bot}, nil
}

// ---------- Signature (src/line/signature.ts) ----------

// ValidateLineSignature 验证 LINE webhook 签名。
// TS: validateLineSignature(body: string, signature: string, channelSecret: string): boolean
func ValidateLineSignature(body []byte, signature, channelSecret string) bool {
	mac := hmac.New(sha256.New, []byte(channelSecret))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ComputeSignature 计算签名(用于测试)。
func ComputeSignature(body []byte, channelSecret string) string {
	mac := hmac.New(sha256.New, []byte(channelSecret))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
