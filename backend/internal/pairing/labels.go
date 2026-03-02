package pairing

// 配对 ID 标签解析 — 对齐 src/pairing/pairing-labels.ts (7L)

import "github.com/openacosmi/claw-acismi/internal/channels"

// ResolvePairingIdLabel 返回渠道的配对 ID 标签。
// 对齐 TS resolvePairingIdLabel()。
func ResolvePairingIdLabel(channel string) string {
	adapter := channels.GetPairingAdapter(channels.ChannelID(channel))
	if adapter != nil {
		label := adapter.IDLabel()
		if label != "" {
			return label
		}
	}
	return "userId"
}
