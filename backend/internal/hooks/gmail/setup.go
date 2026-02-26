package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// --- Gmail Setup 工具函数 ---
// 对应 TS: gmail-setup-utils.ts

const maxOutputChars = 800

// TrimOutput 截断输出
func TrimOutput(value string, max int) string {
	if max <= 0 {
		max = maxOutputChars
	}
	if len(value) <= max {
		return value
	}
	return value[:max] + "…(truncated)"
}

// FormatCommandFailure 格式化命令失败信息
func FormatCommandFailure(command string, code int, stdout, stderr string) string {
	parts := []string{fmt.Sprintf("Command failed: %s (exit code %d)", command, code)}
	if stderr != "" {
		parts = append(parts, "stderr: "+TrimOutput(stderr, maxOutputChars))
	}
	if stdout != "" {
		parts = append(parts, "stdout: "+TrimOutput(stdout, maxOutputChars))
	}
	return strings.Join(parts, "\n")
}

// HasBinary 检查二进制是否可用
func HasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// FindExecutablesOnPath 在 PATH 中查找可执行文件
func FindExecutablesOnPath(bins []string) []string {
	found := make([]string, 0)
	for _, bin := range bins {
		if HasBinary(bin) {
			found = append(found, bin)
		}
	}
	return found
}

// EnsureGcloudOnPath 确保 gcloud 在 PATH
func EnsureGcloudOnPath() bool {
	if HasBinary("gcloud") {
		return true
	}
	// 尝试常见位置
	candidates := []string{
		"/usr/local/google-cloud-sdk/bin",
		"/opt/homebrew/share/google-cloud-sdk/bin",
		filepath.Join(os.Getenv("HOME"), "google-cloud-sdk", "bin"),
	}
	for _, dir := range candidates {
		gcloudPath := filepath.Join(dir, "gcloud")
		if _, err := os.Stat(gcloudPath); err == nil {
			path := os.Getenv("PATH")
			os.Setenv("PATH", dir+string(os.PathListSeparator)+path)
			return true
		}
	}
	return false
}

// RunGcloudCommand 执行 gcloud 命令
func RunGcloudCommand(args []string, timeoutMs int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gcloud", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("gcloud %s: %s", strings.Join(args, " "), err.Error())
	}
	return strings.TrimSpace(string(output)), nil
}

// EnsureTopic 确保 Pub/Sub topic 存在
func EnsureTopic(projectID, topicName string) error {
	args := []string{"pubsub", "topics", "describe", topicName, "--project", projectID, "--format=json"}
	_, err := RunGcloudCommand(args, 30_000)
	if err == nil {
		return nil // topic 已存在
	}
	// 创建 topic
	createArgs := []string{"pubsub", "topics", "create", topicName, "--project", projectID}
	_, err = RunGcloudCommand(createArgs, 30_000)
	return err
}

// EnsureSubscription 确保 Pub/Sub subscription 存在
func EnsureSubscription(projectID, subscription, topicName, pushEndpoint string) error {
	args := []string{"pubsub", "subscriptions", "describe", subscription, "--project", projectID, "--format=json"}
	_, err := RunGcloudCommand(args, 30_000)
	if err == nil {
		return nil // subscription 已存在
	}
	// 创建 subscription
	topicPath := BuildTopicPath(projectID, topicName)
	createArgs := []string{
		"pubsub", "subscriptions", "create", subscription,
		"--project", projectID,
		"--topic", topicPath,
	}
	if pushEndpoint != "" {
		createArgs = append(createArgs, "--push-endpoint", pushEndpoint)
	}
	_, err = RunGcloudCommand(createArgs, 30_000)
	return err
}

// EnsureTailscaleEndpoint 确保 Tailscale 端点已配置
// 对应 TS: gmail-setup-utils.ts ensureTailscaleEndpoint
func EnsureTailscaleEndpoint(mode, path string, port int, target, token string) (string, error) {
	if mode == "off" || mode == "" {
		return "", nil
	}

	if !HasBinary("tailscale") {
		return "", fmt.Errorf("tailscale binary not found; install Tailscale first")
	}

	normalizedPath := NormalizeServePath(path)
	portStr := fmt.Sprintf("%d", port)

	var args []string
	if mode == "funnel" {
		args = []string{"funnel"}
	} else {
		args = []string{"serve"}
	}

	if target != "" {
		args = append(args, "--set-path", normalizedPath, target)
	} else {
		args = append(args, "--set-path", normalizedPath, "http://127.0.0.1:"+portStr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tailscale", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tailscale %s failed: %s (%s)", mode, strings.TrimSpace(string(output)), err)
	}

	return strings.TrimSpace(string(output)), nil
}

// ResolveProjectIDFromGogCredentials 从 gog 凭证文件中解析 Project ID
// 对应 TS: gmail-setup-utils.ts resolveProjectIdFromGogCredentials
func ResolveProjectIDFromGogCredentials() string {
	paths := GogCredentialsPaths()
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			continue
		}
		clientID := extractGogClientID(parsed)
		if projectNum := extractProjectNumber(clientID); projectNum != "" {
			return projectNum
		}
	}
	return ""
}

// GogCredentialsPaths 返回 gog 凭证文件候选路径
func GogCredentialsPaths() []string {
	home := os.Getenv("HOME")
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".config", "gog", "credentials.json"),
		filepath.Join(home, ".gog", "credentials.json"),
	}
}

func extractGogClientID(parsed map[string]interface{}) string {
	// 检查 installed.client_id 或 web.client_id
	for _, key := range []string{"installed", "web"} {
		if section, ok := parsed[key].(map[string]interface{}); ok {
			if clientID, ok := section["client_id"].(string); ok {
				return clientID
			}
		}
	}
	return ""
}

func extractProjectNumber(clientID string) string {
	if clientID == "" {
		return ""
	}
	// client_id 通常是 "PROJECT_NUMBER-HASH.apps.googleusercontent.com"
	parts := strings.SplitN(clientID, "-", 2)
	if len(parts) < 2 {
		return ""
	}
	// 验证是数字
	for _, c := range parts[0] {
		if c < '0' || c > '9' {
			return ""
		}
	}
	return parts[0]
}
