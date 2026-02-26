package reply

import (
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TS 对照: auto-reply/reply/stage-sandbox-media.ts (197L)
//
// 入站媒体文件的沙箱暂存逻辑：
//   - 从 MsgContext 读取 MediaPaths/MediaPath
//   - 将文件拷贝到沙箱工作区 <workspaceDir>/media/inbound/
//   - 对远端主机使用 scp 下载
//   - 拷贝完成后将 ctx 中的路径改写为相对沙箱路径

// SandboxWorkspace 沙箱工作区信息（DI 接口输出）。
// TS 对照: agents/sandbox.ts SandboxWorkspace
type SandboxWorkspace struct {
	WorkspaceDir string
}

// StageSandboxMediaCtx 入站消息上下文（媒体字段）。
// 从 autoreply.MsgContext 中抽出，避免循环依赖。
// TS 对照: templating.ts MsgContext / TemplateContext 媒体相关字段
type StageSandboxMediaCtx struct {
	MediaPath       string
	MediaPaths      []string
	MediaUrls       []string
	MediaUrl        string
	MediaRemoteHost string // 远端主机名（SCP 源主机）
}

// StageSandboxMediaParams stageSandboxMedia 参数。
// TS 对照: stage-sandbox-media.ts stageSandboxMedia params (L13-18)
type StageSandboxMediaParams struct {
	Ctx          *StageSandboxMediaCtx
	SessionKey   string
	WorkspaceDir string
	// MediaDir 本地媒体目录（安全边界）
	MediaDir string
	// ConfigDir 配置目录（远端媒体缓存使用）
	ConfigDir string
	// Sandbox 当前 session 的沙箱工作区（nil 表示无沙箱）
	Sandbox *SandboxWorkspace
}

// StageSandboxMedia 将入站媒体文件暂存到沙箱工作区。
// TS 对照: stage-sandbox-media.ts stageSandboxMedia (L13-165)
func StageSandboxMedia(params StageSandboxMediaParams) {
	ctx := params.Ctx
	if ctx == nil {
		return
	}
	if params.SessionKey == "" {
		return
	}

	// 收集原始路径列表
	hasPathsArray := len(ctx.MediaPaths) > 0
	var rawPaths []string
	if hasPathsArray {
		rawPaths = ctx.MediaPaths
	} else if strings.TrimSpace(ctx.MediaPath) != "" {
		rawPaths = []string{strings.TrimSpace(ctx.MediaPath)}
	}
	if len(rawPaths) == 0 {
		return
	}

	// 确定目标目录
	// TS: remoteMediaCacheDir = ctx.MediaRemoteHost ? path.join(CONFIG_DIR, "media", "remote-cache", sessionKey) : null
	var effectiveWorkspaceDir string
	if params.Sandbox != nil {
		effectiveWorkspaceDir = params.Sandbox.WorkspaceDir
	} else if ctx.MediaRemoteHost != "" && params.ConfigDir != "" {
		effectiveWorkspaceDir = filepath.Join(params.ConfigDir, "media", "remote-cache", params.SessionKey)
	}
	if effectiveWorkspaceDir == "" {
		return
	}

	// 目标子目录
	// TS: sandbox ? path.join(effectiveWorkspaceDir, "media", "inbound") : effectiveWorkspaceDir
	var destDir string
	if params.Sandbox != nil {
		destDir = filepath.Join(effectiveWorkspaceDir, "media", "inbound")
	} else {
		destDir = effectiveWorkspaceDir
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		slog.Warn("stage_sandbox_media: failed to create dest dir", "dir", destDir, "err", err)
		return
	}

	resolveAbsolutePath := func(value string) string {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return ""
		}
		// file:// URL
		if strings.HasPrefix(trimmed, "file://") {
			parsed, err := url.Parse(trimmed)
			if err != nil {
				return ""
			}
			trimmed = parsed.Path
		}
		if !filepath.IsAbs(trimmed) {
			return ""
		}
		return trimmed
	}

	usedNames := make(map[string]struct{})
	staged := make(map[string]string) // absolute source → staged path

	for _, raw := range rawPaths {
		source := resolveAbsolutePath(raw)
		if source == "" {
			continue
		}
		if _, ok := staged[source]; ok {
			continue
		}

		// 本地路径安全检查：必须在 MediaDir 内
		if ctx.MediaRemoteHost == "" && params.MediaDir != "" {
			if !isUnderDir(source, params.MediaDir) {
				slog.Warn("stage_sandbox_media: blocking media outside media directory", "path", source)
				continue
			}
		}

		baseName := filepath.Base(source)
		if baseName == "" || baseName == "." {
			continue
		}
		ext := filepath.Ext(baseName)
		nameNoExt := strings.TrimSuffix(baseName, ext)

		fileName := baseName
		suffix := 1
		for {
			if _, exists := usedNames[fileName]; !exists {
				break
			}
			fileName = fmt.Sprintf("%s-%d%s", nameNoExt, suffix, ext)
			suffix++
		}
		usedNames[fileName] = struct{}{}

		dest := filepath.Join(destDir, fileName)
		var copyErr error
		if ctx.MediaRemoteHost != "" {
			copyErr = scpFile(ctx.MediaRemoteHost, source, dest)
		} else {
			copyErr = copyFile(source, dest)
		}
		if copyErr != nil {
			slog.Warn("stage_sandbox_media: failed to stage file", "source", source, "err", copyErr)
			continue
		}

		// 存储映射：沙箱时用相对路径，否则用绝对路径
		var stagedPath string
		if params.Sandbox != nil {
			stagedPath = filepath.Join("media", "inbound", fileName)
		} else {
			stagedPath = dest
		}
		staged[source] = stagedPath
	}

	if len(staged) == 0 {
		return
	}

	rewriteIfStaged := func(value string) string {
		raw := strings.TrimSpace(value)
		if raw == "" {
			return value
		}
		abs := resolveAbsolutePath(raw)
		if abs == "" {
			return value
		}
		if mapped, ok := staged[abs]; ok {
			return mapped
		}
		return value
	}

	// 重写 MediaPaths / MediaPath
	if hasPathsArray {
		next := make([]string, len(rawPaths))
		for i, p := range rawPaths {
			next[i] = rewriteIfStaged(p)
		}
		ctx.MediaPaths = next
		if len(next) > 0 {
			ctx.MediaPath = next[0]
		}
	} else {
		rewritten := rewriteIfStaged(ctx.MediaPath)
		if rewritten != ctx.MediaPath {
			ctx.MediaPath = rewritten
		}
	}

	// 重写 MediaUrls
	if len(ctx.MediaUrls) > 0 {
		next := make([]string, len(ctx.MediaUrls))
		for i, u := range ctx.MediaUrls {
			next[i] = rewriteIfStaged(u)
		}
		ctx.MediaUrls = next
	}

	// 重写 MediaUrl
	rewrittenURL := rewriteIfStaged(ctx.MediaUrl)
	if rewrittenURL != ctx.MediaUrl {
		ctx.MediaUrl = rewrittenURL
	}
}

// isUnderDir 判断 path 是否在 dir 目录下（防路径穿越）。
func isUnderDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

// copyFile 拷贝文件。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// scpFile 通过 scp 从远端主机下载文件到本地。
// TS 对照: stage-sandbox-media.ts scpFile (L167-197)
func scpFile(remoteHost, remotePath, localPath string) error {
	src := fmt.Sprintf("%s:%s", remoteHost, remotePath)
	cmd := exec.Command("/usr/bin/scp",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		src,
		localPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp failed: %w — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
