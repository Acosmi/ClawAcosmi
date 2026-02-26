package config

// Talk API Key 解析 — 对应 src/config/talk.ts (50 行)
//
// 从环境变量或 shell 配置文件 (.profile, .zprofile, .zshrc, .bashrc)
// 中读取 ELEVENLABS_API_KEY。
//
// 依赖: os (环境变量 + 文件读取)

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// talkApiKeyPattern 匹配 shell 配置文件中的 ELEVENLABS_API_KEY 赋值。
// 对应 TS: /(?:^|\n)\s*(?:export\s+)?ELEVENLABS_API_KEY\s*=\s*["']?([^\n"']+)["']?/
var talkApiKeyPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?ELEVENLABS_API_KEY\s*=\s*["']?([^\n"']+)["']?`)

// profileCandidates shell 配置文件候选列表
var profileCandidates = []string{".profile", ".zprofile", ".zshrc", ".bashrc"}

// ReadTalkApiKeyFromProfile 从用户 home 目录下的 shell 配置文件中读取 ELEVENLABS_API_KEY。
// 对应 TS: readTalkApiKeyFromProfile(deps)
func ReadTalkApiKeyFromProfile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	for _, name := range profileCandidates {
		candidate := filepath.Join(home, name)
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		matches := talkApiKeyPattern.FindSubmatch(data)
		if len(matches) >= 2 {
			value := strings.TrimSpace(string(matches[1]))
			if value != "" {
				return value
			}
		}
	}
	return ""
}

// ResolveTalkApiKey 解析 Talk API Key，优先环境变量，回退到 shell 配置文件。
// 对应 TS: resolveTalkApiKey(env, deps)
func ResolveTalkApiKey() string {
	envValue := strings.TrimSpace(os.Getenv("ELEVENLABS_API_KEY"))
	if envValue != "" {
		return envValue
	}
	return ReadTalkApiKeyFromProfile()
}
