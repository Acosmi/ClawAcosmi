package gateway

import "testing"

// TestWsCloseCodesRFC6455 验证 close code 值符合 RFC 6455 标准。
func TestWsCloseCodesRFC6455(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		wantCode int
	}{
		{"Normal", WsCloseNormal, 1000},
		{"GoingAway", WsCloseGoingAway, 1001},
		{"ProtocolError", WsCloseProtocolError, 1002},
		{"PolicyViolation", WsClosePolicyViolation, 1008},
		{"ServiceRestart", WsCloseServiceRestart, 1012},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.wantCode {
				t.Errorf("WsClose%s = %d, want %d", tt.name, tt.code, tt.wantCode)
			}
		})
	}
}

// TestWsCloseReasonConstants 验证所有关键 close reason 常量存在且非空。
func TestWsCloseReasonConstants(t *testing.T) {
	reasons := map[string]string{
		"PolicyViolation":     WsReasonPolicyViolation,
		"AuthFailed":          WsReasonAuthFailed,
		"ProtocolMismatch":    WsReasonProtocolMismatch,
		"NonceMismatch":       WsReasonNonceMismatch,
		"DeviceAuthFailed":    WsReasonDeviceAuthFailed,
		"PairingRequired":     WsReasonPairingRequired,
		"InvalidRole":         WsReasonInvalidRole,
		"DeviceIdentRequired": WsReasonDeviceIdentRequired,
		"DeviceIdentMismatch": WsReasonDeviceIdentMismatch,
		"DeviceSigExpired":    WsReasonDeviceSigExpired,
		"DeviceSigInvalid":    WsReasonDeviceSigInvalid,
		"DevicePubKeyInvalid": WsReasonDevicePubKeyInvalid,
		"DeviceNonceRequired": WsReasonDeviceNonceRequired,
		"SlowConsumer":        WsReasonSlowConsumer,
		"ServiceRestart":      WsReasonServiceRestart,
		"ConnectFailed":       WsReasonConnectFailed,
	}

	for name, reason := range reasons {
		if reason == "" {
			t.Errorf("WsReason%s is empty", name)
		}
	}
}

// TestWsCloseCodeRange 验证 close code 在 RFC 6455 允许范围内。
func TestWsCloseCodeRange(t *testing.T) {
	codes := []struct {
		name string
		code int
	}{
		{"Normal", WsCloseNormal},
		{"GoingAway", WsCloseGoingAway},
		{"ProtocolError", WsCloseProtocolError},
		{"PolicyViolation", WsClosePolicyViolation},
		{"ServiceRestart", WsCloseServiceRestart},
	}

	for _, c := range codes {
		t.Run(c.name, func(t *testing.T) {
			// RFC 6455: 1000-2999 为 IANA 注册范围
			if c.code < 1000 || c.code > 2999 {
				t.Errorf("WsClose%s = %d, outside RFC 6455 registered range [1000, 2999]", c.name, c.code)
			}
		})
	}
}
