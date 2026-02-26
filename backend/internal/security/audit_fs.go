// audit_fs.go — 文件系统权限审计。
//
// TS 对照: security/audit-fs.ts (195L)
//
// 检查文件/目录的 POSIX 权限位或 Windows ACL，
// 检测 world/group 可写/可读等安全问题。
package security

import (
	"fmt"
	"os"
	"runtime"
)

// PermissionCheck 权限检查结果。
// TS 对照: audit-fs.ts PermissionCheck
type PermissionCheck struct {
	OK            bool   `json:"ok"`
	IsSymlink     bool   `json:"isSymlink"`
	IsDir         bool   `json:"isDir"`
	Mode          *int   `json:"mode,omitempty"`
	Bits          *int   `json:"bits,omitempty"`
	Source        string `json:"source"` // "posix" | "windows-acl" | "unknown"
	WorldWritable bool   `json:"worldWritable"`
	GroupWritable bool   `json:"groupWritable"`
	WorldReadable bool   `json:"worldReadable"`
	GroupReadable bool   `json:"groupReadable"`
	AclSummary    string `json:"aclSummary,omitempty"`
	Error         string `json:"error,omitempty"`
}

// SafeStat 安全 stat（忽略错误返回默认值）。
// TS 对照: audit-fs.ts safeStat()
type SafeStatResult struct {
	OK        bool
	IsSymlink bool
	IsDir     bool
	Mode      *os.FileMode
	Error     string
}

// SafeStat 执行 lstat 并包装错误。
func SafeStat(targetPath string) SafeStatResult {
	info, err := os.Lstat(targetPath)
	if err != nil {
		return SafeStatResult{
			OK:    false,
			Error: err.Error(),
		}
	}
	mode := info.Mode()
	return SafeStatResult{
		OK:        true,
		IsSymlink: mode&os.ModeSymlink != 0,
		IsDir:     mode.IsDir(),
		Mode:      &mode,
	}
}

// InspectPathPermissions 检查路径的权限。
// 在 POSIX 系统上分析文件权限位，在 Windows 上使用 icacls。
// TS 对照: audit-fs.ts inspectPathPermissions()
func InspectPathPermissions(targetPath string) PermissionCheck {
	st := SafeStat(targetPath)
	if !st.OK {
		return PermissionCheck{
			OK:     false,
			Source: "unknown",
			Error:  st.Error,
		}
	}

	var mode *int
	var bits *int
	if st.Mode != nil {
		m := int(*st.Mode)
		mode = &m
		b := ModeBits(m)
		bits = &b
	}

	platform := runtime.GOOS

	if platform == "windows" {
		acl := InspectWindowsAcl(targetPath)
		if !acl.OK {
			return PermissionCheck{
				OK:        true,
				IsSymlink: st.IsSymlink,
				IsDir:     st.IsDir,
				Mode:      mode,
				Bits:      bits,
				Source:    "unknown",
				Error:     acl.Error,
			}
		}
		return PermissionCheck{
			OK:            true,
			IsSymlink:     st.IsSymlink,
			IsDir:         st.IsDir,
			Mode:          mode,
			Bits:          bits,
			Source:        "windows-acl",
			WorldWritable: hasWritable(acl.UntrustedWorld),
			GroupWritable: hasWritable(acl.UntrustedGroup),
			WorldReadable: hasReadable(acl.UntrustedWorld),
			GroupReadable: hasReadable(acl.UntrustedGroup),
			AclSummary:    FormatWindowsAclSummary(acl),
		}
	}

	// POSIX 权限检查
	var b int
	if bits != nil {
		b = *bits
	}
	return PermissionCheck{
		OK:            true,
		IsSymlink:     st.IsSymlink,
		IsDir:         st.IsDir,
		Mode:          mode,
		Bits:          bits,
		Source:        "posix",
		WorldWritable: IsWorldWritable(b),
		GroupWritable: IsGroupWritable(b),
		WorldReadable: IsWorldReadable(b),
		GroupReadable: IsGroupReadable(b),
	}
}

// FormatPermissionDetail 格式化权限检查详情。
// TS 对照: audit-fs.ts formatPermissionDetail()
func FormatPermissionDetail(targetPath string, perms PermissionCheck) string {
	if perms.Source == "windows-acl" {
		summary := perms.AclSummary
		if summary == "" {
			summary = "unknown"
		}
		return targetPath + " acl=" + summary
	}
	return targetPath + " mode=" + FormatOctal(perms.Bits)
}

// FormatPermissionRemediation 格式化权限修复命令。
// TS 对照: audit-fs.ts formatPermissionRemediation()
func FormatPermissionRemediation(targetPath string, perms PermissionCheck, isDir bool, posixMode int) string {
	if perms.Source == "windows-acl" {
		return FormatIcaclsResetCommand(targetPath, isDir)
	}
	return fmt.Sprintf("chmod %03o %s", posixMode, targetPath)
}

// ---------- 位操作工具 ----------

// ModeBits 提取 POSIX 权限位 (低 9 位)。
// TS 对照: audit-fs.ts modeBits()
func ModeBits(mode int) int {
	return mode & 0o777
}

// FormatOctal 格式化权限位为八进制字符串。
// TS 对照: audit-fs.ts formatOctal()
func FormatOctal(bits *int) string {
	if bits == nil {
		return "unknown"
	}
	return fmt.Sprintf("%03o", *bits)
}

// IsWorldWritable 检查是否 world 可写 (其他用户写位)。
func IsWorldWritable(bits int) bool {
	return bits&0o002 != 0
}

// IsGroupWritable 检查是否 group 可写。
func IsGroupWritable(bits int) bool {
	return bits&0o020 != 0
}

// IsWorldReadable 检查是否 world 可读。
func IsWorldReadable(bits int) bool {
	return bits&0o004 != 0
}

// IsGroupReadable 检查是否 group 可读。
func IsGroupReadable(bits int) bool {
	return bits&0o040 != 0
}

// ---------- 辅助函数 ----------

func hasWritable(entries []WindowsAclEntry) bool {
	for _, e := range entries {
		if e.CanWrite {
			return true
		}
	}
	return false
}

func hasReadable(entries []WindowsAclEntry) bool {
	for _, e := range entries {
		if e.CanRead {
			return true
		}
	}
	return false
}
