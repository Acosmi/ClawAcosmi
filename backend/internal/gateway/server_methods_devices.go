package gateway

// server_methods_devices.go — device.* 方法处理器
// 对应 TS: src/gateway/server-methods/devices.ts (191L)
//
// 方法列表 (5):
//   device.pair.list, device.pair.approve, device.pair.reject,
//   device.token.rotate, device.token.revoke

import (
	"strings"
)

// DeviceHandlers 返回 device.* 方法映射。
func DeviceHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"device.pair.list":    handleDevicePairList,
		"device.pair.approve": handleDevicePairApprove,
		"device.pair.reject":  handleDevicePairReject,
		"device.token.rotate": handleDeviceTokenRotate,
		"device.token.revoke": handleDeviceTokenRevoke,
	}
}

// ---------- device.pair.list ----------

func handleDevicePairList(ctx *MethodHandlerContext) {
	baseDir := ctx.Context.PairingBaseDir
	if baseDir == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "pairing base dir not configured"))
		return
	}

	list, err := ListDevicePairing(baseDir)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to list device pairing: "+err.Error()))
		return
	}

	// Redact tokens from paired devices
	redacted := make([]map[string]interface{}, 0, len(list.Paired))
	for _, d := range list.Paired {
		entry := map[string]interface{}{
			"deviceId":     d.DeviceID,
			"publicKey":    d.PublicKey,
			"displayName":  d.DisplayName,
			"role":         d.Role,
			"scopes":       d.Scopes,
			"createdAtMs":  d.CreatedAtMs,
			"approvedAtMs": d.ApprovedAtMs,
		}
		// Token summaries (redact actual token values)
		if d.Tokens != nil {
			tokenSummaries := make(map[string]interface{})
			for k, t := range d.Tokens {
				tokenSummaries[k] = map[string]interface{}{
					"role":         t.Role,
					"scopes":       t.Scopes,
					"createdAtMs":  t.CreatedAtMs,
					"rotatedAtMs":  t.RotatedAtMs,
					"revokedAtMs":  t.RevokedAtMs,
					"lastUsedAtMs": t.LastUsedAtMs,
				}
			}
			entry["tokens"] = tokenSummaries
		}
		redacted = append(redacted, entry)
	}

	ctx.Respond(true, map[string]interface{}{
		"pending": list.Pending,
		"paired":  redacted,
	}, nil)
}

// ---------- device.pair.approve ----------

func handleDevicePairApprove(ctx *MethodHandlerContext) {
	baseDir := ctx.Context.PairingBaseDir
	if baseDir == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "pairing base dir not configured"))
		return
	}

	requestID, _ := ctx.Params["requestId"].(string)
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "device.pair.approve requires requestId"))
		return
	}

	result, err := ApproveDevicePairing(requestID, baseDir)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "pairing approval failed: "+err.Error()))
		return
	}
	if result == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "pairing request not found"))
		return
	}

	if ctx.Context.BroadcastFn != nil {
		ctx.Context.BroadcastFn("device.pair.approved", map[string]interface{}{
			"deviceId":    result.Device.DeviceID,
			"displayName": result.Device.DisplayName,
		}, nil)
	}

	ctx.Respond(true, map[string]interface{}{
		"ok":       true,
		"deviceId": result.Device.DeviceID,
	}, nil)
}

// ---------- device.pair.reject ----------

func handleDevicePairReject(ctx *MethodHandlerContext) {
	baseDir := ctx.Context.PairingBaseDir
	if baseDir == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "pairing base dir not configured"))
		return
	}

	requestID, _ := ctx.Params["requestId"].(string)
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "device.pair.reject requires requestId"))
		return
	}

	result, err := RejectDevicePairing(requestID, baseDir)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "pairing rejection failed: "+err.Error()))
		return
	}
	if result == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "pairing request not found"))
		return
	}

	if ctx.Context.BroadcastFn != nil {
		ctx.Context.BroadcastFn("device.pair.rejected", map[string]interface{}{
			"requestId": requestID,
			"deviceId":  result.DeviceID,
		}, nil)
	}

	ctx.Respond(true, map[string]interface{}{"ok": true}, nil)
}

// ---------- device.token.rotate ----------

func handleDeviceTokenRotate(ctx *MethodHandlerContext) {
	baseDir := ctx.Context.PairingBaseDir
	if baseDir == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "pairing base dir not configured"))
		return
	}

	deviceID, _ := ctx.Params["deviceId"].(string)
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "device.token.rotate requires deviceId"))
		return
	}

	role, _ := ctx.Params["role"].(string)
	if role == "" {
		role = "operator"
	}

	// 提取可选 scopes
	var scopes []string
	if scopesRaw, ok := ctx.Params["scopes"].([]interface{}); ok {
		for _, s := range scopesRaw {
			if str, ok := s.(string); ok {
				scopes = append(scopes, str)
			}
		}
	}

	token, err := RotateDeviceToken(deviceID, role, scopes, baseDir)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "token rotation failed: "+err.Error()))
		return
	}
	if token == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "device not found or role invalid"))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"ok":       true,
		"deviceId": deviceID,
		"token":    token.Token,
		"role":     token.Role,
	}, nil)
}

// ---------- device.token.revoke ----------

func handleDeviceTokenRevoke(ctx *MethodHandlerContext) {
	baseDir := ctx.Context.PairingBaseDir
	if baseDir == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "pairing base dir not configured"))
		return
	}

	deviceID, _ := ctx.Params["deviceId"].(string)
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "device.token.revoke requires deviceId"))
		return
	}

	role, _ := ctx.Params["role"].(string)
	if role == "" {
		role = "operator"
	}

	token, err := RevokeDeviceToken(deviceID, role, baseDir)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "token revocation failed: "+err.Error()))
		return
	}
	if token == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "device or token not found"))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"ok":       true,
		"deviceId": deviceID,
	}, nil)
}
