package channels

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ChannelDistributeResult 频道 VFS 分级结果。
type ChannelDistributeResult struct {
	Indexed  int           `json:"indexed"`
	Skipped  int           `json:"skipped"`
	Duration time.Duration `json:"duration"`
}

// ChannelVFSWriter 频道分级写入接口（避免直接依赖 uhms 包）。
type ChannelVFSWriter interface {
	WriteSystemEntry(namespace, category, id, l0, l1, l2 string, meta map[string]interface{}) error
	SystemEntryExists(namespace, category, id string) bool
}

// DistributeChannels 将所有频道元数据写入 VFS _system/plugins/ 命名空间。
// 频道元数据是静态的，不需要增量 hash — 直接覆盖即可。
func DistributeChannels(vfs ChannelVFSWriter) (*ChannelDistributeResult, error) {
	start := time.Now()
	result := &ChannelDistributeResult{}

	for _, chID := range chatChannelOrder {
		meta, ok := chatChannelMeta[chID]
		if !ok {
			continue
		}

		l0 := generateChannelL0(meta)
		l1 := generateChannelL1(meta)
		l2 := generateChannelL2(meta)

		metaMap := map[string]interface{}{
			"name":        string(meta.ID),
			"label":       meta.Label,
			"type":        "channel",
			"distributed": true,
		}

		if err := vfs.WriteSystemEntry("plugins", "channels", string(meta.ID), l0, l1, l2, metaMap); err != nil {
			slog.Warn("channel_distributor: write failed", "channel", meta.ID, "error", err)
			continue
		}
		result.Indexed++
	}

	result.Duration = time.Since(start)
	slog.Info("channel_distributor: distribute complete",
		"indexed", result.Indexed,
		"duration", result.Duration,
	)
	return result, nil
}

// generateChannelL0 生成频道 L0 摘要 (~50 tokens)。
func generateChannelL0(meta ChannelMetaEntry) string {
	label := meta.Label
	if meta.SelectionLabel != "" {
		label = meta.SelectionLabel
	}
	return fmt.Sprintf("%s: %s [type: channel]", label, meta.Blurb)
}

// generateChannelL1 生成频道 L1 概览 (~200 tokens)。
func generateChannelL1(meta ChannelMetaEntry) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", meta.Label))
	sb.WriteString(fmt.Sprintf("**ID**: %s\n", meta.ID))
	if meta.SelectionLabel != "" {
		sb.WriteString(fmt.Sprintf("**Full Name**: %s\n", meta.SelectionLabel))
	}
	sb.WriteString(fmt.Sprintf("**Description**: %s\n", meta.Blurb))
	if meta.DocsPath != "" {
		sb.WriteString(fmt.Sprintf("**Docs**: %s\n", meta.DocsPath))
	}
	if meta.DetailLabel != "" {
		sb.WriteString(fmt.Sprintf("**Detail**: %s\n", meta.DetailLabel))
	}
	sb.WriteString(fmt.Sprintf("**Order**: %d\n", meta.Order))
	return sb.String()
}

// generateChannelL2 生成频道 L2 完整内容 — 频道元数据本身较短，L2 与 L1 相同。
func generateChannelL2(meta ChannelMetaEntry) string {
	return generateChannelL1(meta)
}

// CollectDistributedChannelIDs 返回已分级的频道 ID 列表。
func CollectDistributedChannelIDs() []string {
	ids := make([]string, 0, len(chatChannelOrder))
	for _, chID := range chatChannelOrder {
		ids = append(ids, string(chID))
	}
	return ids
}
