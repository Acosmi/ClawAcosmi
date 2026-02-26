package cli

import (
	"strconv"
	"strings"
)

// 对应 TS src/cli/argv.ts — 参数解析工具函数

// HasFlag 检查 argv 中是否存在指定 flag（-- 之前）。
func HasFlag(args []string, name string) bool {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == name {
			return true
		}
	}
	return false
}

// GetFlagValue 获取 flag 的值。
// 返回值含义：
//   - (value, true) — flag 存在且有值
//   - ("", true)    — flag 存在但无值
//   - ("", false)   — flag 不存在
func GetFlagValue(args []string, name string) (string, bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if arg == name {
			// 下一个参数作为值
			if i+1 < len(args) && isValueToken(args[i+1]) {
				return args[i+1], true
			}
			return "", true
		}
		if strings.HasPrefix(arg, name+"=") {
			val := arg[len(name)+1:]
			return val, true
		}
	}
	return "", false
}

// isValueToken 判断参数是否为值（非 flag）
func isValueToken(arg string) bool {
	if arg == "" || arg == "--" {
		return false
	}
	if !strings.HasPrefix(arg, "-") {
		return true
	}
	// 负数也是值
	_, err := strconv.ParseFloat(strings.TrimPrefix(arg, "-"), 64)
	return err == nil
}

// GetVerboseFlag 检查是否启用了 verbose 模式。
func GetVerboseFlag(args []string, includeDebug bool) bool {
	if HasFlag(args, "--verbose") {
		return true
	}
	if includeDebug && HasFlag(args, "--debug") {
		return true
	}
	return false
}

// GetPositiveIntFlag 获取正整数 flag 值。
// 返回 -1 表示 flag 不存在，0 表示无效值。
func GetPositiveIntFlag(args []string, name string) int {
	val, found := GetFlagValue(args, name)
	if !found {
		return -1
	}
	if val == "" {
		return 0
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// GetCommandPath 提取命令路径（跳过 flag），最多提取 depth 层。
// 对应 TS getCommandPath(argv, depth)。
func GetCommandPath(args []string, depth int) []string {
	var path []string
	for _, arg := range args {
		if arg == "" {
			continue
		}
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		path = append(path, arg)
		if len(path) >= depth {
			break
		}
	}
	return path
}

// GetPrimaryCommand 获取第一层命令名。
func GetPrimaryCommand(args []string) string {
	path := GetCommandPath(args, 1)
	if len(path) == 0 {
		return ""
	}
	return path[0]
}

// HasHelpOrVersion 检查是否有 --help/-h 或 --version/-v/-V flag。
func HasHelpOrVersion(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-h", "--help", "-v", "-V", "--version":
			return true
		}
	}
	return false
}

// RewriteUpdateFlagArgv 将 --update flag 重写为 update 子命令。
// 对应 TS run-main.ts rewriteUpdateFlagArgv()。
// 例：["openacosmi", "--update"] → ["openacosmi", "update"]
func RewriteUpdateFlagArgv(argv []string) []string {
	idx := -1
	for i, arg := range argv {
		if arg == "--update" {
			idx = i
			break
		}
	}
	if idx == -1 {
		return argv
	}
	out := make([]string, 0, len(argv))
	for i, arg := range argv {
		if i == idx {
			out = append(out, "update")
		} else {
			out = append(out, arg)
		}
	}
	return out
}
