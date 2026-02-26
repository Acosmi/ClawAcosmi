package config

// 配置版本比较 — 对应 src/config/version.ts (50 行)
//
// 解析 "v1.2.3-4" 格式的版本号并进行语义比较。
// 依赖: 无 npm 依赖，纯正则逻辑。

import (
	"regexp"
	"strconv"
)

// OpenAcosmiVersion 版本号结构
type OpenAcosmiVersion struct {
	Major    int
	Minor    int
	Patch    int
	Revision int
}

// versionRE 匹配 v1.2.3 或 v1.2.3-4 格式
var versionRE = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-(\d+))?`)

// ParseOpenAcosmiVersion 解析版本字符串
// 返回 nil 表示无法解析。
// 对应 TS: parseOpenAcosmiVersion(raw)
func ParseOpenAcosmiVersion(raw string) *OpenAcosmiVersion {
	if raw == "" {
		return nil
	}
	match := versionRE.FindStringSubmatch(raw)
	if match == nil {
		return nil
	}

	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])
	revision := 0
	if match[4] != "" {
		revision, _ = strconv.Atoi(match[4])
	}

	return &OpenAcosmiVersion{
		Major:    major,
		Minor:    minor,
		Patch:    patch,
		Revision: revision,
	}
}

// CompareOpenAcosmiVersions 比较两个版本字符串
// 返回: -1 (a < b), 0 (a == b), 1 (a > b)
// 如果任一版本字符串无法解析，返回 (0, false)
// 对应 TS: compareOpenAcosmiVersions(a, b)
func CompareOpenAcosmiVersions(a, b string) (int, bool) {
	parsedA := ParseOpenAcosmiVersion(a)
	parsedB := ParseOpenAcosmiVersion(b)
	if parsedA == nil || parsedB == nil {
		return 0, false
	}

	if parsedA.Major != parsedB.Major {
		if parsedA.Major < parsedB.Major {
			return -1, true
		}
		return 1, true
	}
	if parsedA.Minor != parsedB.Minor {
		if parsedA.Minor < parsedB.Minor {
			return -1, true
		}
		return 1, true
	}
	if parsedA.Patch != parsedB.Patch {
		if parsedA.Patch < parsedB.Patch {
			return -1, true
		}
		return 1, true
	}
	if parsedA.Revision != parsedB.Revision {
		if parsedA.Revision < parsedB.Revision {
			return -1, true
		}
		return 1, true
	}

	return 0, true
}
