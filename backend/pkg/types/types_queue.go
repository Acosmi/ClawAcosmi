package types

// 消息队列类型 — 继承自 src/config/types.queue.ts

// QueueMode 消息队列处理模式
type QueueMode string

const (
	QueueSteer            QueueMode = "steer"
	QueueFollowup         QueueMode = "followup"
	QueueCollect          QueueMode = "collect"
	QueueSteerBacklog     QueueMode = "steer-backlog" // 注意：原版有两种写法
	QueueSteerPlusBacklog QueueMode = "steer+backlog"
	QueueQueue            QueueMode = "queue"
	QueueInterrupt        QueueMode = "interrupt"
)

// QueueDropPolicy 队列溢出丢弃策略
type QueueDropPolicy string

const (
	QueueDropOld       QueueDropPolicy = "old"
	QueueDropNew       QueueDropPolicy = "new"
	QueueDropSummarize QueueDropPolicy = "summarize"
)

// QueueModeByProvider 各频道的队列模式覆盖
// 原版: export type QueueModeByProvider
type QueueModeByProvider struct {
	WhatsApp   QueueMode `json:"whatsapp,omitempty"`
	Telegram   QueueMode `json:"telegram,omitempty"`
	Discord    QueueMode `json:"discord,omitempty"`
	GoogleChat QueueMode `json:"googlechat,omitempty"`
	Slack      QueueMode `json:"slack,omitempty"`
	Signal     QueueMode `json:"signal,omitempty"`
	IMessage   QueueMode `json:"imessage,omitempty"`
	MSTeams    QueueMode `json:"msteams,omitempty"`
	WebChat    QueueMode `json:"webchat,omitempty"`
}

// InboundDebounceByProvider 各频道的入站消息防抖配置(ms)
type InboundDebounceByProvider map[string]int

// QueueConfig 队列配置
// 原版: export type QueueConfig
type QueueConfig struct {
	Mode                QueueMode                 `json:"mode,omitempty"`
	ByChannel           *QueueModeByProvider      `json:"byChannel,omitempty"`
	DebounceMs          *int                      `json:"debounceMs,omitempty"`
	DebounceMsByChannel InboundDebounceByProvider `json:"debounceMsByChannel,omitempty"`
	Cap                 *int                      `json:"cap,omitempty"`
	Drop                QueueDropPolicy           `json:"drop,omitempty"`
}

// InboundDebounceConfig 入站消息防抖配置
type InboundDebounceConfig struct {
	DebounceMs *int                      `json:"debounceMs,omitempty"`
	ByChannel  InboundDebounceByProvider `json:"byChannel,omitempty"`
}
