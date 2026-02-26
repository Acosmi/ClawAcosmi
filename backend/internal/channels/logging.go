package channels

import "fmt"

// 频道日志辅助 — 继承自 src/channels/logging.ts (34 行)

// LogInboundDrop 记录入站消息丢弃
func LogInboundDrop(log func(string), channel, reason, target string) {
	t := ""
	if target != "" {
		t = " target=" + target
	}
	log(fmt.Sprintf("%s: drop %s%s", channel, reason, t))
}

// LogTypingFailure 记录打字状态发送失败
func LogTypingFailure(log func(string), channel, target, action string, err error) {
	t := ""
	if target != "" {
		t = " target=" + target
	}
	a := ""
	if action != "" {
		a = " action=" + action
	}
	log(fmt.Sprintf("%s typing%s failed%s: %v", channel, a, t, err))
}

// LogAckFailure 记录确认清理失败
func LogAckFailure(log func(string), channel, target string, err error) {
	t := ""
	if target != "" {
		t = " target=" + target
	}
	log(fmt.Sprintf("%s ack cleanup failed%s: %v", channel, t, err))
}
