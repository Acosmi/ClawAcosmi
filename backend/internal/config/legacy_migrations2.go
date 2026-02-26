package config

import "fmt"

// ── Part 2 迁移 ──

func migAgentModelConfigV2() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "agent.model-config-v2", Describe: "Migrate legacy model/allowedModels/aliases/fallbacks",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			agentRoot := getRecord(raw["agent"])
			defaults := getRecord(getRecord(getRecord(raw["agents"]))["defaults"]) // agents.defaults
			agent := agentRoot
			if agent == nil {
				agent = defaults
			}
			if agent == nil {
				return
			}
			label := "agent"
			if agentRoot == nil {
				label = "agents.defaults"
			}

			legacyModel, _ := agent["model"].(string)
			legacyImageModel, _ := agent["imageModel"].(string)
			legacyAllowed := toStringSlice(agent["allowedModels"])
			legacyModelFB := toStringSlice(agent["modelFallbacks"])
			legacyImageModelFB := toStringSlice(agent["imageModelFallbacks"])
			legacyAliases := getRecord(agent["modelAliases"])
			if legacyAliases == nil {
				legacyAliases = map[string]interface{}{}
			}

			hasLegacy := legacyModel != "" || legacyImageModel != "" ||
				len(legacyAllowed) > 0 || len(legacyModelFB) > 0 ||
				len(legacyImageModelFB) > 0 || len(legacyAliases) > 0
			if !hasLegacy {
				return
			}

			models := getRecord(agent["models"])
			if models == nil {
				models = map[string]interface{}{}
			}

			ensureModel := func(key string) {
				key2 := key
				if key2 == "" {
					return
				}
				if models[key2] == nil {
					models[key2] = map[string]interface{}{}
				}
			}
			ensureModel(legacyModel)
			ensureModel(legacyImageModel)
			for _, k := range legacyAllowed {
				ensureModel(k)
			}
			for _, k := range legacyModelFB {
				ensureModel(k)
			}
			for _, k := range legacyImageModelFB {
				ensureModel(k)
			}
			for _, targetRaw := range legacyAliases {
				if t, ok := targetRaw.(string); ok {
					ensureModel(t)
				}
			}

			for alias, targetRaw := range legacyAliases {
				target, ok := targetRaw.(string)
				if !ok || target == "" {
					continue
				}
				entry := getRecord(models[target])
				if entry == nil {
					entry = map[string]interface{}{}
				}
				if _, has := entry["alias"]; !has {
					entry["alias"] = alias
					models[target] = entry
				}
			}

			// model obj
			currentModel := getRecord(agent["model"])
			if currentModel != nil {
				if currentModel["primary"] == nil && legacyModel != "" {
					currentModel["primary"] = legacyModel
				}
				if len(legacyModelFB) > 0 {
					fb, _ := currentModel["fallbacks"].([]interface{})
					if len(fb) == 0 {
						currentModel["fallbacks"] = toInterfaceSlice(legacyModelFB)
					}
				}
				agent["model"] = currentModel
			} else if legacyModel != "" || len(legacyModelFB) > 0 {
				agent["model"] = map[string]interface{}{
					"primary": legacyModel, "fallbacks": toInterfaceSlice(legacyModelFB),
				}
			}

			// imageModel obj
			currentImageModel := getRecord(agent["imageModel"])
			if currentImageModel != nil {
				if currentImageModel["primary"] == nil && legacyImageModel != "" {
					currentImageModel["primary"] = legacyImageModel
				}
				if len(legacyImageModelFB) > 0 {
					fb, _ := currentImageModel["fallbacks"].([]interface{})
					if len(fb) == 0 {
						currentImageModel["fallbacks"] = toInterfaceSlice(legacyImageModelFB)
					}
				}
				agent["imageModel"] = currentImageModel
			} else if legacyImageModel != "" || len(legacyImageModelFB) > 0 {
				agent["imageModel"] = map[string]interface{}{
					"primary": legacyImageModel, "fallbacks": toInterfaceSlice(legacyImageModelFB),
				}
			}

			agent["models"] = models
			if legacyModel != "" {
				*changes = append(*changes, fmt.Sprintf("Migrated %s.model string → %s.model.primary.", label, label))
			}
			if len(legacyModelFB) > 0 {
				*changes = append(*changes, fmt.Sprintf("Migrated %s.modelFallbacks → %s.model.fallbacks.", label, label))
			}
			if legacyImageModel != "" {
				*changes = append(*changes, fmt.Sprintf("Migrated %s.imageModel string → %s.imageModel.primary.", label, label))
			}
			if len(legacyImageModelFB) > 0 {
				*changes = append(*changes, fmt.Sprintf("Migrated %s.imageModelFallbacks → %s.imageModel.fallbacks.", label, label))
			}
			if len(legacyAllowed) > 0 {
				*changes = append(*changes, fmt.Sprintf("Migrated %s.allowedModels → %s.models.", label, label))
			}
			if len(legacyAliases) > 0 {
				*changes = append(*changes, fmt.Sprintf("Migrated %s.modelAliases → %s.models.*.alias.", label, label))
			}
			delete(agent, "allowedModels")
			delete(agent, "modelAliases")
			delete(agent, "modelFallbacks")
			delete(agent, "imageModelFallbacks")
		},
	}
}

func migRoutingAgentsV2() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "routing.agents-v2", Describe: "Move routing.agents/defaultAgentId to agents.list",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			routing := getRecord(raw["routing"])
			if routing == nil {
				return
			}
			routingAgents := getRecord(routing["agents"])
			agents := ensureRecord(raw, "agents")
			list := getAgentsList(agents)
			if list == nil {
				list = []interface{}{}
			}

			if routingAgents != nil {
				for rawId, entryRaw := range routingAgents {
					agentId := fmt.Sprintf("%v", rawId)
					entry := getRecord(entryRaw)
					if entry == nil {
						continue
					}
					target := ensureAgentEntry(&list, agentId)
					entryCopy := deepCloneMap(entry)

					if mp, has := entryCopy["mentionPatterns"]; has {
						gc := ensureRecord(target, "groupChat")
						if gc["mentionPatterns"] == nil {
							gc["mentionPatterns"] = mp
							*changes = append(*changes, fmt.Sprintf("Moved routing.agents.%s.mentionPatterns → agents.list.", agentId))
						}
						delete(entryCopy, "mentionPatterns")
					}

					if legacyGC := getRecord(entryCopy["groupChat"]); legacyGC != nil {
						gc := ensureRecord(target, "groupChat")
						mergeMissing(gc, legacyGC)
						delete(entryCopy, "groupChat")
					}

					if legacySandbox := getRecord(entryCopy["sandbox"]); legacySandbox != nil {
						if sandboxTools := getRecord(legacySandbox["tools"]); sandboxTools != nil {
							tools := ensureRecord(target, "tools")
							sandbox := ensureRecord(tools, "sandbox")
							toolPolicy := ensureRecord(sandbox, "tools")
							mergeMissing(toolPolicy, sandboxTools)
							delete(legacySandbox, "tools")
						}
						entryCopy["sandbox"] = legacySandbox
					}
					mergeMissing(target, entryCopy)
				}
				delete(routing, "agents")
				*changes = append(*changes, "Moved routing.agents → agents.list.")
			}

			if defaultId, ok := routing["defaultAgentId"].(string); ok && defaultId != "" {
				hasDefault := false
				for _, item := range list {
					e := getRecord(item)
					if e != nil && e["default"] == true {
						hasDefault = true
						break
					}
				}
				if !hasDefault {
					entry := ensureAgentEntry(&list, defaultId)
					entry["default"] = true
					*changes = append(*changes, fmt.Sprintf("Moved routing.defaultAgentId → agents.list (id %q).default.", defaultId))
				} else {
					*changes = append(*changes, "Removed routing.defaultAgentId (agents.list default already set).")
				}
				delete(routing, "defaultAgentId")
			}

			if len(list) > 0 {
				agents["list"] = list
			}
			if len(routing) == 0 {
				delete(raw, "routing")
			}
		},
	}
}

func migRoutingConfigV2() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "routing.config-v2", Describe: "Move routing bindings/groupChat/queue/agentToAgent/transcribeAudio",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			routing := getRecord(raw["routing"])
			if routing == nil {
				return
			}

			// bindings
			if routing["bindings"] != nil {
				if raw["bindings"] == nil {
					raw["bindings"] = routing["bindings"]
					*changes = append(*changes, "Moved routing.bindings → bindings.")
				} else {
					*changes = append(*changes, "Removed routing.bindings (bindings already set).")
				}
				delete(routing, "bindings")
			}

			// agentToAgent
			if routing["agentToAgent"] != nil {
				tools := ensureRecord(raw, "tools")
				if tools["agentToAgent"] == nil {
					tools["agentToAgent"] = routing["agentToAgent"]
					*changes = append(*changes, "Moved routing.agentToAgent → tools.agentToAgent.")
				} else {
					*changes = append(*changes, "Removed routing.agentToAgent (tools.agentToAgent already set).")
				}
				delete(routing, "agentToAgent")
			}

			// queue
			if routing["queue"] != nil {
				messages := ensureRecord(raw, "messages")
				if messages["queue"] == nil {
					messages["queue"] = routing["queue"]
					*changes = append(*changes, "Moved routing.queue → messages.queue.")
				} else {
					*changes = append(*changes, "Removed routing.queue (messages.queue already set).")
				}
				delete(routing, "queue")
			}

			// groupChat
			if gc := getRecord(routing["groupChat"]); gc != nil {
				if hl := gc["historyLimit"]; hl != nil {
					messages := ensureRecord(raw, "messages")
					mgc := ensureRecord(messages, "groupChat")
					if mgc["historyLimit"] == nil {
						mgc["historyLimit"] = hl
						*changes = append(*changes, "Moved routing.groupChat.historyLimit → messages.groupChat.historyLimit.")
					}
					delete(gc, "historyLimit")
				}
				if mp := gc["mentionPatterns"]; mp != nil {
					messages := ensureRecord(raw, "messages")
					mgc := ensureRecord(messages, "groupChat")
					if mgc["mentionPatterns"] == nil {
						mgc["mentionPatterns"] = mp
						*changes = append(*changes, "Moved routing.groupChat.mentionPatterns → messages.groupChat.mentionPatterns.")
					}
					delete(gc, "mentionPatterns")
				}
				if len(gc) == 0 {
					delete(routing, "groupChat")
				}
			}

			// transcribeAudio
			if routing["transcribeAudio"] != nil {
				mapped := mapLegacyAudioTranscription(routing["transcribeAudio"])
				if mapped != nil {
					tools := ensureRecord(raw, "tools")
					media := ensureRecord(tools, "media")
					mediaAudio := ensureRecord(media, "audio")
					models, _ := mediaAudio["models"].([]interface{})
					if len(models) == 0 {
						mediaAudio["enabled"] = true
						mediaAudio["models"] = []interface{}{mapped}
						*changes = append(*changes, "Moved routing.transcribeAudio → tools.media.audio.models.")
					}
				}
				delete(routing, "transcribeAudio")
			}

			// audio.transcription
			if audio := getRecord(raw["audio"]); audio != nil {
				if audio["transcription"] != nil {
					mapped := mapLegacyAudioTranscription(audio["transcription"])
					if mapped != nil {
						tools := ensureRecord(raw, "tools")
						media := ensureRecord(tools, "media")
						mediaAudio := ensureRecord(media, "audio")
						models, _ := mediaAudio["models"].([]interface{})
						if len(models) == 0 {
							mediaAudio["enabled"] = true
							mediaAudio["models"] = []interface{}{mapped}
							*changes = append(*changes, "Moved audio.transcription → tools.media.audio.models.")
						}
					}
					delete(audio, "transcription")
					if len(audio) == 0 {
						delete(raw, "audio")
					}
				}
			}

			if len(routing) == 0 {
				delete(raw, "routing")
			}
		},
	}
}

// ── Part 3 迁移 ──

func migAuthClaudeCliOAuth() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "auth.anthropic-claude-cli-mode-oauth", Describe: "Switch claude-cli auth to oauth",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			auth := getRecord(raw["auth"])
			if auth == nil {
				return
			}
			profiles := getRecord(auth["profiles"])
			if profiles == nil {
				return
			}
			claudeCli := getRecord(profiles["anthropic:claude-cli"])
			if claudeCli == nil {
				return
			}
			if claudeCli["mode"] != "token" {
				return
			}
			claudeCli["mode"] = "oauth"
			*changes = append(*changes, `Updated auth.profiles["anthropic:claude-cli"].mode → "oauth".`)
		},
	}
}

func migToolsBashToExec() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "tools.bash->tools.exec", Describe: "Move tools.bash to tools.exec",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			tools := ensureRecord(raw, "tools")
			bash := getRecord(tools["bash"])
			if bash == nil {
				return
			}
			if tools["exec"] == nil {
				tools["exec"] = bash
				*changes = append(*changes, "Moved tools.bash → tools.exec.")
			} else {
				*changes = append(*changes, "Removed tools.bash (tools.exec already set).")
			}
			delete(tools, "bash")
		},
	}
}

func migMessagesTTSEnabled() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "messages.tts.enabled->auto", Describe: "Move messages.tts.enabled to auto",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			messages := getRecord(raw["messages"])
			if messages == nil {
				return
			}
			tts := getRecord(messages["tts"])
			if tts == nil {
				return
			}
			if tts["auto"] != nil {
				if _, has := tts["enabled"]; has {
					delete(tts, "enabled")
					*changes = append(*changes, "Removed messages.tts.enabled (messages.tts.auto already set).")
				}
				return
			}
			enabled, ok := tts["enabled"].(bool)
			if !ok {
				return
			}
			if enabled {
				tts["auto"] = "always"
			} else {
				tts["auto"] = "off"
			}
			delete(tts, "enabled")
			*changes = append(*changes, fmt.Sprintf("Moved messages.tts.enabled → messages.tts.auto (%v).", tts["auto"]))
		},
	}
}

func migAgentDefaultsV2() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "agent.defaults-v2", Describe: "Move agent config to agents.defaults and tools",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			agent := getRecord(raw["agent"])
			if agent == nil {
				return
			}
			agents := ensureRecord(raw, "agents")
			defaults := getRecord(agents["defaults"])
			if defaults == nil {
				defaults = map[string]interface{}{}
			}
			tools := ensureRecord(raw, "tools")

			// agent.tools.allow/deny → tools.allow/deny
			if agentTools := getRecord(agent["tools"]); agentTools != nil {
				if tools["allow"] == nil && agentTools["allow"] != nil {
					tools["allow"] = agentTools["allow"]
					*changes = append(*changes, "Moved agent.tools.allow → tools.allow.")
				}
				if tools["deny"] == nil && agentTools["deny"] != nil {
					tools["deny"] = agentTools["deny"]
					*changes = append(*changes, "Moved agent.tools.deny → tools.deny.")
				}
			}

			// agent.elevated → tools.elevated
			if elevated := getRecord(agent["elevated"]); elevated != nil {
				if tools["elevated"] == nil {
					tools["elevated"] = elevated
					*changes = append(*changes, "Moved agent.elevated → tools.elevated.")
				}
			}

			// agent.bash → tools.exec
			if bash := getRecord(agent["bash"]); bash != nil {
				if tools["exec"] == nil {
					tools["exec"] = bash
					*changes = append(*changes, "Moved agent.bash → tools.exec.")
				}
			}

			// agent.sandbox.tools → tools.sandbox.tools
			if sandbox := getRecord(agent["sandbox"]); sandbox != nil {
				if sandboxTools := getRecord(sandbox["tools"]); sandboxTools != nil {
					ts := ensureRecord(tools, "sandbox")
					tp := ensureRecord(ts, "tools")
					mergeMissing(tp, sandboxTools)
					delete(sandbox, "tools")
					*changes = append(*changes, "Moved agent.sandbox.tools → tools.sandbox.tools.")
				}
			}

			// agent.subagents.tools → tools.subagents.tools
			if subagents := getRecord(agent["subagents"]); subagents != nil {
				if subTools := getRecord(subagents["tools"]); subTools != nil {
					ts := ensureRecord(tools, "subagents")
					tp := ensureRecord(ts, "tools")
					mergeMissing(tp, subTools)
					delete(subagents, "tools")
					*changes = append(*changes, "Moved agent.subagents.tools → tools.subagents.tools.")
				}
			}

			agentCopy := deepCloneMap(agent)
			delete(agentCopy, "tools")
			delete(agentCopy, "elevated")
			delete(agentCopy, "bash")
			if s := getRecord(agentCopy["sandbox"]); s != nil {
				delete(s, "tools")
			}
			if s := getRecord(agentCopy["subagents"]); s != nil {
				delete(s, "tools")
			}

			mergeMissing(defaults, agentCopy)
			agents["defaults"] = defaults
			raw["agents"] = agents
			delete(raw, "agent")
			*changes = append(*changes, "Moved agent → agents.defaults.")
		},
	}
}

func migIdentityToAgentsList() LegacyConfigMigration {
	return LegacyConfigMigration{
		ID: "identity->agents.list", Describe: "Move identity to agents.list[].identity",
		Apply: func(raw map[string]interface{}, changes *[]string) {
			identity := getRecord(raw["identity"])
			if identity == nil {
				return
			}
			agents := ensureRecord(raw, "agents")
			list := getAgentsList(agents)
			if list == nil {
				list = []interface{}{}
			}
			defaultId := resolveDefaultAgentIdFromRaw(raw)
			entry := ensureAgentEntry(&list, defaultId)
			if entry["identity"] == nil {
				entry["identity"] = identity
				*changes = append(*changes, fmt.Sprintf("Moved identity → agents.list (id %q).identity.", defaultId))
			} else {
				*changes = append(*changes, "Removed identity (agents.list identity already set).")
			}
			agents["list"] = list
			raw["agents"] = agents
			delete(raw, "identity")
		},
	}
}

// ── 辅助函数 ──

func toStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		result = append(result, fmt.Sprintf("%v", item))
	}
	return result
}

func toInterfaceSlice(s []string) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}
