// Package sessions — 会话路径解析。
//
// 对齐 TS: src/config/sessions/paths.ts (91L)
package sessions

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/config"
	"github.com/Acosmi/ClawAcosmi/internal/routing"
)

// ---------- 会话目录 ----------

// ResolveAgentSessionsDir 解析 agent 的会话目录路径。
// 格式: <stateDir>/agents/<normalizedAgentId>/sessions
func ResolveAgentSessionsDir(agentID string) string {
	root := config.ResolveStateDir()
	id := routing.NormalizeAgentID(agentID)
	return filepath.Join(root, "agents", id, "sessions")
}

// ResolveSessionTranscriptsDir 解析默认 agent 的转录目录。
func ResolveSessionTranscriptsDir() string {
	return ResolveAgentSessionsDir(routing.DefaultAgentID)
}

// ResolveSessionTranscriptsDirForAgent 解析指定 agent 的转录目录。
func ResolveSessionTranscriptsDirForAgent(agentID string) string {
	if agentID == "" {
		agentID = routing.DefaultAgentID
	}
	return ResolveAgentSessionsDir(agentID)
}

// ---------- 存储路径 ----------

// ResolveDefaultSessionStorePath 解析默认的 sessions.json 路径。
func ResolveDefaultSessionStorePath(agentID string) string {
	return filepath.Join(ResolveAgentSessionsDir(agentID), "sessions.json")
}

// ResolveSessionTranscriptPath 解析会话转录 JSONL 文件路径。
// 对齐 TS: paths.ts resolveSessionTranscriptPath()
func ResolveSessionTranscriptPath(sessionID, agentID string, topicID interface{}) string {
	var fileName string
	switch v := topicID.(type) {
	case string:
		if v != "" {
			safe := url.PathEscape(v)
			fileName = fmt.Sprintf("%s-topic-%s.jsonl", sessionID, safe)
		}
	case int:
		fileName = fmt.Sprintf("%s-topic-%d.jsonl", sessionID, v)
	case int64:
		fileName = fmt.Sprintf("%s-topic-%d.jsonl", sessionID, v)
	}
	if fileName == "" {
		fileName = sessionID + ".jsonl"
	}
	if agentID == "" {
		agentID = routing.DefaultAgentID
	}
	return filepath.Join(ResolveAgentSessionsDir(agentID), fileName)
}

// ResolveSessionFilePath 解析实际的会话文件路径（优先使用 entry.SessionFile）。
// 对齐 TS: paths.ts resolveSessionFilePath()
func ResolveSessionFilePath(sessionID string, entry *FullSessionEntry, agentID string) string {
	if entry != nil {
		candidate := strings.TrimSpace(entry.SessionFile)
		if candidate != "" {
			return candidate
		}
	}
	return ResolveSessionTranscriptPath(sessionID, agentID, nil)
}

// ResolveStorePath 解析 session store 的路径。
// 支持模板参数 {agentId} 替换和 ~ 前缀展开。
// 对齐 TS: paths.ts resolveStorePath()
func ResolveStorePath(store string, agentID string) string {
	id := routing.NormalizeAgentID(agentID)
	if store == "" {
		return ResolveDefaultSessionStorePath(id)
	}

	// 模板替换
	if strings.Contains(store, "{agentId}") {
		expanded := strings.ReplaceAll(store, "{agentId}", id)
		return resolveWithHomeExpansion(expanded)
	}

	return resolveWithHomeExpansion(store)
}

// resolveWithHomeExpansion 展开 ~ 前缀为用户主目录。
func resolveWithHomeExpansion(p string) string {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
