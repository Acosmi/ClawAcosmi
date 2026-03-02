package runner

// action_risk.go — 动作危险等级分类
//
// 子智能体（Argus/Coder）工具调用按危险等级分级：
//   RiskNone   — 纯读操作（截图、元素树查询），跳过审批
//   RiskLow    — 低风险操作（滚动、hover），可配置自动审批
//   RiskMedium — 中风险操作（点击、键入），请求用户确认
//   RiskHigh   — 高风险操作（导航、提交），强制审批
//
// 审批模式（ApprovalMode）：
//   "none"             — 全部跳过（开发调试用）
//   "medium_and_above" — 仅 Medium+High 需审批（推荐）
//   "all"              — 所有操作都需审批

// ActionRiskLevel 动作危险等级。
type ActionRiskLevel int

const (
	RiskNone   ActionRiskLevel = 0 // 截图、读取元素树、查询状态
	RiskLow    ActionRiskLevel = 1 // 滚动、hover、等待
	RiskMedium ActionRiskLevel = 2 // 点击、键入、选择
	RiskHigh   ActionRiskLevel = 3 // 导航、表单提交、文件操作
)

// actionRiskMap Argus MCP 工具名 → 危险等级映射。
// 覆盖全部 16 个 Argus MCP 工具（感知 7 + 动作 7 + macOS 1 + Shell 1）。
// 未知工具默认 RiskMedium（安全保守策略）。
var actionRiskMap = map[string]ActionRiskLevel{
	// 感知（RiskNone）— 纯只读
	"capture_screen":   RiskNone,
	"describe_scene":   RiskNone,
	"locate_element":   RiskNone,
	"read_text":        RiskNone,
	"detect_dialog":    RiskNone,
	"watch_for_change": RiskNone,
	"mouse_position":   RiskNone,

	// 低风险（RiskLow）
	"scroll": RiskLow,

	// 中风险（RiskMedium）— 交互操作
	"click":          RiskMedium,
	"double_click":   RiskMedium,
	"type_text":      RiskMedium,
	"press_key":      RiskMedium,
	"hotkey":         RiskMedium,
	"macos_shortcut": RiskMedium,

	// 高风险（RiskHigh）— 导航/执行
	"open_url":  RiskHigh,
	"run_shell": RiskHigh,
}

// ClassifyActionRisk 返回工具名对应的风险等级。
// 未知工具默认为 RiskMedium（安全保守策略）。
func ClassifyActionRisk(toolName string) ActionRiskLevel {
	if level, ok := actionRiskMap[toolName]; ok {
		return level
	}
	return RiskMedium
}

// ShouldRequireApproval 根据风险等级和审批模式判断是否需要用户确认。
func ShouldRequireApproval(risk ActionRiskLevel, approvalMode string) bool {
	switch approvalMode {
	case "none":
		return false
	case "all":
		return true
	case "medium_and_above", "":
		// 默认模式：Medium 及以上需审批
		return risk >= RiskMedium
	default:
		// 未知模式，保守策略
		return risk >= RiskMedium
	}
}

// RiskLevelString 返回风险等级的字符串表示。
func RiskLevelString(level ActionRiskLevel) string {
	switch level {
	case RiskNone:
		return "none"
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	default:
		return "unknown"
	}
}
