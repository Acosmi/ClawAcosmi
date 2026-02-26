package config

import "fmt"

// legacyConfigMigrations 所有迁移 (Part 1 + 2 + 3)
var legacyConfigMigrations = []LegacyConfigMigration{
	// ── Part 1 ──
	migBindingsProviderToChannel(),
	migBindingsAccountIDCase(),
	migSessionSendPolicyProvider(),
	migQueueByProviderToChannel(),
	migProvidersToChannels(),
	migRoutingAllowFrom(),
	migRoutingGroupChatRequireMention(),
	migGatewayToken(),
	migTelegramRequireMention(),
	// ── Part 2 ──
	migAgentModelConfigV2(),
	migRoutingAgentsV2(),
	migRoutingConfigV2(),
	// ── Part 3 ──
	migAuthClaudeCliOAuth(),
	migToolsBashToExec(),
	migMessagesTTSEnabled(),
	migAgentDefaultsV2(),
	migIdentityToAgentsList(),
}

// ── Part 1 迁移 ──

func migBindingsProviderToChannel() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "bindings.match.provider->channel", Describe: "Move bindings[].match.provider to channel",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			bindings, ok := raw["bindings"].([]interface{})
			if !ok {
				return
			}
			touched := false
			for _, item := range bindings {
				entry := getRecord(item)
				if entry == nil {
					continue
				}
				match := getRecord(entry["match"])
				if match == nil {
					continue
				}
				if ch, ok := match["channel"].(string); ok && ch != "" {
					continue
				}
				provider, ok := match["provider"].(string)
				if !ok || provider == "" {
					continue
				}
				match["channel"] = provider
				delete(match, "provider")
				touched = true
			}
			if touched {
				*changes = append(*changes, "Moved bindings[].match.provider → bindings[].match.channel.")
			}
		},
	}
}

func migBindingsAccountIDCase() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "bindings.match.accountID->accountId", Describe: "Fix accountID casing",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			bindings, ok := raw["bindings"].([]interface{})
			if !ok {
				return
			}
			touched := false
			for _, item := range bindings {
				entry := getRecord(item)
				if entry == nil {
					continue
				}
				match := getRecord(entry["match"])
				if match == nil {
					continue
				}
				if _, has := match["accountId"]; has {
					continue
				}
				accountID := match["accountID"]
				if accountID == nil {
					continue
				}
				match["accountId"] = accountID
				delete(match, "accountID")
				touched = true
			}
			if touched {
				*changes = append(*changes, "Moved bindings[].match.accountID → bindings[].match.accountId.")
			}
		},
	}
}

func migSessionSendPolicyProvider() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "session.sendPolicy.rules.match.provider->channel", Describe: "Fix sendPolicy rules provider",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			session := getRecord(raw["session"])
			if session == nil {
				return
			}
			sendPolicy := getRecord(session["sendPolicy"])
			if sendPolicy == nil {
				return
			}
			rules, ok := sendPolicy["rules"].([]interface{})
			if !ok {
				return
			}
			touched := false
			for _, item := range rules {
				rule := getRecord(item)
				if rule == nil {
					continue
				}
				match := getRecord(rule["match"])
				if match == nil {
					continue
				}
				if ch, ok := match["channel"].(string); ok && ch != "" {
					continue
				}
				provider, ok := match["provider"].(string)
				if !ok || provider == "" {
					continue
				}
				match["channel"] = provider
				delete(match, "provider")
				touched = true
			}
			if touched {
				*changes = append(*changes, "Moved session.sendPolicy.rules[].match.provider → match.channel.")
			}
		},
	}
}

func migQueueByProviderToChannel() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "messages.queue.byProvider->byChannel", Describe: "Rename queue byProvider",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			messages := getRecord(raw["messages"])
			if messages == nil {
				return
			}
			queue := getRecord(messages["queue"])
			if queue == nil {
				return
			}
			if _, has := queue["byProvider"]; !has {
				return
			}
			if _, has := queue["byChannel"]; !has {
				queue["byChannel"] = queue["byProvider"]
				*changes = append(*changes, "Moved messages.queue.byProvider → messages.queue.byChannel.")
			} else {
				*changes = append(*changes, "Removed messages.queue.byProvider (byChannel already set).")
			}
			delete(queue, "byProvider")
		},
	}
}

func migProvidersToChannels() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "providers->channels", Describe: "Move provider sections to channels.*",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			legacyKeys := []string{"whatsapp", "telegram", "discord", "slack", "signal", "imessage", "msteams"}
			var found []string
			for _, key := range legacyKeys {
				if isRecord(raw[key]) {
					found = append(found, key)
				}
			}
			if len(found) == 0 {
				return
			}
			channels := ensureRecord(raw, "channels")
			for _, key := range found {
				legacy := getRecord(raw[key])
				if legacy == nil {
					continue
				}
				ch := ensureRecord(channels, key)
				had := len(ch) > 0
				mergeMissing(ch, legacy)
				delete(raw, key)
				if had {
					*changes = append(*changes, fmt.Sprintf("Merged %s → channels.%s.", key, key))
				} else {
					*changes = append(*changes, fmt.Sprintf("Moved %s → channels.%s.", key, key))
				}
			}
		},
	}
}

func migRoutingAllowFrom() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "routing.allowFrom->channels.whatsapp.allowFrom", Describe: "Move routing.allowFrom",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			routing := getRecord(raw["routing"])
			if routing == nil {
				return
			}
			allowFrom, has := routing["allowFrom"]
			if !has {
				return
			}
			channels := getRecord(raw["channels"])
			whatsapp := getRecord(nil)
			if channels != nil {
				whatsapp = getRecord(channels["whatsapp"])
			}
			if whatsapp == nil {
				delete(routing, "allowFrom")
				if len(routing) == 0 {
					delete(raw, "routing")
				}
				*changes = append(*changes, "Removed routing.allowFrom (channels.whatsapp not configured).")
				return
			}
			if _, has := whatsapp["allowFrom"]; !has {
				whatsapp["allowFrom"] = allowFrom
				*changes = append(*changes, "Moved routing.allowFrom → channels.whatsapp.allowFrom.")
			} else {
				*changes = append(*changes, "Removed routing.allowFrom (channels.whatsapp.allowFrom already set).")
			}
			delete(routing, "allowFrom")
			if len(routing) == 0 {
				delete(raw, "routing")
			}
		},
	}
}

func migRoutingGroupChatRequireMention() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID:       "routing.groupChat.requireMention->groups.*.requireMention",
		Describe: "Move routing.groupChat.requireMention to channels groups",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			routing := getRecord(raw["routing"])
			if routing == nil {
				return
			}
			groupChat := getRecord(routing["groupChat"])
			if groupChat == nil {
				return
			}
			requireMention, has := groupChat["requireMention"]
			if !has {
				return
			}
			channels := ensureRecord(raw, "channels")
			applyTo := func(key string, requireExisting bool) {
				if requireExisting && !isRecord(channels[key]) {
					return
				}
				section := getRecord(channels[key])
				if section == nil {
					section = map[string]interface{}{}
				}
				groups := getRecord(section["groups"])
				if groups == nil {
					groups = map[string]interface{}{}
				}
				entry := getRecord(groups["*"])
				if entry == nil {
					entry = map[string]interface{}{}
				}
				if _, has := entry["requireMention"]; !has {
					entry["requireMention"] = requireMention
					groups["*"] = entry
					section["groups"] = groups
					channels[key] = section
					*changes = append(*changes, fmt.Sprintf(`Moved routing.groupChat.requireMention → channels.%s.groups."*".requireMention.`, key))
				} else {
					*changes = append(*changes, fmt.Sprintf(`Removed routing.groupChat.requireMention (channels.%s.groups."*" already set).`, key))
				}
			}
			applyTo("whatsapp", true)
			applyTo("telegram", false)
			applyTo("imessage", false)
			delete(groupChat, "requireMention")
			if len(groupChat) == 0 {
				delete(routing, "groupChat")
			}
			if len(routing) == 0 {
				delete(raw, "routing")
			}
			raw["channels"] = channels
		},
	}
}

func migGatewayToken() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "gateway.token->gateway.auth.token", Describe: "Move gateway.token",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			gateway := getRecord(raw["gateway"])
			if gateway == nil {
				return
			}
			token, has := gateway["token"]
			if !has {
				return
			}
			auth := getRecord(gateway["auth"])
			if auth == nil {
				auth = map[string]interface{}{}
			}
			if _, has := auth["token"]; !has {
				auth["token"] = token
				if _, has := auth["mode"]; !has {
					auth["mode"] = "token"
				}
				*changes = append(*changes, "Moved gateway.token → gateway.auth.token.")
			} else {
				*changes = append(*changes, "Removed gateway.token (gateway.auth.token already set).")
			}
			delete(gateway, "token")
			if len(auth) > 0 {
				gateway["auth"] = auth
			}
		},
	}
}

func migTelegramRequireMention() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID:       "telegram.requireMention->channels.telegram.groups.*.requireMention",
		Describe: "Move telegram.requireMention",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			channels := ensureRecord(raw, "channels")
			telegram := getRecord(channels["telegram"])
			if telegram == nil {
				return
			}
			requireMention, has := telegram["requireMention"]
			if !has {
				return
			}
			groups := getRecord(telegram["groups"])
			if groups == nil {
				groups = map[string]interface{}{}
			}
			entry := getRecord(groups["*"])
			if entry == nil {
				entry = map[string]interface{}{}
			}
			if _, has := entry["requireMention"]; !has {
				entry["requireMention"] = requireMention
				groups["*"] = entry
				telegram["groups"] = groups
				*changes = append(*changes, `Moved telegram.requireMention → channels.telegram.groups."*".requireMention.`)
			} else {
				*changes = append(*changes, `Removed telegram.requireMention (channels.telegram.groups."*" already set).`)
			}
			delete(telegram, "requireMention")
		},
	}
}
