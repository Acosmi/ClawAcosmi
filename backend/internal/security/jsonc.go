// jsonc.go — JSONC (JSON with Comments) 解析支持。
//
// OpenAcosmi 配置文件允许 // 和 /* */ 注释以及尾逗号（JSONC/JSON5 格式），
// 但 encoding/json 不支持。本文件提供 ParseJSONC 函数，先使用 hujson
// 清洗注释和尾逗号，再交给标准库 JSON 解析。
//
// 用于安全审计（audit_extra.go）和自动修复（fix.go）中的配置文件解析。
package security

import (
	"encoding/json"
	"fmt"

	"github.com/tailscale/hujson"
)

// ParseJSONC 解析 JSONC 格式数据（含 // 注释、/* */ 块注释、尾逗号）。
// 先通过 hujson.Standardize 清洗为标准 JSON，再用 encoding/json 反序列化。
// 与 config/loader.go 中的 parseJSON5 功能等价，但为 security 包提供独立实现，
// 避免跨包依赖。
func ParseJSONC(data []byte, v interface{}) error {
	standardized, err := hujson.Standardize(data)
	if err != nil {
		return fmt.Errorf("JSONC syntax error: %w", err)
	}
	return json.Unmarshal(standardized, v)
}
