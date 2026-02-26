package bridge

import "fmt"

// WhatsApp action 路由 — 继承自 src/agents/tools/whatsapp-actions.ts (41L)

// HandleWhatsAppAction WhatsApp action 分发路由
func HandleWhatsAppAction(params map[string]interface{}, actionGate ActionGate) (ToolResult, error) {
	action, err := ReadStringParam(params, "action", true)
	if err != nil {
		return ErrorResult(err), err
	}

	switch action {
	case "react":
		if !actionGate("reactions") {
			return ToolResult{}, fmt.Errorf("WhatsApp reactions are disabled")
		}
		chatJid, err := ReadStringParam(params, "chatJid", true)
		if err != nil {
			return ErrorResult(err), err
		}
		messageId, err := ReadStringParam(params, "messageId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		reaction := ReadReactionParams(params)
		participant, _ := ReadStringParam(params, "participant", false)
		accountId, _ := ReadStringParam(params, "accountId", false)
		fromMeRaw, _ := params["fromMe"].(bool)

		// 5D: 实际调用 sendReactionWhatsApp
		_ = chatJid
		_ = messageId
		_ = participant
		_ = accountId
		_ = fromMeRaw

		if !reaction.Remove && !reaction.IsEmpty {
			return OkResult(map[string]interface{}{"ok": true, "added": reaction.Emoji}), nil
		}
		return OkResult(map[string]interface{}{"ok": true, "removed": true}), nil

	default:
		return ToolResult{}, fmt.Errorf("unsupported WhatsApp action: %s", action)
	}
}
