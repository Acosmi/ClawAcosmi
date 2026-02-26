package cli

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// 对应 TS src/cli/progress.ts — CLI 进度显示
// 简化实现：旋转 spinner + 百分比行进度（不依赖外部库）

// ProgressReporter 进度报告器接口
type ProgressReporter struct {
	label         string
	total         int
	completed     int
	percent       int
	indeterminate bool
	active        bool
	done          chan struct{}
	stream        *os.File
}

var activeProgress atomic.Int32

// spinnerFrames ANSI spinner 帧
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ProgressOptions 进度选项
type ProgressOptions struct {
	Label         string
	Indeterminate bool
	Total         int
	Enabled       bool
}

// noopProgress 空操作进度器
var noopProgress = &ProgressReporter{active: false}

// CreateProgress 创建 CLI 进度报告器。
func CreateProgress(opts ProgressOptions) *ProgressReporter {
	if !opts.Enabled {
		return noopProgress
	}
	if activeProgress.Load() > 0 {
		return noopProgress
	}
	// 非 TTY 不显示进度
	stat, err := os.Stderr.Stat()
	if err != nil || stat.Mode()&os.ModeCharDevice == 0 {
		return noopProgress
	}

	activeProgress.Add(1)
	p := &ProgressReporter{
		label:         opts.Label,
		total:         opts.Total,
		indeterminate: opts.Indeterminate || opts.Total <= 0,
		active:        true,
		done:          make(chan struct{}),
		stream:        os.Stderr,
	}

	// 启动 spinner goroutine
	go p.runSpinner()

	return p
}

// runSpinner 后台运行 spinner 动画
func (p *ProgressReporter) runSpinner() {
	frame := 0
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.done:
			// 清除行
			fmt.Fprintf(p.stream, "\r\033[K")
			return
		case <-ticker.C:
			if p.indeterminate {
				fmt.Fprintf(p.stream, "\r%s %s", spinnerFrames[frame%len(spinnerFrames)], p.label)
			} else {
				fmt.Fprintf(p.stream, "\r%s %s %d%%",
					spinnerFrames[frame%len(spinnerFrames)], p.label, p.percent)
			}
			frame++
		}
	}
}

// SetLabel 更新进度标签
func (p *ProgressReporter) SetLabel(label string) {
	if !p.active {
		return
	}
	p.label = label
}

// SetPercent 设置百分比
func (p *ProgressReporter) SetPercent(percent int) {
	if !p.active {
		return
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	p.percent = percent
	p.indeterminate = false
}

// Tick 增加完成计数
func (p *ProgressReporter) Tick(delta int) {
	if !p.active || p.total <= 0 {
		return
	}
	p.completed += delta
	if p.completed > p.total {
		p.completed = p.total
	}
	p.SetPercent(p.completed * 100 / p.total)
}

// Done 停止进度显示
func (p *ProgressReporter) Done() {
	if !p.active {
		return
	}
	close(p.done)
	activeProgress.Add(-1)
}

// ClearProgressLine 清除当前进度条行。
// 在 log/error 输出前调用，避免进度条残留。
// 对应 TS defaultRuntime.log/error 调用前 clearProgressLine 联动。
func ClearProgressLine() {
	if activeProgress.Load() <= 0 {
		return
	}
	fmt.Fprint(os.Stderr, "\r\033[K")
}

// WithProgress 在执行 fn 期间显示进度，完成后自动停止。
func WithProgress[T any](opts ProgressOptions, fn func(p *ProgressReporter) (T, error)) (T, error) {
	p := CreateProgress(opts)
	defer p.Done()
	return fn(p)
}
