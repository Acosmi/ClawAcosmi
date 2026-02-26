package cli

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// 对应 TS src/cli/banner.ts — ASCII art 龙虾 + 版本 banner

var (
	bannerOnce sync.Once
	// BannerEnabled 控制是否输出 banner（可通过环境变量或 flag 禁用）
	BannerEnabled = true
)

// lobsterASCII 龙虾 ASCII art（与 TS LOBSTER_ASCII 对应）
var lobsterASCII = []string{
	"▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄",
	"██▄▀▀▀▄██░▀▀▀▄██░▀▀▀▀██░▄██░██▄▀▀▀▄██▄▀▀▀▀██▄▀▀▀▄██▄▀▀▀▀██░▄█▄░██▀▀░▀▀██",
	"██░███░██░▀▀▀███░▀▀▀███░█▀▄░██░▀▀▀░██░██████░███░███▀▀▀▄██░█▀█░████░████",
	"██▀▄▄▄▀██░██████░▄▄▄▄██░███░██░███░██▀▄▄▄▄██▀▄▄▄▀██▄▄▄▄▀██░███░██▄▄░▄▄██",
	"▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀",
	"                      🦜 OPENACOSMI 🦜                      ",
}

// taglines 随机标语（精选自 TS tagline.ts）
var taglines = []string{
	"Your AI, your rules",
	"Lobster-grade intelligence",
	"Pinch-perfect conversations",
	"The crustacean of computation",
	"Shell-shocked? We can help",
}

// FormatBannerLine 格式化一行 CLI banner（版本 + commit + tagline）。
func FormatBannerLine() string {
	commit := ResolveCommitHash()
	tag := taglines[0] // 简化: 使用固定标语，可后续增加随机
	return fmt.Sprintf("🦜 OpenAcosmi %s (%s) — %s", Version, commit, tag)
}

// FormatBannerArt 格式化完整 ASCII art banner。
func FormatBannerArt() string {
	return strings.Join(lobsterASCII, "\n")
}

// EmitBanner 输出 CLI banner（仅首次调用生效）。
// 对应 TS emitCliBanner()。
func EmitBanner() {
	if !BannerEnabled {
		return
	}
	if IsTruthyEnv("OPENACOSMI_HIDE_BANNER") {
		return
	}
	// 非 TTY 不输出 banner
	stat, err := os.Stdout.Stat()
	if err != nil || stat.Mode()&os.ModeCharDevice == 0 {
		return
	}
	bannerOnce.Do(func() {
		line := FormatBannerLine()
		fmt.Fprintf(os.Stdout, "\n%s\n\n", line)
	})
}
