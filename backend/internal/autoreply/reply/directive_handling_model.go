package reply

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/agents/models"
	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
	"github.com/Acosmi/ClawAcosmi/pkg/utils"
)

// TS 对照: auto-reply/reply/directive-handling.model.ts (403L)
// 模型指令信息展示 (status/summary/list) + 模型选择解析。

// ---------- BuildModelPickerCatalog ----------

// BuildModelPickerCatalogParams buildModelPickerCatalog 参数。
type BuildModelPickerCatalogParams struct {
	Cfg                 *types.OpenAcosmiConfig
	DefaultProvider     string
	DefaultModel        string
	AliasIndex          models.ModelAliasIndex
	AllowedModelCatalog []ModelPickerCatalogEntry
}

// BuildModelPickerCatalog 从配置、allowlist、catalog 合并模型列表。
// TS 对照: directive-handling.model.ts buildModelPickerCatalog (L27-167)
func BuildModelPickerCatalog(params BuildModelPickerCatalogParams) []ModelPickerCatalogEntry {
	resolvedDefault := models.ResolveConfiguredModelRef(
		params.Cfg, params.DefaultProvider, params.DefaultModel,
	)

	// ---------- 内部：从配置提取已配置的模型 ----------
	buildConfiguredCatalog := func() []ModelPickerCatalogEntry {
		var out []ModelPickerCatalogEntry
		keys := make(map[string]bool)

		pushRef := func(provider, id, name string) {
			provider = models.NormalizeProviderId(provider)
			id = strings.TrimSpace(id)
			if provider == "" || id == "" {
				return
			}
			key := models.ModelKey(provider, id)
			if keys[key] {
				return
			}
			keys[key] = true
			if name == "" {
				name = id
			}
			out = append(out, ModelPickerCatalogEntry{Provider: provider, ID: id, Name: name})
		}

		pushRaw := func(raw string) {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				return
			}
			ref := models.ResolveModelRefFromString(raw, params.DefaultProvider, &params.AliasIndex)
			if ref == nil {
				return
			}
			pushRef(ref.Provider, ref.Model, "")
		}

		// 1. 配置的默认模型
		pushRef(resolvedDefault.Provider, resolvedDefault.Model, "")

		// 2. 模型 fallbacks
		if params.Cfg != nil && params.Cfg.Agents != nil && params.Cfg.Agents.Defaults != nil {
			defaults := params.Cfg.Agents.Defaults
			if defaults.Model != nil && defaults.Model.Fallbacks != nil {
				for _, fb := range *defaults.Model.Fallbacks {
					pushRaw(fb)
				}
			}

			// 3. 图像模型
			if defaults.ImageModel != nil {
				pushRaw(defaults.ImageModel.Primary)
				if defaults.ImageModel.Fallbacks != nil {
					for _, fb := range *defaults.ImageModel.Fallbacks {
						pushRaw(fb)
					}
				}
			}

			// 4. models allowlist 中的 keys
			for raw := range defaults.Models {
				pushRaw(raw)
			}
		}

		return out
	}

	// ---------- 合并去重 ----------
	keys := make(map[string]bool)
	var out []ModelPickerCatalogEntry

	push := func(entry ModelPickerCatalogEntry) {
		provider := models.NormalizeProviderId(entry.Provider)
		id := strings.TrimSpace(entry.ID)
		if provider == "" || id == "" {
			return
		}
		key := models.ModelKey(provider, id)
		if keys[key] {
			return
		}
		keys[key] = true
		out = append(out, ModelPickerCatalogEntry{Provider: provider, ID: id, Name: entry.Name})
	}

	hasAllowlist := false
	if params.Cfg != nil && params.Cfg.Agents != nil && params.Cfg.Agents.Defaults != nil {
		hasAllowlist = len(params.Cfg.Agents.Defaults.Models) > 0
	}

	if !hasAllowlist {
		// 无 allowlist: catalog 先, configured 后
		for _, entry := range params.AllowedModelCatalog {
			push(entry)
		}
		for _, entry := range buildConfiguredCatalog() {
			push(entry)
		}
		return out
	}

	// 有 allowlist: catalog + allowlist keys + 确保 default 存在
	for _, entry := range params.AllowedModelCatalog {
		push(entry)
	}

	if params.Cfg.Agents != nil && params.Cfg.Agents.Defaults != nil {
		for raw := range params.Cfg.Agents.Defaults.Models {
			ref := models.ResolveModelRefFromString(
				raw, params.DefaultProvider, &params.AliasIndex,
			)
			if ref == nil {
				continue
			}
			push(ModelPickerCatalogEntry{
				Provider: ref.Provider,
				ID:       ref.Model,
				Name:     ref.Model,
			})
		}
	}

	if resolvedDefault.Model != "" {
		push(ModelPickerCatalogEntry{
			Provider: resolvedDefault.Provider,
			ID:       resolvedDefault.Model,
			Name:     resolvedDefault.Model,
		})
	}

	return out
}

// ---------- MaybeHandleModelDirectiveInfo ----------

// ModelDirectiveInfoParams maybeHandleModelDirectiveInfo 参数。
// TS 对照: directive-handling.model.ts maybeHandleModelDirectiveInfo params (L169-182)
type ModelDirectiveInfoParams struct {
	Directives          InlineDirectives
	Cfg                 *types.OpenAcosmiConfig
	AgentDir            string
	ActiveAgentID       string
	Provider            string
	Model               string
	DefaultProvider     string
	DefaultModel        string
	AliasIndex          models.ModelAliasIndex
	AllowedModelCatalog []ModelPickerCatalogEntry
	ResetModelOverride  bool
	Surface             string
	// AuthResolver 异步解析 auth label。
	// 若 nil，使用默认占位 "unknown"。
	AuthResolver func(provider string) AuthLabel
}

// MaybeHandleModelDirectiveInfo 处理 /model（summary）、/model status、/model list。
// TS 对照: directive-handling.model.ts maybeHandleModelDirectiveInfo (L169-312)
// 返回 nil 表示不处理（非 model 信息查询类指令）。
func MaybeHandleModelDirectiveInfo(params ModelDirectiveInfoParams) *autoreply.ReplyPayload {
	if !params.Directives.HasModelDirective {
		return nil
	}

	rawDirective := strings.TrimSpace(params.Directives.RawModelDirective)
	directive := strings.ToLower(rawDirective)
	wantsStatus := directive == "status"
	wantsSummary := rawDirective == ""
	wantsLegacyList := directive == "list"

	if !wantsSummary && !wantsStatus && !wantsLegacyList {
		return nil
	}

	if params.Directives.RawModelProfile != "" {
		return &autoreply.ReplyPayload{Text: "Auth profile override requires a model selection."}
	}

	pickerCatalog := BuildModelPickerCatalog(BuildModelPickerCatalogParams{
		Cfg:                 params.Cfg,
		DefaultProvider:     params.DefaultProvider,
		DefaultModel:        params.DefaultModel,
		AliasIndex:          params.AliasIndex,
		AllowedModelCatalog: params.AllowedModelCatalog,
	})

	// /model list → 委托已有的 /models 命令
	if wantsLegacyList {
		return &autoreply.ReplyPayload{Text: "Use /models or /models <provider> to browse models."}
	}

	// /model (summary)
	if wantsSummary {
		current := params.Provider + "/" + params.Model
		isTelegram := params.Surface == "telegram"

		if isTelegram {
			return &autoreply.ReplyPayload{
				Text: strings.Join([]string{
					fmt.Sprintf("Current: %s", current),
					"",
					"Tap below to browse models, or use:",
					"/model <provider/model> to switch",
					"/model status for details",
				}, "\n"),
			}
		}

		return &autoreply.ReplyPayload{
			Text: strings.Join([]string{
				fmt.Sprintf("Current: %s", current),
				"",
				"Switch: /model <provider/model>",
				"Browse: /models (providers) or /models <provider> (models)",
				"More: /model status",
			}, "\n"),
		}
	}

	// /model status
	if len(pickerCatalog) == 0 {
		return &autoreply.ReplyPayload{Text: "No models available."}
	}

	// 收集每个 provider 的 auth label
	authByProvider := make(map[string]string)
	for _, entry := range pickerCatalog {
		provider := models.NormalizeProviderId(entry.Provider)
		if _, ok := authByProvider[provider]; ok {
			continue
		}
		if params.AuthResolver != nil {
			auth := params.AuthResolver(provider)
			authByProvider[provider] = FormatAuthLabel(auth)
		} else {
			authByProvider[provider] = "unknown"
		}
	}

	current := params.Provider + "/" + params.Model
	defaultLabel := params.DefaultProvider + "/" + params.DefaultModel

	// 构建 auth file 路径显示
	authFilePath := filepath.Join(params.AgentDir, "auth-profiles.json")
	formatPath := func(v string) string { return utils.ShortenHomePath(v) }

	lines := []string{
		fmt.Sprintf("Current: %s", current),
		fmt.Sprintf("Default: %s", defaultLabel),
		fmt.Sprintf("Agent: %s", params.ActiveAgentID),
		fmt.Sprintf("Auth file: %s", formatPath(authFilePath)),
	}
	if params.ResetModelOverride {
		lines = append(lines, "(previous selection reset to default)")
	}

	// 按 provider 分组展示
	type providerGroup struct {
		provider string
		entries  []ModelPickerCatalogEntry
	}
	var groups []providerGroup
	groupIndex := make(map[string]int)

	for _, entry := range pickerCatalog {
		provider := models.NormalizeProviderId(entry.Provider)
		if idx, ok := groupIndex[provider]; ok {
			groups[idx].entries = append(groups[idx].entries, entry)
		} else {
			groupIndex[provider] = len(groups)
			groups = append(groups, providerGroup{provider: provider, entries: []ModelPickerCatalogEntry{entry}})
		}
	}

	for _, grp := range groups {
		authLabel := authByProvider[grp.provider]
		if authLabel == "" {
			authLabel = "missing"
		}
		endpoint := ResolveProviderEndpointLabel(grp.provider, params.Cfg)
		endpointSuffix := " endpoint: default"
		if endpoint.Endpoint != "" {
			endpointSuffix = fmt.Sprintf(" endpoint: %s", endpoint.Endpoint)
		}
		apiSuffix := ""
		if endpoint.API != "" {
			apiSuffix = fmt.Sprintf(" api: %s", endpoint.API)
		}

		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("[%s]%s%s auth: %s",
			grp.provider, endpointSuffix, apiSuffix, authLabel))

		for _, entry := range grp.entries {
			label := grp.provider + "/" + entry.ID
			aliases := params.AliasIndex.ByKey[label]
			aliasSuffix := ""
			if len(aliases) > 0 {
				aliasSuffix = fmt.Sprintf(" (%s)", strings.Join(aliases, ", "))
			}
			lines = append(lines, fmt.Sprintf("  • %s%s", label, aliasSuffix))
		}
	}

	return &autoreply.ReplyPayload{Text: strings.Join(lines, "\n")}
}

// ---------- ResolveModelSelectionFromDirective ----------

// ModelSelectionFromDirectiveParams resolveModelSelectionFromDirective 参数。
// TS 对照: directive-handling.model.ts resolveModelSelectionFromDirective params (L314-327)
type ModelSelectionFromDirectiveParams struct {
	Directives          InlineDirectives
	Cfg                 *types.OpenAcosmiConfig
	AgentDir            string
	DefaultProvider     string
	DefaultModel        string
	AliasIndex          models.ModelAliasIndex
	AllowedModelKeys    map[string]bool // nil 或 empty = allow any
	AllowedModelCatalog []ModelPickerCatalogEntry
	Provider            string
	// AuthStore 用于 profile override 解析。nil 表示无 store。
	AuthStore *AuthProfileStore
}

// ModelSelectionFromDirectiveResult resolveModelSelectionFromDirective 结果。
type ModelSelectionFromDirectiveResult struct {
	ModelSelection  *ModelDirectiveSelection
	ProfileOverride string
	ErrorText       string
}

// numericModelRe 纯数字模型选择检测。
var numericModelRe = regexp.MustCompile(`^[0-9]+$`)

// ResolveModelSelectionFromDirective 解析 /model <provider/model> 选择。
// TS 对照: directive-handling.model.ts resolveModelSelectionFromDirective (L314-402)
func ResolveModelSelectionFromDirective(params ModelSelectionFromDirectiveParams) ModelSelectionFromDirectiveResult {
	if !params.Directives.HasModelDirective || params.Directives.RawModelDirective == "" {
		if params.Directives.RawModelProfile != "" {
			return ModelSelectionFromDirectiveResult{
				ErrorText: "Auth profile override requires a model selection.",
			}
		}
		return ModelSelectionFromDirectiveResult{}
	}

	raw := strings.TrimSpace(params.Directives.RawModelDirective)

	// 纯数字拒绝
	if numericModelRe.MatchString(raw) {
		return ModelSelectionFromDirectiveResult{
			ErrorText: strings.Join([]string{
				"Numeric model selection is not supported in chat.",
				"",
				"Browse: /models or /models <provider>",
				"Switch: /model <provider/model>",
			}, "\n"),
		}
	}

	var sel *ModelDirectiveSelection

	// 1. 尝试 explicit 解析
	explicit := models.ResolveModelRefFromString(raw, params.DefaultProvider, &params.AliasIndex)
	if explicit != nil {
		explicitKey := models.ModelKey(explicit.Provider, explicit.Model)
		allowAny := len(params.AllowedModelKeys) == 0
		if allowAny || params.AllowedModelKeys[explicitKey] {
			isDefault := explicit.Provider == params.DefaultProvider &&
				explicit.Model == params.DefaultModel
			sel = &ModelDirectiveSelection{
				Provider: explicit.Provider,
				Model:    explicit.Model,
				Source:   "directive",
			}
			if isDefault {
				sel.AckMessage = fmt.Sprintf("Model set to %s/%s (default)",
					explicit.Provider, explicit.Model)
			} else {
				sel.AckMessage = fmt.Sprintf("Model set to %s/%s",
					explicit.Provider, explicit.Model)
			}
		}
	}

	// 2. 回退到 resolveModelDirectiveSelection
	if sel == nil {
		resolved := ResolveModelDirectiveSelection(ResolveModelDirectiveParams{
			ModelArg: raw,
			State: ModelSelectionState{
				DefaultProvider: params.DefaultProvider,
				DefaultModel:    params.DefaultModel,
				AliasIndex:      params.AliasIndex,
				AllowedSet: models.AllowedModelSet{
					AllowAny:    len(params.AllowedModelKeys) == 0,
					AllowedKeys: params.AllowedModelKeys,
				},
			},
		})
		if resolved.AckMessage != "" && strings.HasPrefix(resolved.AckMessage, "Could not resolve") {
			return ModelSelectionFromDirectiveResult{ErrorText: resolved.AckMessage}
		}
		if resolved.AckMessage != "" && strings.HasPrefix(resolved.AckMessage, "Model not allowed") {
			return ModelSelectionFromDirectiveResult{ErrorText: resolved.AckMessage}
		}
		if resolved.Source == "directive" {
			sel = &resolved
		}
	}

	// 3. Auth profile override
	var profileOverride string
	if sel != nil && params.Directives.RawModelProfile != "" {
		profileResolved := ResolveProfileOverride(
			params.Directives.RawModelProfile,
			sel.Provider,
			params.AuthStore,
		)
		if profileResolved.Error != "" {
			return ModelSelectionFromDirectiveResult{ErrorText: profileResolved.Error}
		}
		profileOverride = profileResolved.ProfileID
	}

	return ModelSelectionFromDirectiveResult{
		ModelSelection:  sel,
		ProfileOverride: profileOverride,
	}
}
