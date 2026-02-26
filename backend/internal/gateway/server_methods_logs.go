package gateway

// server_methods_logs.go — logs.tail
// 对应 TS src/gateway/server-methods/logs.ts
//
// 逻辑自包含：file stat → seek → read → split lines → 返回。
// 支持 rolling log 文件解析、cursor 续读、maxBytes 限制。

import (
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	defaultLogLimit    = 500
	defaultLogMaxBytes = 250_000
	maxLogLimit        = 5000
	maxLogBytes        = 1_000_000
)

var rollingLogRE = regexp.MustCompile(`^openacosmi-\d{4}-\d{2}-\d{2}\.log$`)

// LogsHandlers 返回 logs.* 方法处理器映射。
func LogsHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"logs.tail": handleLogsTail,
	}
}

// ---------- logs.tail ----------
// 对应 TS logs.ts L150-183

func handleLogsTail(ctx *MethodHandlerContext) {
	var cursorPtr *int64
	if v, ok := ctx.Params["cursor"]; ok {
		if f, ok := v.(float64); ok && !math.IsNaN(f) && !math.IsInf(f, 0) {
			c := int64(math.Max(0, math.Floor(f)))
			cursorPtr = &c
		}
	}

	limitRaw := defaultLogLimit
	if v, ok := ctx.Params["limit"]; ok {
		if f, ok := v.(float64); ok {
			limitRaw = int(f)
		}
	}

	maxBytesRaw := defaultLogMaxBytes
	if v, ok := ctx.Params["maxBytes"]; ok {
		if f, ok := v.(float64); ok {
			maxBytesRaw = int(f)
		}
	}

	// 解析日志文件路径
	logFile := ctx.Context.LogFilePath
	if logFile == "" {
		ctx.Respond(true, map[string]interface{}{
			"file":      "",
			"cursor":    0,
			"size":      0,
			"lines":     []string{},
			"truncated": false,
			"reset":     false,
		}, nil)
		return
	}

	resolved := resolveLogFile(logFile)
	result := readLogSlice(resolved, cursorPtr, limitRaw, maxBytesRaw)
	result["file"] = resolved
	ctx.Respond(true, result, nil)
}

// resolveLogFile 解析日志文件路径（如果文件不存在，尝试找最新的 rolling log）。
func resolveLogFile(file string) string {
	if _, err := os.Stat(file); err == nil {
		return file
	}
	if !rollingLogRE.MatchString(filepath.Base(file)) {
		return file
	}

	dir := filepath.Dir(file)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return file
	}

	type candidate struct {
		path    string
		modTime int64
	}
	var candidates []candidate
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if rollingLogRE.MatchString(e.Name()) {
			fullPath := filepath.Join(dir, e.Name())
			info, err := e.Info()
			if err != nil {
				continue
			}
			candidates = append(candidates, candidate{path: fullPath, modTime: info.ModTime().UnixMilli()})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime > candidates[j].modTime
	})

	if len(candidates) > 0 {
		return candidates[0].path
	}
	return file
}

// readLogSlice 读取日志文件的尾部切片。
func readLogSlice(file string, cursor *int64, limit, maxBytes int) map[string]interface{} {
	stat, err := os.Stat(file)
	if err != nil {
		return map[string]interface{}{
			"cursor":    int64(0),
			"size":      int64(0),
			"lines":     []string{},
			"truncated": false,
			"reset":     false,
		}
	}

	size := stat.Size()
	mb := clampInt(maxBytes, 1, maxLogBytes)
	lim := clampInt(limit, 1, maxLogLimit)

	var start int64
	reset := false
	truncated := false

	if cursor != nil {
		c := *cursor
		if c > size {
			reset = true
			start = logMax64(0, size-int64(mb))
			truncated = start > 0
		} else {
			start = c
			if size-start > int64(mb) {
				reset = true
				truncated = true
				start = logMax64(0, size-int64(mb))
			}
		}
	} else {
		start = logMax64(0, size-int64(mb))
		truncated = start > 0
	}

	if size == 0 || size <= start {
		return map[string]interface{}{
			"cursor":    size,
			"size":      size,
			"lines":     []string{},
			"truncated": truncated,
			"reset":     reset,
		}
	}

	f, err := os.Open(file)
	if err != nil {
		return map[string]interface{}{
			"cursor":    int64(0),
			"size":      int64(0),
			"lines":     []string{},
			"truncated": false,
			"reset":     false,
		}
	}
	defer f.Close()

	// 检查 start 前一个字符是否为换行
	prefix := ""
	if start > 0 {
		prefixBuf := make([]byte, 1)
		if _, perr := f.ReadAt(prefixBuf, start-1); perr == nil {
			prefix = string(prefixBuf)
		}
	}

	length := size - start
	buf := make([]byte, length)
	n, err := f.ReadAt(buf, start)
	if err != nil && err != io.EOF {
		return map[string]interface{}{
			"cursor":    int64(0),
			"size":      int64(0),
			"lines":     []string{},
			"truncated": false,
			"reset":     false,
		}
	}

	text := string(buf[:n])
	lines := strings.Split(text, "\n")

	// 如果从中间开始且前一个字符不是换行，跳过第一行（不完整）
	if start > 0 && prefix != "\n" && len(lines) > 0 {
		lines = lines[1:]
	}
	// 去掉末尾空行
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	// 限制行数
	if len(lines) > lim {
		lines = lines[len(lines)-lim:]
	}

	return map[string]interface{}{
		"cursor":    size,
		"size":      size,
		"lines":     lines,
		"truncated": truncated,
		"reset":     reset,
	}
}

func logMax64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
