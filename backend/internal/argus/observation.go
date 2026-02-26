package argus

// observation.go — VisionObservation 数据结构
//
// 视觉观测帧数据，由 ScreenObserver 产生，存储在 ObservationBuffer 中。
// 主 Agent 通过 ObservationBuffer 查询接口获取最新视觉状态。

import "time"

// ObservationTrigger 观测触发来源。
type ObservationTrigger string

const (
	TriggerScheduled  ObservationTrigger = "scheduled"   // 定时轮询
	TriggerPostAction ObservationTrigger = "post_action" // 动作执行后
	TriggerManual     ObservationTrigger = "manual"      // 用户手动触发
)

// VLMActionResult VLA 模型推理结果（仅关键帧有）。
type VLMActionResult struct {
	Action    string     `json:"action"`   // "CLICK"/"TYPE"/"SCROLL"/"NAVIGATE"/"DONE"
	Value     string     `json:"value"`    // 操作值
	Position  [2]float32 `json:"position"` // 归一化坐标 [x, y] 0.0~1.0
	XPx       int        `json:"x_px"`     // 转换后像素坐标
	YPx       int        `json:"y_px"`
	Reasoning string     `json:"reasoning"` // 推理解释
	RawOutput string     `json:"raw_output"`
}

// VisionObservation 视觉观测帧。
type VisionObservation struct {
	// 标识
	ID        uint64 `json:"id"`
	SessionID string `json:"session_id"`

	// 时间
	CapturedAt      int64         `json:"captured_at"` // Unix ms
	CaptureLatency  time.Duration `json:"capture_latency"`
	VLMInferLatency time.Duration `json:"vlm_infer_latency"` // VLA 推理耗时

	// 内容
	ScreenshotPNG   []byte `json:"screenshot_png,omitempty"`
	ThumbJPEGBase64 string `json:"thumb_b64,omitempty"` // 64×64 缩略图
	Width           int    `json:"width"`
	Height          int    `json:"height"`

	// 变化检测
	FrameHash       uint64  `json:"frame_hash"`       // xxhash64(thumbnail)
	ChangeMagnitude float32 `json:"change_magnitude"` // 0.0~1.0
	IsKeyframe      bool    `json:"is_keyframe"`

	// VLM 分析结果
	VLMAction    *VLMActionResult `json:"vlm_action,omitempty"`
	VLMModelID   string           `json:"vlm_model_id,omitempty"`
	ActiveWindow string           `json:"active_window"`

	// 溯源
	TriggerSource ObservationTrigger `json:"trigger_source"`
	CauseActionID uint64             `json:"cause_action_id,omitempty"`
}
