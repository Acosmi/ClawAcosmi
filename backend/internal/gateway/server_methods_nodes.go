package gateway

// server_methods_nodes.go — node.* 方法处理器
// 对应 TS: src/gateway/server-methods/nodes.ts (538L)
//
// 方法列表 (11):
//   node.pair.request, node.pair.list, node.pair.approve, node.pair.reject, node.pair.verify,
//   node.list, node.describe, node.invoke, node.invoke.result, node.event, node.rename

import (
	"sort"
	"strings"
	"time"

	"github.com/openacosmi/claw-acismi/internal/infra"
)

// NodeHandlers 返回 node.* 方法映射。
func NodeHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"node.pair.request":  handleNodePairRequest,
		"node.pair.list":     handleNodePairList,
		"node.pair.approve":  handleNodePairApprove,
		"node.pair.reject":   handleNodePairReject,
		"node.pair.verify":   handleNodePairVerify,
		"node.list":          handleNodeList,
		"node.describe":      handleNodeDescribe,
		"node.invoke":        handleNodeInvoke,
		"node.invoke.result": handleNodeInvokeResult,
		"node.event":         handleNodeEvent,
		"node.rename":        handleNodeRename,
	}
}

// ---------- node.pair.request ----------

func handleNodePairRequest(ctx *MethodHandlerContext) {
	nodeID, _ := ctx.Params["nodeId"].(string)
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "node.pair.request requires nodeId"))
		return
	}

	req := infra.NodePairingPendingRequest{
		NodeID: nodeID,
	}
	if dn, ok := ctx.Params["displayName"].(string); ok {
		req.DisplayName = dn
	}
	if p, ok := ctx.Params["platform"].(string); ok {
		req.Platform = p
	}
	if v, ok := ctx.Params["version"].(string); ok {
		req.Version = v
	}
	if cv, ok := ctx.Params["coreVersion"].(string); ok {
		req.CoreVersion = cv
	}
	if uv, ok := ctx.Params["uiVersion"].(string); ok {
		req.UIVersion = uv
	}
	if df, ok := ctx.Params["deviceFamily"].(string); ok {
		req.DeviceFamily = df
	}
	if mi, ok := ctx.Params["modelIdentifier"].(string); ok {
		req.ModelIdentifier = mi
	}
	if hn, ok := ctx.Params["hostname"].(string); ok {
		req.Hostname = hn
	}
	if ri, ok := ctx.Params["remoteIp"].(string); ok {
		req.RemoteIP = ri
	}
	if caps, ok := ctx.Params["caps"].([]interface{}); ok {
		for _, c := range caps {
			if s, ok := c.(string); ok {
				req.Caps = append(req.Caps, s)
			}
		}
	}
	if cmds, ok := ctx.Params["commands"].([]interface{}); ok {
		for _, c := range cmds {
			if s, ok := c.(string); ok {
				req.Commands = append(req.Commands, s)
			}
		}
	}

	result, created := infra.RequestNodePairing(req)
	if result == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to create pairing request"))
		return
	}

	resp := map[string]interface{}{
		"status":    "pending",
		"requestId": result.RequestID,
		"created":   created,
	}

	// TS: 仅 created 时 broadcast "node.pair.requested"
	if created && ctx.Context.BroadcastFn != nil {
		ctx.Context.BroadcastFn("node.pair.requested", map[string]interface{}{
			"requestId":   result.RequestID,
			"nodeId":      result.NodeID,
			"displayName": result.DisplayName,
		}, nil)
	}

	ctx.Respond(true, resp, nil)
}

// ---------- node.pair.list ----------

func handleNodePairList(ctx *MethodHandlerContext) {
	state := infra.ListNodePairingStatus()
	if state == nil {
		ctx.Respond(true, map[string]interface{}{
			"pending": []interface{}{},
			"paired":  []interface{}{},
		}, nil)
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"pending": state.Pending,
		"paired":  redactPairedNodes(state.Paired),
	}, nil)
}

// ---------- node.pair.approve ----------

func handleNodePairApprove(ctx *MethodHandlerContext) {
	requestID, _ := ctx.Params["requestId"].(string)
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "node.pair.approve requires requestId"))
		return
	}

	node, err := infra.ApproveNodePairing(requestID)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "unknown requestId"))
		return
	}

	// TS: broadcast "node.pair.resolved" with decision="approved"
	if ctx.Context.BroadcastFn != nil {
		ctx.Context.BroadcastFn("node.pair.resolved", map[string]interface{}{
			"requestId": requestID,
			"nodeId":    node.NodeID,
			"decision":  "approved",
			"ts":        time.Now().UnixMilli(),
		}, nil)
	}

	ctx.Respond(true, map[string]interface{}{
		"nodeId": node.NodeID,
		"token":  node.Token,
	}, nil)
}

// ---------- node.pair.reject ----------

func handleNodePairReject(ctx *MethodHandlerContext) {
	requestID, _ := ctx.Params["requestId"].(string)
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "node.pair.reject requires requestId"))
		return
	}

	found := infra.RejectNodePairing(requestID)
	if !found {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "unknown requestId"))
		return
	}

	// TS: broadcast "node.pair.resolved" with decision="rejected"
	if ctx.Context.BroadcastFn != nil {
		ctx.Context.BroadcastFn("node.pair.resolved", map[string]interface{}{
			"requestId": requestID,
			"decision":  "rejected",
			"ts":        time.Now().UnixMilli(),
		}, nil)
	}

	ctx.Respond(true, map[string]interface{}{"ok": true}, nil)
}

// ---------- node.pair.verify ----------

func handleNodePairVerify(ctx *MethodHandlerContext) {
	nodeID, _ := ctx.Params["nodeId"].(string)
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "node.pair.verify requires nodeId"))
		return
	}

	token, _ := ctx.Params["token"].(string)
	token = strings.TrimSpace(token)
	if token == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "node.pair.verify requires token"))
		return
	}

	node, valid := infra.VerifyNodeToken(nodeID, token)
	result := map[string]interface{}{"valid": valid}
	if node != nil {
		result["node"] = node
	}
	ctx.Respond(true, result, nil)
}

// ---------- node.list ----------
// TS: 使用 listDevicePairing() 获取 paired 设备列表，过滤 role=node，
// 然后与 nodeRegistry.listConnected() 合并。

func handleNodeList(ctx *MethodHandlerContext) {
	baseDir := ctx.Context.PairingBaseDir

	// 获取 paired 设备中 role=node 的记录
	pairedByID := make(map[string]*PairedDevice)
	list, err := ListDevicePairing(baseDir)
	if err == nil && list != nil {
		for _, dev := range list.Paired {
			if isNodeEntry(dev) {
				pairedByID[dev.DeviceID] = dev
			}
		}
	}

	// 获取当前已连接节点
	connectedByID := make(map[string]*ConnectedNodeInfo)
	if ctx.Context.NodeRegistryGW != nil {
		for _, cn := range ctx.Context.NodeRegistryGW.ConnectedNodes() {
			cp := cn
			connectedByID[cn.NodeID] = &cp
		}
	}

	// 合并 nodeId 集合
	nodeIDs := make(map[string]bool)
	for id := range pairedByID {
		nodeIDs[id] = true
	}
	for id := range connectedByID {
		nodeIDs[id] = true
	}

	nodes := make([]map[string]interface{}, 0, len(nodeIDs))
	for nodeID := range nodeIDs {
		paired := pairedByID[nodeID]
		live := connectedByID[nodeID]

		// TS: caps/commands 优先 live 然后 paired
		caps := uniqueSortedStrings(coalesceStrings(live != nil, liveOrNilCaps(live), pairedOrNilCaps(paired)))
		commands := uniqueSortedStrings(coalesceStrings(live != nil, liveOrNilCommands(live), pairedOrNilCommands(paired)))

		entry := map[string]interface{}{
			"nodeId":          nodeID,
			"displayName":     nodeCoalesceStr(liveStr(live, "displayName"), pairedStr(paired, "displayName")),
			"platform":        nodeCoalesceStr(liveStr(live, "platform"), pairedStr(paired, "platform")),
			"version":         liveStr(live, "version"),
			"coreVersion":     liveStr(live, "coreVersion"),
			"uiVersion":       liveStr(live, "uiVersion"),
			"deviceFamily":    nodeCoalesceStr(liveStr(live, "deviceFamily"), pairedStr(paired, "deviceFamily")),
			"modelIdentifier": liveStr(live, "modelIdentifier"),
			"remoteIp":        nodeCoalesceStr(liveStr(live, "remoteIp"), pairedStr(paired, "remoteIp")),
			"caps":            caps,
			"commands":        commands,
			"paired":          paired != nil,
			"connected":       live != nil,
		}
		if live != nil {
			entry["pathEnv"] = live.PathEnv
			entry["permissions"] = live.Permissions
			entry["connectedAtMs"] = live.ConnectedAtMs
		}
		nodes = append(nodes, entry)
	}

	// TS 排序: connected first, then displayName asc, then nodeId
	sort.Slice(nodes, func(i, j int) bool {
		ci := nodes[i]["connected"].(bool)
		cj := nodes[j]["connected"].(bool)
		if ci != cj {
			return ci // connected first
		}
		ni := nodeListSortKey(nodes[i])
		nj := nodeListSortKey(nodes[j])
		if ni != nj {
			return ni < nj
		}
		return nodes[i]["nodeId"].(string) < nodes[j]["nodeId"].(string)
	})

	ctx.Respond(true, map[string]interface{}{
		"ts":    time.Now().UnixMilli(),
		"nodes": nodes,
	}, nil)
}

// ---------- node.describe ----------

func handleNodeDescribe(ctx *MethodHandlerContext) {
	nodeID, _ := ctx.Params["nodeId"].(string)
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "nodeId required"))
		return
	}

	baseDir := ctx.Context.PairingBaseDir

	// 查找 paired device with role=node
	var paired *PairedDevice
	list, err := ListDevicePairing(baseDir)
	if err == nil && list != nil {
		for _, dev := range list.Paired {
			if dev.DeviceID == nodeID && isNodeEntry(dev) {
				paired = dev
				break
			}
		}
	}

	// 查找 connected node
	var live *ConnectedNodeInfo
	if ctx.Context.NodeRegistryGW != nil {
		for _, cn := range ctx.Context.NodeRegistryGW.ConnectedNodes() {
			if cn.NodeID == nodeID {
				cp := cn
				live = &cp
				break
			}
		}
	}

	if paired == nil && live == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "unknown nodeId"))
		return
	}

	caps := uniqueSortedStrings(liveOrNilCaps(live))
	commands := uniqueSortedStrings(liveOrNilCommands(live))

	result := map[string]interface{}{
		"ts":              time.Now().UnixMilli(),
		"nodeId":          nodeID,
		"displayName":     nodeCoalesceStr(liveStr(live, "displayName"), pairedStr(paired, "displayName")),
		"platform":        nodeCoalesceStr(liveStr(live, "platform"), pairedStr(paired, "platform")),
		"version":         liveStr(live, "version"),
		"coreVersion":     liveStr(live, "coreVersion"),
		"uiVersion":       liveStr(live, "uiVersion"),
		"deviceFamily":    liveStr(live, "deviceFamily"),
		"modelIdentifier": liveStr(live, "modelIdentifier"),
		"remoteIp":        nodeCoalesceStr(liveStr(live, "remoteIp"), pairedStr(paired, "remoteIp")),
		"caps":            caps,
		"commands":        commands,
		"paired":          paired != nil,
		"connected":       live != nil,
	}
	if live != nil {
		result["pathEnv"] = live.PathEnv
		result["permissions"] = live.Permissions
		result["connectedAtMs"] = live.ConnectedAtMs
	}

	ctx.Respond(true, result, nil)
}

// ---------- node.invoke ----------
// TS: 检查节点在线 → 命令策略检查 → invoke

func handleNodeInvoke(ctx *MethodHandlerContext) {
	if ctx.Context.NodeRegistryGW == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "node registry not available"))
		return
	}

	nodeID, _ := ctx.Params["nodeId"].(string)
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "nodeId and command required"))
		return
	}

	command, _ := ctx.Params["command"].(string)
	command = strings.TrimSpace(command)
	if command == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "nodeId and command required"))
		return
	}

	// 查找已连接节点
	var nodeSession *ConnectedNodeInfo
	for _, cn := range ctx.Context.NodeRegistryGW.ConnectedNodes() {
		if cn.NodeID == nodeID {
			cp := cn
			nodeSession = &cp
			break
		}
	}
	if nodeSession == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "node not connected"))
		return
	}

	// FIND-9: 命令策略检查
	// TS: resolveNodeCommandAllowlist(cfg, nodeSession) → isNodeCommandAllowed
	policyInput := NodeCommandPolicyInput{
		Platform:     nodeSession.Platform,
		DeviceFamily: nodeSession.DeviceFamily,
	}
	// HEALTH-D3: 从 config 读取 gateway.nodes.allowCommands / denyCommands
	// 对应 TS: resolveNodeCommandAllowlist(cfg, nodeSession)
	if cfg := ctx.Context.Config; cfg != nil && cfg.Gateway != nil && cfg.Gateway.Nodes != nil {
		policyInput.AllowCommands = cfg.Gateway.Nodes.AllowCommands
		policyInput.DenyCommands = cfg.Gateway.Nodes.DenyCommands
	}

	allowlist := ResolveNodeCommandAllowlist(policyInput)
	allowed := IsNodeCommandAllowed(command, nodeSession.Commands, allowlist)
	if !allowed.OK {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "node command not allowed: "+allowed.Reason))
		return
	}

	params := ctx.Params["params"]

	result, err := ctx.Context.NodeRegistryGW.Invoke(nodeID, command, params)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "node invoke failed: "+err.Error()))
		return
	}

	ctx.Respond(true, result, nil)
}

// ---------- node.invoke.result ----------
// TS: normalizeNodeInvokeResultParams + callerNodeId 校验

func handleNodeInvokeResult(ctx *MethodHandlerContext) {
	if ctx.Context.NodeRegistryGW == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "node registry not available"))
		return
	}

	// FIND-10: normalizeNodeInvokeResultParams
	// TS: payloadJSON=null → 删除; payloadJSON非string → 移到payload; error=null → 删除
	params := make(map[string]interface{})
	for k, v := range ctx.Params {
		params[k] = v
	}

	// 归一化 payloadJSON
	if pj, exists := params["payloadJSON"]; exists {
		if pj == nil {
			delete(params, "payloadJSON")
		} else if _, isStr := pj.(string); !isStr {
			// payloadJSON 不是 string → 移到 payload
			if _, hasPayload := params["payload"]; !hasPayload {
				params["payload"] = pj
			}
			delete(params, "payloadJSON")
		}
	}
	// error=null → 删除
	if e, exists := params["error"]; exists && e == nil {
		delete(params, "error")
	}

	nodeID, _ := params["nodeId"].(string)
	requestID, _ := params["id"].(string)
	result := params

	if err := ctx.Context.NodeRegistryGW.HandleInvokeResult(nodeID, requestID, result); err != nil {
		// TS: late-arriving 结果返回成功 + ignored=true
		ctx.Respond(true, map[string]interface{}{"ok": true, "ignored": true}, nil)
		return
	}

	ctx.Respond(true, map[string]interface{}{"ok": true}, nil)
}

// ---------- node.event ----------

func handleNodeEvent(ctx *MethodHandlerContext) {
	if ctx.Context.NodeRegistryGW == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "node registry not available"))
		return
	}

	nodeID, _ := ctx.Params["nodeId"].(string)
	event, _ := ctx.Params["event"].(string)
	payload := ctx.Params["payload"]

	if err := ctx.Context.NodeRegistryGW.HandleNodeEvent(nodeID, event, payload); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "node event failed: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{"ok": true}, nil)
}

// ---------- node.rename ----------

func handleNodeRename(ctx *MethodHandlerContext) {
	nodeID, _ := ctx.Params["nodeId"].(string)
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "node.rename requires nodeId"))
		return
	}

	displayName, _ := ctx.Params["displayName"].(string)
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "displayName required"))
		return
	}

	found := infra.UpdatePairedNodeMetadata(nodeID, func(n *infra.NodePairingPairedNode) {
		n.DisplayName = displayName
	})

	if !found {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "unknown nodeId"))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"nodeId":      nodeID,
		"displayName": displayName,
	}, nil)
}

// ---------- 辅助函数 ----------

// isNodeEntry 检查 PairedDevice 是否为 "node" 角色。
// TS: isNodeEntry checks role==="node" || roles.includes("node")
func isNodeEntry(dev *PairedDevice) bool {
	if dev.Role == "node" {
		return true
	}
	for _, r := range dev.Roles {
		if r == "node" {
			return true
		}
	}
	return false
}

func redactPairedNodes(nodes []infra.NodePairingPairedNode) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, map[string]interface{}{
			"nodeId":      n.NodeID,
			"displayName": n.DisplayName,
			"platform":    n.Platform,
			"version":     n.Version,
			"createdAtMs": n.CreatedAtMs,
			"approvedAt":  n.ApprovedAtMs,
		})
	}
	return result
}

// uniqueSortedStrings 去重 + 排序字符串切片。
func uniqueSortedStrings(s []string) []string {
	if len(s) == 0 {
		return []string{}
	}
	seen := make(map[string]bool, len(s))
	var out []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func nodeCoalesceStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// coalesceStrings 优先返回 primary（若 usePrimary 且非空），否则返回 fallback。
func coalesceStrings(usePrimary bool, primary, fallback []string) []string {
	if usePrimary && primary != nil {
		return primary
	}
	if fallback != nil {
		return fallback
	}
	return nil
}

func liveStr(live *ConnectedNodeInfo, field string) string {
	if live == nil {
		return ""
	}
	switch field {
	case "displayName":
		return live.DisplayName
	case "platform":
		return live.Platform
	case "version":
		return live.Version
	case "coreVersion":
		return live.CoreVersion
	case "uiVersion":
		return live.UIVersion
	case "deviceFamily":
		return live.DeviceFamily
	case "modelIdentifier":
		return live.ModelIdentifier
	case "remoteIp":
		return live.RemoteIP
	}
	return ""
}

func pairedStr(paired *PairedDevice, field string) string {
	if paired == nil {
		return ""
	}
	switch field {
	case "displayName":
		return paired.DisplayName
	case "platform":
		return paired.Platform
	case "remoteIp":
		return paired.RemoteIP
	case "deviceFamily":
		return ""
	}
	return ""
}

func liveOrNilCaps(live *ConnectedNodeInfo) []string {
	if live == nil {
		return nil
	}
	return live.Caps
}

func liveOrNilCommands(live *ConnectedNodeInfo) []string {
	if live == nil {
		return nil
	}
	return live.Commands
}

func pairedOrNilCaps(_ *PairedDevice) []string {
	// TS node.list: paired node default caps=[] (no caps stored in device pairing)
	return nil
}

func pairedOrNilCommands(_ *PairedDevice) []string {
	return nil
}

func nodeListSortKey(node map[string]interface{}) string {
	if dn, ok := node["displayName"].(string); ok && dn != "" {
		return strings.ToLower(dn)
	}
	if id, ok := node["nodeId"].(string); ok {
		return strings.ToLower(id)
	}
	return ""
}
