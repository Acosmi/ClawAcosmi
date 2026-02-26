package discord

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// Discord Gateway 日志 — 继承自 src/discord/gateway-logging.ts (68L)

// infoDebugMarkers 需要提升为 info 级别的 gateway debug 消息标记
var infoDebugMarkers = []string{
	"WebSocket connection closed",
	"Reconnecting with backoff",
	"Attempting resume with backoff",
}

// shouldPromoteGatewayDebug 判断 gateway debug 消息是否应提升为 info 级别
func shouldPromoteGatewayDebug(message string) bool {
	for _, marker := range infoDebugMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// formatGatewayMetrics 格式化 gateway 指标消息
func formatGatewayMetrics(metrics interface{}) string {
	if metrics == nil {
		return "null"
	}
	switch v := metrics.(type) {
	case string:
		return v
	case int, int64, float64, bool:
		return fmt.Sprintf("%v", v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "[unserializable metrics]"
		}
		return string(b)
	}
}

// GatewayLogger Gateway 日志接口
type GatewayLogger struct {
	// InfoLog 信息级别日志函数（可选，为 nil 时仅输出 debug 日志）
	InfoLog func(msg string)
	// IsVerbose 动态判断是否启用详细日志，替代静态 bool。
	// 对齐 TS logVerbose 的全局动态开关能力 (gateway-logging.ts)
	IsVerbose func() bool
}

// LogGatewayDebug 记录 gateway debug 消息。
// 如果消息匹配 infoDebugMarkers 且 InfoLog 不为 nil，则同时输出 info 级别日志。
func (gl *GatewayLogger) LogGatewayDebug(msg interface{}) {
	message := fmt.Sprintf("%v", msg)
	if gl.IsVerbose != nil && gl.IsVerbose() {
		log.Printf("[discord gateway] %s", message)
	}
	if shouldPromoteGatewayDebug(message) && gl.InfoLog != nil {
		gl.InfoLog(fmt.Sprintf("discord gateway: %s", message))
	}
}

// LogGatewayWarning 记录 gateway warning 消息
func (gl *GatewayLogger) LogGatewayWarning(warning interface{}) {
	if gl.IsVerbose != nil && gl.IsVerbose() {
		log.Printf("[discord gateway warning] %v", warning)
	}
}

// LogGatewayMetrics 记录 gateway 指标
func (gl *GatewayLogger) LogGatewayMetrics(metrics interface{}) {
	if gl.IsVerbose != nil && gl.IsVerbose() {
		log.Printf("[discord gateway metrics] %s", formatGatewayMetrics(metrics))
	}
}

// GatewayEmitter 抽象 gateway 事件发射器。
// 对齐 TS @buape/carbon 的自定义 EventEmitter 接口 (gateway-logging.ts)
type GatewayEmitter interface {
	On(event string, handler func(interface{}))
	RemoveListener(event string, handler func(interface{}))
}

// AttachDiscordGatewayLogging 注册 gateway 日志监听器，返回 cleanup 函数。
// 对齐 TS attachDiscordGatewayLogging (gateway-logging.ts L33-67)
func AttachDiscordGatewayLogging(emitter GatewayEmitter, logger *GatewayLogger) func() {
	if emitter == nil {
		return func() {}
	}

	onDebug := func(msg interface{}) { logger.LogGatewayDebug(msg) }
	onWarning := func(msg interface{}) { logger.LogGatewayWarning(msg) }
	onMetrics := func(msg interface{}) { logger.LogGatewayMetrics(msg) }

	emitter.On("debug", onDebug)
	emitter.On("warning", onWarning)
	emitter.On("metrics", onMetrics)

	return func() {
		emitter.RemoveListener("debug", onDebug)
		emitter.RemoveListener("warning", onWarning)
		emitter.RemoveListener("metrics", onMetrics)
	}
}
