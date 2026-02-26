package channels

// 提及门控逻辑 — 继承自 src/channels/mention-gating.ts (60 行)

// MentionGateParams @提及门控输入参数
type MentionGateParams struct {
	RequireMention      bool
	CanDetectMention    bool
	WasMentioned        bool
	ImplicitMention     bool
	ShouldBypassMention bool
}

// MentionGateResult @提及门控结果
type MentionGateResult struct {
	EffectiveWasMentioned bool
	ShouldSkip            bool
}

// ResolveMentionGating 解析提及门控逻辑
func ResolveMentionGating(p MentionGateParams) MentionGateResult {
	effective := p.WasMentioned || p.ImplicitMention || p.ShouldBypassMention
	shouldSkip := p.RequireMention && p.CanDetectMention && !effective
	return MentionGateResult{
		EffectiveWasMentioned: effective,
		ShouldSkip:            shouldSkip,
	}
}

// MentionGateWithBypassParams 含命令绕过的门控参数
type MentionGateWithBypassParams struct {
	IsGroup           bool
	RequireMention    bool
	CanDetectMention  bool
	WasMentioned      bool
	ImplicitMention   bool
	HasAnyMention     bool
	AllowTextCommands bool
	HasControlCommand bool
	CommandAuthorized bool
}

// MentionGateWithBypassResult 含绕过标记的门控结果
type MentionGateWithBypassResult struct {
	MentionGateResult
	ShouldBypassMention bool
}

// ResolveMentionGatingWithBypass 解析含命令绕过的提及门控
func ResolveMentionGatingWithBypass(p MentionGateWithBypassParams) MentionGateWithBypassResult {
	bypass := p.IsGroup &&
		p.RequireMention &&
		!p.WasMentioned &&
		!p.HasAnyMention &&
		p.AllowTextCommands &&
		p.CommandAuthorized &&
		p.HasControlCommand

	gate := ResolveMentionGating(MentionGateParams{
		RequireMention:      p.RequireMention,
		CanDetectMention:    p.CanDetectMention,
		WasMentioned:        p.WasMentioned,
		ImplicitMention:     p.ImplicitMention,
		ShouldBypassMention: bypass,
	})

	return MentionGateWithBypassResult{
		MentionGateResult:   gate,
		ShouldBypassMention: bypass,
	}
}
