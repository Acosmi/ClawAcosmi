// tools/nodes_utils.go — 节点工具辅助函数。
// TS 参考：src/agents/tools/nodes-utils.ts (178L)
// 全量移植：节点列表解析 + 配对列表 + 模糊匹配 + 默认节点选择
package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// ---------- 类型定义 ----------

// NodeListNode 节点列表中的节点信息。
type NodeListNode struct {
	NodeID          string          `json:"nodeId"`
	DisplayName     string          `json:"displayName,omitempty"`
	Platform        string          `json:"platform,omitempty"`
	Version         string          `json:"version,omitempty"`
	CoreVersion     string          `json:"coreVersion,omitempty"`
	UIVersion       string          `json:"uiVersion,omitempty"`
	RemoteIP        string          `json:"remoteIp,omitempty"`
	DeviceFamily    string          `json:"deviceFamily,omitempty"`
	ModelIdentifier string          `json:"modelIdentifier,omitempty"`
	Caps            []string        `json:"caps,omitempty"`
	Commands        []string        `json:"commands,omitempty"`
	Permissions     map[string]bool `json:"permissions,omitempty"`
	Paired          bool            `json:"paired,omitempty"`
	Connected       bool            `json:"connected,omitempty"`
}

// PendingPairRequest 待处理的配对请求。
type PendingPairRequest struct {
	RequestID   string `json:"requestId"`
	NodeID      string `json:"nodeId"`
	DisplayName string `json:"displayName,omitempty"`
	Platform    string `json:"platform,omitempty"`
	Version     string `json:"version,omitempty"`
	CoreVersion string `json:"coreVersion,omitempty"`
	UIVersion   string `json:"uiVersion,omitempty"`
	RemoteIP    string `json:"remoteIp,omitempty"`
	IsRepair    bool   `json:"isRepair,omitempty"`
	Ts          int64  `json:"ts"`
}

// PairedNode 已配对节点。
type PairedNode struct {
	NodeID       string          `json:"nodeId"`
	Token        string          `json:"token,omitempty"`
	DisplayName  string          `json:"displayName,omitempty"`
	Platform     string          `json:"platform,omitempty"`
	Version      string          `json:"version,omitempty"`
	CoreVersion  string          `json:"coreVersion,omitempty"`
	UIVersion    string          `json:"uiVersion,omitempty"`
	RemoteIP     string          `json:"remoteIp,omitempty"`
	Permissions  map[string]bool `json:"permissions,omitempty"`
	CreatedAtMs  int64           `json:"createdAtMs,omitempty"`
	ApprovedAtMs int64           `json:"approvedAtMs,omitempty"`
}

// PairingList 配对列表。
type PairingList struct {
	Pending []PendingPairRequest `json:"pending"`
	Paired  []PairedNode         `json:"paired"`
}

// ---------- 解析 ----------

var nodeKeyNormRE = regexp.MustCompile(`[^a-z0-9]+`)
var nodeKeyLeadDashRE = regexp.MustCompile(`^-+`)
var nodeKeyTrailDashRE = regexp.MustCompile(`-+$`)

// normalizeNodeKey 将节点标识标准化为可比较的 key。
// 对齐 TS: normalizeNodeKey()
func normalizeNodeKey(value string) string {
	result := strings.ToLower(value)
	result = nodeKeyNormRE.ReplaceAllString(result, "-")
	result = nodeKeyLeadDashRE.ReplaceAllString(result, "")
	result = nodeKeyTrailDashRE.ReplaceAllString(result, "")
	return result
}

// ---------- 默认节点选择 ----------

// PickDefaultNode 从节点列表中选择默认节点。
// 优先选择有 canvas 能力且已连接的本地 Mac 节点。
// 对齐 TS: pickDefaultNode()
func PickDefaultNode(nodes []NodeListNode) *NodeListNode {
	// 过滤：有 canvas cap 或无 cap 信息的节点
	var withCanvas []NodeListNode
	for _, n := range nodes {
		if len(n.Caps) == 0 || containsStr(n.Caps, "canvas") {
			withCanvas = append(withCanvas, n)
		}
	}
	if len(withCanvas) == 0 {
		return nil
	}

	// 优先已连接
	var connected []NodeListNode
	for _, n := range withCanvas {
		if n.Connected {
			connected = append(connected, n)
		}
	}
	candidates := withCanvas
	if len(connected) > 0 {
		candidates = connected
	}
	if len(candidates) == 1 {
		return &candidates[0]
	}

	// 再优先本地 Mac
	var local []NodeListNode
	for _, n := range candidates {
		if strings.HasPrefix(strings.ToLower(n.Platform), "mac") && strings.HasPrefix(n.NodeID, "mac-") {
			local = append(local, n)
		}
	}
	if len(local) == 1 {
		return &local[0]
	}
	return nil
}

// ---------- 节点 ID 解析 ----------

// ResolveNodeIDFromList 从节点列表中解析节点 ID。
// 支持精确匹配、IP 匹配、显示名匹配、前缀匹配。
// 对齐 TS: resolveNodeIdFromList()
func ResolveNodeIDFromList(nodes []NodeListNode, query string, allowDefault bool) (string, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		if allowDefault {
			picked := PickDefaultNode(nodes)
			if picked != nil {
				return picked.NodeID, nil
			}
		}
		return "", fmt.Errorf("node required")
	}

	qNorm := normalizeNodeKey(q)
	var matches []NodeListNode
	for _, n := range nodes {
		if n.NodeID == q {
			matches = append(matches, n)
			continue
		}
		if n.RemoteIP != "" && n.RemoteIP == q {
			matches = append(matches, n)
			continue
		}
		if n.DisplayName != "" && normalizeNodeKey(n.DisplayName) == qNorm {
			matches = append(matches, n)
			continue
		}
		if len(q) >= 6 && strings.HasPrefix(n.NodeID, q) {
			matches = append(matches, n)
			continue
		}
	}

	if len(matches) == 1 {
		return matches[0].NodeID, nil
	}
	if len(matches) == 0 {
		var known []string
		for _, n := range nodes {
			label := n.DisplayName
			if label == "" {
				label = n.RemoteIP
			}
			if label == "" {
				label = n.NodeID
			}
			if label != "" {
				known = append(known, label)
			}
		}
		knownStr := ""
		if len(known) > 0 {
			knownStr = " (known: " + strings.Join(known, ", ") + ")"
		}
		return "", fmt.Errorf("unknown node: %s%s", q, knownStr)
	}
	// 多个匹配
	var labels []string
	for _, n := range matches {
		label := n.DisplayName
		if label == "" {
			label = n.RemoteIP
		}
		if label == "" {
			label = n.NodeID
		}
		labels = append(labels, label)
	}
	return "", fmt.Errorf("ambiguous node: %s (matches: %s)", q, strings.Join(labels, ", "))
}

// ---------- Gateway 节点加载 ----------

// NodeLoader 节点加载接口。
type NodeLoader interface {
	LoadNodes(ctx context.Context) ([]NodeListNode, error)
}

// ResolveNodeID 通过 gateway 加载节点列表并解析节点 ID。
// 对齐 TS: resolveNodeId()
func ResolveNodeID(ctx context.Context, loader NodeLoader, query string, allowDefault bool) (string, error) {
	if loader == nil {
		return "", fmt.Errorf("node loader not configured")
	}
	nodes, err := loader.LoadNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("load nodes: %w", err)
	}
	return ResolveNodeIDFromList(nodes, query, allowDefault)
}

// ListNodesViaLoader 通过 loader 加载节点列表。
// 对齐 TS: listNodes()
func ListNodesViaLoader(ctx context.Context, loader NodeLoader) ([]NodeListNode, error) {
	if loader == nil {
		return nil, fmt.Errorf("node loader not configured")
	}
	return loader.LoadNodes(ctx)
}

// ---------- 辅助 ----------

func containsStr(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}
