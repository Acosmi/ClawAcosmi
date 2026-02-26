package plugins

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// --- 归档处理 ---

// isArchiveFile 判断是否为支持的归档文件
func isArchiveFile(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz")
}

// extractTarGz 解压 .tar.gz / .tgz 文件
// 对应 TS: archive.ts extractArchive
func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		// 安全检查：防止路径遍历
		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			continue
		}

		target := filepath.Join(destDir, cleanName)
		if !IsPathInside(destDir, target) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", target, err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode&0o777))
			if err != nil {
				return fmt.Errorf("create %s: %w", target, err)
			}
			// 限制单文件最大 100MB 防止 zip bomb
			if _, err := io.Copy(out, io.LimitReader(tr, 100<<20)); err != nil {
				out.Close()
				return fmt.Errorf("write %s: %w", target, err)
			}
			out.Close()
		}
	}
	return nil
}

// resolvePackedRootDir 找到解压后的根目录
// npm pack 通常创建 package/ 子目录
func resolvePackedRootDir(extractDir string) (string, error) {
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", fmt.Errorf("failed to read extract dir: %w", err)
	}

	// 单个子目录 → 使用该子目录作为根
	dirs := make([]os.DirEntry, 0)
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		}
	}
	if len(dirs) == 1 {
		return filepath.Join(extractDir, dirs[0].Name()), nil
	}

	// 多个条目或无子目录 → 检查是否有 package.json
	if _, err := os.Stat(filepath.Join(extractDir, "package.json")); err == nil {
		return extractDir, nil
	}

	if len(dirs) == 0 {
		return "", fmt.Errorf("no package directory found in archive")
	}
	// 多个目录：优先选 "package"
	for _, d := range dirs {
		if d.Name() == "package" {
			return filepath.Join(extractDir, "package"), nil
		}
	}
	return filepath.Join(extractDir, dirs[0].Name()), nil
}

// --- 目录复制 ---

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// --- 命令执行 ---

// runNpmInstall 在指定目录执行 npm install --omit=dev
func runNpmInstall(dir string, timeoutMs int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "npm", "install", "--omit=dev", "--silent")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err.Error(), strings.TrimSpace(string(output)))
	}
	return nil
}
