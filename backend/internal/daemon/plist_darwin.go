//go:build darwin

package daemon

import (
	"os"
	"regexp"
	"strings"
)

// LaunchAgentPlistParams 是构建 plist 的参数
type LaunchAgentPlistParams struct {
	Label            string
	Comment          string
	ProgramArguments []string
	WorkingDirectory string
	StdoutPath       string
	StderrPath       string
	Environment      map[string]string
}

// plistEscape XML 转义
func plistEscape(value string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(value)
}

// plistUnescape XML 反转义
func plistUnescape(value string) string {
	r := strings.NewReplacer(
		"&apos;", "'",
		"&quot;", `"`,
		"&gt;", ">",
		"&lt;", "<",
		"&amp;", "&",
	)
	return r.Replace(value)
}

// BuildLaunchAgentPlist 构建 LaunchAgent plist XML
// 对应 TS: launchd-plist.ts buildLaunchAgentPlist
func BuildLaunchAgentPlist(params LaunchAgentPlistParams) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString("\n")
	sb.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">`)
	sb.WriteString("\n")
	sb.WriteString(`<plist version="1.0">`)
	sb.WriteString("\n  <dict>\n")

	// Label
	sb.WriteString("    <key>Label</key>\n")
	sb.WriteString("    <string>" + plistEscape(params.Label) + "</string>\n")

	// Comment
	if comment := strings.TrimSpace(params.Comment); comment != "" {
		sb.WriteString("    <key>Comment</key>\n")
		sb.WriteString("    <string>" + plistEscape(comment) + "</string>\n")
	}

	// RunAtLoad + KeepAlive
	sb.WriteString("    <key>RunAtLoad</key>\n")
	sb.WriteString("    <true/>\n")
	sb.WriteString("    <key>KeepAlive</key>\n")
	sb.WriteString("    <true/>\n")

	// ProgramArguments
	sb.WriteString("    <key>ProgramArguments</key>\n")
	sb.WriteString("    <array>\n")
	for _, arg := range params.ProgramArguments {
		sb.WriteString("      <string>" + plistEscape(arg) + "</string>\n")
	}
	sb.WriteString("    </array>\n")

	// WorkingDirectory
	if params.WorkingDirectory != "" {
		sb.WriteString("    <key>WorkingDirectory</key>\n")
		sb.WriteString("    <string>" + plistEscape(params.WorkingDirectory) + "</string>\n")
	}

	// StandardOutPath / StandardErrorPath
	sb.WriteString("    <key>StandardOutPath</key>\n")
	sb.WriteString("    <string>" + plistEscape(params.StdoutPath) + "</string>\n")
	sb.WriteString("    <key>StandardErrorPath</key>\n")
	sb.WriteString("    <string>" + plistEscape(params.StderrPath) + "</string>\n")

	// EnvironmentVariables
	if len(params.Environment) > 0 {
		sb.WriteString("    <key>EnvironmentVariables</key>\n")
		sb.WriteString("    <dict>\n")
		for k, v := range params.Environment {
			if strings.TrimSpace(v) == "" {
				continue
			}
			sb.WriteString("    <key>" + plistEscape(k) + "</key>\n")
			sb.WriteString("    <string>" + plistEscape(strings.TrimSpace(v)) + "</string>\n")
		}
		sb.WriteString("    </dict>\n")
	}

	sb.WriteString("  </dict>\n")
	sb.WriteString("</plist>\n")
	return sb.String()
}

// ReadLaunchAgentProgramArgumentsFromFile 从 plist 文件读取程序参数
// 对应 TS: launchd-plist.ts readLaunchAgentProgramArgumentsFromFile
func ReadLaunchAgentProgramArgumentsFromFile(plistPath string) (*GatewayServiceCommand, error) {
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return nil, nil // 文件不存在时返回 nil
	}
	content := string(data)

	// 解析 ProgramArguments
	programRe := regexp.MustCompile(`(?is)<key>ProgramArguments</key>\s*<array>(.*?)</array>`)
	programMatch := programRe.FindStringSubmatch(content)
	if programMatch == nil {
		return nil, nil
	}

	stringRe := regexp.MustCompile(`(?is)<string>(.*?)</string>`)
	argMatches := stringRe.FindAllStringSubmatch(programMatch[1], -1)
	var args []string
	for _, m := range argMatches {
		arg := strings.TrimSpace(plistUnescape(m[1]))
		if arg != "" {
			args = append(args, arg)
		}
	}

	result := &GatewayServiceCommand{
		ProgramArguments: args,
		SourcePath:       plistPath,
	}

	// 解析 WorkingDirectory
	wdRe := regexp.MustCompile(`(?is)<key>WorkingDirectory</key>\s*<string>(.*?)</string>`)
	wdMatch := wdRe.FindStringSubmatch(content)
	if wdMatch != nil {
		result.WorkingDirectory = strings.TrimSpace(plistUnescape(wdMatch[1]))
	}

	// 解析 EnvironmentVariables
	envRe := regexp.MustCompile(`(?is)<key>EnvironmentVariables</key>\s*<dict>(.*?)</dict>`)
	envMatch := envRe.FindStringSubmatch(content)
	if envMatch != nil {
		kvRe := regexp.MustCompile(`(?is)<key>(.*?)</key>\s*<string>(.*?)</string>`)
		kvMatches := kvRe.FindAllStringSubmatch(envMatch[1], -1)
		if len(kvMatches) > 0 {
			result.Environment = make(map[string]string)
			for _, kv := range kvMatches {
				key := strings.TrimSpace(plistUnescape(kv[1]))
				if key == "" {
					continue
				}
				result.Environment[key] = strings.TrimSpace(plistUnescape(kv[2]))
			}
		}
	}

	return result, nil
}

// TryExtractPlistLabel 从 plist 内容中提取 Label
// 对应 TS: inspect.ts tryExtractPlistLabel
func TryExtractPlistLabel(contents string) string {
	re := regexp.MustCompile(`(?is)<key>Label</key>\s*<string>([\s\S]*?)</string>`)
	match := re.FindStringSubmatch(contents)
	if match == nil {
		return ""
	}
	return strings.TrimSpace(match[1])
}
