// skill_scanner.go — 技能文件安全扫描器。
//
// TS 对照: security/skill-scanner.ts (442L)
//
// 扫描 JavaScript/TypeScript 技能文件中的安全风险模式:
//   - 危险的命令执行 (child_process)
//   - 可疑网络活动 (非标准端口 WebSocket)
//   - 潜在数据泄露 (读文件 + 发网络)
//
// 纯文件系统 + 正则操作，无外部依赖。
package security

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SkillScanSeverity 扫描发现的严重程度。
type SkillScanSeverity string

const (
	SeverityCriticalScan SkillScanSeverity = "critical"
	SeverityWarnScan     SkillScanSeverity = "warn"
	SeverityInfoScan     SkillScanSeverity = "info"
)

// SkillScanFinding 单个扫描发现。
// TS 对照: skill-scanner.ts SkillScanFinding
type SkillScanFinding struct {
	RuleID   string            `json:"ruleId"`
	Severity SkillScanSeverity `json:"severity"`
	File     string            `json:"file"`
	Line     int               `json:"line"`
	Message  string            `json:"message"`
	Evidence string            `json:"evidence"`
}

// SkillScanSummary 扫描汇总。
// TS 对照: skill-scanner.ts SkillScanSummary
type SkillScanSummary struct {
	ScannedFiles int                `json:"scannedFiles"`
	Critical     int                `json:"critical"`
	Warn         int                `json:"warn"`
	Info         int                `json:"info"`
	Findings     []SkillScanFinding `json:"findings"`
}

// SkillScanOptions 扫描选项。
// TS 对照: skill-scanner.ts SkillScanOptions
type SkillScanOptions struct {
	IncludeFiles []string
	MaxFiles     int
	MaxFileBytes int64
}

// 可扫描的文件扩展名。
var scannableExtensions = map[string]bool{
	".js":  true,
	".ts":  true,
	".mjs": true,
	".cjs": true,
	".mts": true,
	".cts": true,
	".jsx": true,
	".tsx": true,
}

const (
	defaultMaxScanFiles = 500
	defaultMaxFileBytes = 1024 * 1024
)

// ---------- 规则定义 ----------

type lineRule struct {
	ruleID          string
	severity        SkillScanSeverity
	message         string
	pattern         *regexp.Regexp
	requiresContext *regexp.Regexp
}

type sourceRule struct {
	ruleID          string
	severity        SkillScanSeverity
	message         string
	pattern         *regexp.Regexp
	requiresContext *regexp.Regexp
}

var lineRules = []lineRule{
	{
		ruleID:   "dangerous-exec",
		severity: SeverityCriticalScan,
		message:  "Shell command execution detected (child_process)",
		pattern:  regexp.MustCompile(`\b(exec|execSync|spawn|spawnSync|execFile|execFileSync)\s*\(`),
	},
	{
		ruleID:   "dangerous-eval",
		severity: SeverityCriticalScan,
		message:  "Dynamic code evaluation detected",
		pattern:  regexp.MustCompile(`\beval\s*\(`),
	},
	{
		ruleID:          "dangerous-function-constructor",
		severity:        SeverityCriticalScan,
		message:         "Function constructor (dynamic code execution)",
		pattern:         regexp.MustCompile(`new\s+Function\s*\(`),
		requiresContext: nil,
	},
	{
		ruleID:   "suspicious-network",
		severity: SeverityWarnScan,
		message:  "WebSocket connection to non-standard port",
		pattern:  regexp.MustCompile(`new\s+WebSocket\s*\(\s*["']wss?://[^"']*:(\d+)`),
	},
}

var standardPorts = map[string]bool{
	"80": true, "443": true, "8080": true, "8443": true, "3000": true,
}

var sourceRules = []sourceRule{
	{
		ruleID:          "potential-exfiltration",
		severity:        SeverityWarnScan,
		message:         "File read combined with network send — possible data exfiltration",
		pattern:         regexp.MustCompile(`readFileSync|readFile`),
		requiresContext: regexp.MustCompile(`(?i)\bfetch\b|\bpost\b|http\.request`),
	},
	{
		ruleID:          "env-exfiltration",
		severity:        SeverityWarnScan,
		message:         "Environment variable access combined with network — possible credential leak",
		pattern:         regexp.MustCompile(`process\.env`),
		requiresContext: regexp.MustCompile(`(?i)\bfetch\b|\bpost\b|http\.request`),
	},
}

// ---------- 核心扫描器 ----------

const maxEvidenceLen = 120

// truncateEvidence 截断证据文本。
func truncateEvidence(evidence string) string {
	if len(evidence) <= maxEvidenceLen {
		return evidence
	}
	return evidence[:maxEvidenceLen-3] + "..."
}

// ScanSource 扫描源代码中的安全问题。
// TS 对照: skill-scanner.ts scanSource()
func ScanSource(source, filePath string) []SkillScanFinding {
	var findings []SkillScanFinding
	lines := strings.Split(source, "\n")

	for lineIdx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// 跳过注释行
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}

		for _, rule := range lineRules {
			if !rule.pattern.MatchString(trimmed) {
				continue
			}

			// WebSocket 端口检查
			if rule.ruleID == "suspicious-network" {
				m := rule.pattern.FindStringSubmatch(trimmed)
				if len(m) > 1 && standardPorts[m[1]] {
					continue
				}
			}

			if rule.requiresContext != nil && !rule.requiresContext.MatchString(source) {
				continue
			}

			findings = append(findings, SkillScanFinding{
				RuleID:   rule.ruleID,
				Severity: rule.severity,
				File:     filePath,
				Line:     lineIdx + 1,
				Message:  rule.message,
				Evidence: truncateEvidence(trimmed),
			})
		}
	}

	// 源码级规则（跨行检查）
	for _, rule := range sourceRules {
		if !rule.pattern.MatchString(source) {
			continue
		}
		if rule.requiresContext != nil && !rule.requiresContext.MatchString(source) {
			continue
		}
		// 查找第一行匹配位置
		lineNo := 1
		for i, line := range lines {
			if rule.pattern.MatchString(line) {
				lineNo = i + 1
				break
			}
		}
		findings = append(findings, SkillScanFinding{
			RuleID:   rule.ruleID,
			Severity: rule.severity,
			File:     filePath,
			Line:     lineNo,
			Message:  rule.message,
			Evidence: truncateEvidence(rule.pattern.FindString(source)),
		})
	}

	return findings
}

// ---------- 目录扫描 ----------

func isScannable(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return scannableExtensions[ext]
}

func normalizeScanOptions(opts *SkillScanOptions) SkillScanOptions {
	result := SkillScanOptions{
		MaxFiles:     defaultMaxScanFiles,
		MaxFileBytes: defaultMaxFileBytes,
	}
	if opts != nil {
		if opts.MaxFiles > 0 {
			result.MaxFiles = opts.MaxFiles
		}
		if opts.MaxFileBytes > 0 {
			result.MaxFileBytes = opts.MaxFileBytes
		}
		result.IncludeFiles = opts.IncludeFiles
	}
	return result
}

// walkDirWithLimit 遍历目录并收集文件路径（限制数量）。
func walkDirWithLimit(dirPath string, maxFiles int) ([]string, error) {
	var files []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无权限目录
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "node_modules" || base == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if len(files) >= maxFiles {
			return filepath.SkipAll
		}
		if isScannable(path) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// readScannableSource 读取可扫描的源文件。
func readScannableSource(filePath string, maxBytes int64) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}
	if info.Size() > maxBytes {
		return "", nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ScanDirectory 扫描目录中的技能文件。
// TS 对照: skill-scanner.ts scanDirectory()
func ScanDirectory(dirPath string, opts *SkillScanOptions) ([]SkillScanFinding, error) {
	normalized := normalizeScanOptions(opts)
	files, err := walkDirWithLimit(dirPath, normalized.MaxFiles)
	if err != nil {
		return nil, err
	}

	var findings []SkillScanFinding
	for _, file := range files {
		source, err := readScannableSource(file, normalized.MaxFileBytes)
		if err != nil || source == "" {
			continue
		}
		// 使用相对路径
		relPath, err := filepath.Rel(dirPath, file)
		if err != nil {
			relPath = file
		}
		findings = append(findings, ScanSource(source, relPath)...)
	}

	return findings, nil
}

// ScanDirectoryWithSummary 扫描目录并返回汇总。
// TS 对照: skill-scanner.ts scanDirectoryWithSummary()
func ScanDirectoryWithSummary(dirPath string, opts *SkillScanOptions) (*SkillScanSummary, error) {
	normalized := normalizeScanOptions(opts)
	files, err := walkDirWithLimit(dirPath, normalized.MaxFiles)
	if err != nil {
		return nil, err
	}

	var findings []SkillScanFinding
	scannedFiles := 0

	for _, file := range files {
		source, err := readScannableSource(file, normalized.MaxFileBytes)
		if err != nil || source == "" {
			continue
		}
		scannedFiles++
		relPath, err := filepath.Rel(dirPath, file)
		if err != nil {
			relPath = file
		}
		findings = append(findings, ScanSource(source, relPath)...)
	}

	summary := &SkillScanSummary{
		ScannedFiles: scannedFiles,
		Findings:     findings,
	}
	for _, f := range findings {
		switch f.Severity {
		case SeverityCriticalScan:
			summary.Critical++
		case SeverityWarnScan:
			summary.Warn++
		case SeverityInfoScan:
			summary.Info++
		}
	}

	return summary, nil
}
