// windows_acl.go — Windows ACL (icacls) 解析与检查。
//
// TS 对照: security/windows-acl.ts (229L)
//
// 通过 icacls 命令检查 Windows 文件/目录的 ACL，
// 分类 principal 为 trusted/world/group，检测不安全权限。
// 在非 Windows 平台上运行时功能退化为 no-op。
package security

import (
	"os/exec"
	"os/user"
	"runtime"
	"strings"
)

// WindowsAclEntry 一条 ACL 条目。
// TS 对照: windows-acl.ts WindowsAclEntry
type WindowsAclEntry struct {
	Principal string
	Rights    []string
	RawRights string
	CanRead   bool
	CanWrite  bool
}

// WindowsAclSummary ACL 检查摘要。
// TS 对照: windows-acl.ts WindowsAclSummary
type WindowsAclSummary struct {
	OK             bool
	Entries        []WindowsAclEntry
	UntrustedWorld []WindowsAclEntry
	UntrustedGroup []WindowsAclEntry
	Trusted        []WindowsAclEntry
	Error          string
}

// 继承标记（不影响权限判断）。
var inheritFlags = map[string]bool{
	"I": true, "OI": true, "CI": true, "IO": true, "NP": true,
}

// 公共世界 principal。
var worldPrincipals = map[string]bool{
	"everyone":                          true,
	"users":                             true,
	"builtin\\users":                    true,
	"authenticated users":               true,
	"nt authority\\authenticated users": true,
}

// 受信任基础 principal。
var trustedBase = map[string]bool{
	"nt authority\\system":    true,
	"system":                  true,
	"builtin\\administrators": true,
	"creator owner":           true,
}

var worldSuffixes = []string{"\\users", "\\authenticated users"}
var trustedSuffixes = []string{"\\administrators", "\\system"}

// ResolveWindowsUserPrincipal 解析当前 Windows 用户 principal。
// 在非 Windows 平台返回空字符串。
// TS 对照: windows-acl.ts resolveWindowsUserPrincipal()
func ResolveWindowsUserPrincipal() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return u.Username
}

// buildTrustedPrincipals 构建受信任 principal 集合。
func buildTrustedPrincipals() map[string]bool {
	trusted := make(map[string]bool)
	for k, v := range trustedBase {
		trusted[k] = v
	}
	principal := ResolveWindowsUserPrincipal()
	if principal != "" {
		normalized := strings.ToLower(strings.TrimSpace(principal))
		trusted[normalized] = true
		// 也添加不含域名部分
		if idx := strings.LastIndex(normalized, "\\"); idx >= 0 {
			trusted[normalized[idx+1:]] = true
		}
	}
	return trusted
}

// classifyPrincipal 分类 principal 为 trusted/world/group。
func classifyPrincipal(principal string) string {
	normalized := strings.ToLower(strings.TrimSpace(principal))
	trusted := buildTrustedPrincipals()

	if trusted[normalized] {
		return "trusted"
	}
	for _, suffix := range trustedSuffixes {
		if strings.HasSuffix(normalized, suffix) {
			return "trusted"
		}
	}
	if worldPrincipals[normalized] {
		return "world"
	}
	for _, suffix := range worldSuffixes {
		if strings.HasSuffix(normalized, suffix) {
			return "world"
		}
	}
	return "group"
}

// rightsFromTokens 从权限 token 列表中解析读写权限。
func rightsFromTokens(tokens []string) (canRead, canWrite bool) {
	upper := strings.ToUpper(strings.Join(tokens, ""))
	canWrite = strings.ContainsAny(upper, "FMW") || strings.Contains(upper, "D")
	canRead = strings.ContainsAny(upper, "FMR")
	return
}

// ParseIcaclsOutput 解析 icacls 命令输出。
// TS 对照: windows-acl.ts parseIcaclsOutput()
func ParseIcaclsOutput(output, targetPath string) []WindowsAclEntry {
	var entries []WindowsAclEntry
	normalizedTarget := strings.TrimSpace(targetPath)
	lowerTarget := strings.ToLower(normalizedTarget)
	quotedLower := strings.ToLower(`"` + normalizedTarget + `"`)

	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimRight(rawLine, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)

		// 跳过状态行
		if strings.HasPrefix(lower, "successfully processed") ||
			strings.HasPrefix(lower, "processed") ||
			strings.HasPrefix(lower, "failed processing") ||
			strings.HasPrefix(lower, "no mapping between account names") {
			continue
		}

		// 剥离目标路径前缀
		entry := trimmed
		if strings.HasPrefix(lower, lowerTarget) {
			entry = strings.TrimSpace(trimmed[len(normalizedTarget):])
		} else if strings.HasPrefix(lower, quotedLower) {
			entry = strings.TrimSpace(trimmed[len(normalizedTarget)+2:])
		}
		if entry == "" {
			continue
		}

		// 分割 principal 和 rights
		idx := strings.IndexByte(entry, ':')
		if idx < 0 {
			continue
		}
		principal := strings.TrimSpace(entry[:idx])
		rawRights := strings.TrimSpace(entry[idx+1:])

		// 提取括号内的 token
		var tokens []string
		for _, part := range strings.Split(rawRights, "(") {
			part = strings.TrimSuffix(strings.TrimSpace(part), ")")
			if part != "" {
				tokens = append(tokens, part)
			}
		}

		// 跳过 DENY 条目
		hasDeny := false
		for _, t := range tokens {
			if strings.ToUpper(t) == "DENY" {
				hasDeny = true
				break
			}
		}
		if hasDeny {
			continue
		}

		// 过滤继承标记
		var rights []string
		for _, t := range tokens {
			if !inheritFlags[strings.ToUpper(t)] {
				rights = append(rights, t)
			}
		}
		if len(rights) == 0 {
			continue
		}

		canRead, canWrite := rightsFromTokens(rights)
		entries = append(entries, WindowsAclEntry{
			Principal: principal,
			Rights:    rights,
			RawRights: rawRights,
			CanRead:   canRead,
			CanWrite:  canWrite,
		})
	}

	return entries
}

// SummarizeWindowsAcl 对 ACL 条目按信任级别分类。
// TS 对照: windows-acl.ts summarizeWindowsAcl()
func SummarizeWindowsAcl(entries []WindowsAclEntry) (trusted, untrustedWorld, untrustedGroup []WindowsAclEntry) {
	for _, entry := range entries {
		switch classifyPrincipal(entry.Principal) {
		case "trusted":
			trusted = append(trusted, entry)
		case "world":
			untrustedWorld = append(untrustedWorld, entry)
		default:
			untrustedGroup = append(untrustedGroup, entry)
		}
	}
	return
}

// InspectWindowsAcl 执行 icacls 并解析 ACL。
// 在非 Windows 平台返回 ok: false。
// TS 对照: windows-acl.ts inspectWindowsAcl()
func InspectWindowsAcl(targetPath string) WindowsAclSummary {
	if runtime.GOOS != "windows" {
		return WindowsAclSummary{
			OK:    false,
			Error: "not windows platform",
		}
	}

	cmd := exec.Command("icacls", targetPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return WindowsAclSummary{
			OK:    false,
			Error: err.Error(),
		}
	}

	entries := ParseIcaclsOutput(string(out), targetPath)
	tr, uw, ug := SummarizeWindowsAcl(entries)

	return WindowsAclSummary{
		OK:             true,
		Entries:        entries,
		Trusted:        tr,
		UntrustedWorld: uw,
		UntrustedGroup: ug,
	}
}

// FormatWindowsAclSummary 格式化 ACL 摘要为可读字符串。
// TS 对照: windows-acl.ts formatWindowsAclSummary()
func FormatWindowsAclSummary(summary WindowsAclSummary) string {
	if !summary.OK {
		return "unknown"
	}
	untrusted := append(summary.UntrustedWorld, summary.UntrustedGroup...)
	if len(untrusted) == 0 {
		return "trusted-only"
	}
	var parts []string
	for _, entry := range untrusted {
		parts = append(parts, entry.Principal+":"+entry.RawRights)
	}
	return strings.Join(parts, ", ")
}

// FormatIcaclsResetCommand 格式化 icacls 重置命令。
// TS 对照: windows-acl.ts formatIcaclsResetCommand()
func FormatIcaclsResetCommand(targetPath string, isDir bool) string {
	u := ResolveWindowsUserPrincipal()
	if u == "" {
		u = "%USERNAME%"
	}
	grant := "F"
	if isDir {
		grant = "(OI)(CI)F"
	}
	return `icacls "` + targetPath + `" /inheritance:r /grant:r "` + u + `:` + grant + `" /grant:r "SYSTEM:` + grant + `"`
}

// CreateIcaclsResetCommand 创建 icacls 重置命令的结构化表示。
// TS 对照: windows-acl.ts createIcaclsResetCommand()
type IcaclsResetCommand struct {
	Command string
	Args    []string
	Display string
}

// CreateIcaclsResetCommand 返回值。如果无法获取用户名返回 nil。
func CreateIcaclsResetCommand(targetPath string, isDir bool) *IcaclsResetCommand {
	u := ResolveWindowsUserPrincipal()
	if u == "" {
		return nil
	}
	grant := "F"
	if isDir {
		grant = "(OI)(CI)F"
	}
	args := []string{
		targetPath,
		"/inheritance:r",
		"/grant:r", u + ":" + grant,
		"/grant:r", "SYSTEM:" + grant,
	}
	return &IcaclsResetCommand{
		Command: "icacls",
		Args:    args,
		Display: FormatIcaclsResetCommand(targetPath, isDir),
	}
}
