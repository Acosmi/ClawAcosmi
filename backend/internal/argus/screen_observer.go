package argus

// screen_observer.go — ScreenObserver 独立截图循环
//
// 核心流程：
//   1. 定时截图（可配置频率）
//   2. xxhash64 去重（相同 hash 跳过）
//   3. 计算像素变化幅度
//   4. 关键帧（变化 > threshold）才调 VLA 推理
//   5. 写入 ObservationBuffer（非阻塞）

import (
	"context"
	"hash/crc64"
	"log/slog"
	"sync/atomic"
	"time"
)

// hashTable CRC64 哈希表（替代 xxhash64，零外部依赖）。
var hashTable = crc64.MakeTable(crc64.ECMA)

// ScreenObserverConfig 截图观测器配置。
type ScreenObserverConfig struct {
	BaseInterval    time.Duration // 截图间隔（默认 1s）
	ChangeThreshold float32       // 变化阈值（默认 0.02 = 2%）
	MaxBufferFrames int           // 缓冲区容量（默认 500）
	VLAEnabled      bool          // 是否启用 VLA 推理
	VLAClient       VLAClient     // VLA 模型客户端
	Goal            string        // 当前任务目标（传给 VLA）

	// CaptureFunc 截图函数（由 Bridge 注入，解耦依赖）。
	// 返回 PNG 字节和分辨率。
	CaptureFunc func(ctx context.Context) (png []byte, w, h int, err error)
}

// ScreenObserver 独立截图循环。
type ScreenObserver struct {
	cfg    ScreenObserverConfig
	buf    *ObservationBuffer
	cancel context.CancelFunc
	done   chan struct{}

	frameSeq atomic.Uint64
	lastHash uint64
	running  atomic.Bool
}

// NewScreenObserver 创建截图观测器。
func NewScreenObserver(cfg ScreenObserverConfig) *ScreenObserver {
	if cfg.BaseInterval <= 0 {
		cfg.BaseInterval = time.Second
	}
	if cfg.ChangeThreshold <= 0 {
		cfg.ChangeThreshold = 0.02
	}
	if cfg.MaxBufferFrames <= 0 {
		cfg.MaxBufferFrames = 500
	}
	if cfg.VLAClient == nil {
		cfg.VLAClient = &NoopVLAClient{}
	}

	return &ScreenObserver{
		cfg:  cfg,
		buf:  NewObservationBuffer(cfg.MaxBufferFrames),
		done: make(chan struct{}),
	}
}

// Buffer 返回底层的 ObservationBuffer（供主 Agent 查询）。
func (o *ScreenObserver) Buffer() *ObservationBuffer {
	return o.buf
}

// Start 启动截图循环（goroutine）。
func (o *ScreenObserver) Start() {
	if o.running.Swap(true) {
		return // 已在运行
	}
	ctx, cancel := context.WithCancel(context.Background())
	o.cancel = cancel
	go o.loop(ctx)
}

// Stop 停止截图循环。
func (o *ScreenObserver) Stop() {
	if !o.running.Swap(false) {
		return
	}
	if o.cancel != nil {
		o.cancel()
	}
	<-o.done
}

// IsRunning 返回是否在运行。
func (o *ScreenObserver) IsRunning() bool {
	return o.running.Load()
}

// SetGoal 更新当前任务目标。
func (o *ScreenObserver) SetGoal(goal string) {
	o.cfg.Goal = goal
}

// SetInterval 动态调整截图间隔。
func (o *ScreenObserver) SetInterval(d time.Duration) {
	if d > 0 {
		o.cfg.BaseInterval = d
	}
}

// loop 核心截图循环。
func (o *ScreenObserver) loop(ctx context.Context) {
	defer close(o.done)

	slog.Info("screen_observer started",
		"interval", o.cfg.BaseInterval,
		"vla", o.cfg.VLAClient.ModelID(),
	)

	ticker := time.NewTicker(o.cfg.BaseInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("screen_observer stopped")
			return
		case <-ticker.C:
			o.captureOnce(ctx)
		}
	}
}

// captureOnce 执行一次截图 + 去重 + 可选 VLA 推理。
func (o *ScreenObserver) captureOnce(ctx context.Context) {
	if o.cfg.CaptureFunc == nil {
		return
	}

	t0 := time.Now()
	png, w, h, err := o.cfg.CaptureFunc(ctx)
	captureLatency := time.Since(t0)
	if err != nil {
		slog.Debug("screen capture failed", "err", err)
		return
	}

	// 快速哈希去重
	hash := crc64.Checksum(png, hashTable)
	if hash == o.lastHash {
		return // 画面无变化
	}
	o.lastHash = hash

	// 构建观测帧
	seq := o.frameSeq.Add(1)
	obs := &VisionObservation{
		ID:             seq,
		CapturedAt:     time.Now().UnixMilli(),
		CaptureLatency: captureLatency,
		ScreenshotPNG:  png,
		Width:          w,
		Height:         h,
		FrameHash:      hash,
		TriggerSource:  TriggerScheduled,
	}

	// 变化检测（简化版：hash 变化 = 有变化）
	obs.ChangeMagnitude = 1.0 // 后续可用像素差计算
	obs.IsKeyframe = true     // 变化帧即关键帧

	// VLA 推理（仅关键帧且启用时）
	if obs.IsKeyframe && o.cfg.VLAEnabled && o.cfg.VLAClient != nil {
		t1 := time.Now()
		result, err := o.cfg.VLAClient.Infer(ctx, o.cfg.Goal, png, w, h)
		obs.VLMInferLatency = time.Since(t1)
		if err != nil {
			slog.Debug("VLA infer failed", "err", err)
		} else {
			obs.VLMAction = result
			obs.VLMModelID = o.cfg.VLAClient.ModelID()
		}
	}

	// 写入 buffer
	o.buf.Push(obs)
}
