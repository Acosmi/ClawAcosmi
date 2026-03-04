// apply.go — 链接理解应用层。
//
// TS 对照: link-understanding/apply.ts (38L)
//
// 运行链接理解并将结果应用到消息上下文。
package linkparse

import (
	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
	"github.com/Acosmi/ClawAcosmi/internal/autoreply/reply"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ApplyLinkUnderstandingResult 链接理解应用结果。
// TS 对照: apply.ts ApplyLinkUnderstandingResult
type ApplyLinkUnderstandingResult struct {
	Outputs []string
	URLs    []string
}

// ApplyLinkUnderstandingParams 链接理解应用参数。
type ApplyLinkUnderstandingParams struct {
	Ctx         *autoreply.MsgContext
	ToolsConfig *types.ToolsConfig
	Verbose     bool
}

// ApplyLinkUnderstanding 运行链接理解并将结果应用到上下文。
// TS 对照: apply.ts applyLinkUnderstanding()
func ApplyLinkUnderstanding(params ApplyLinkUnderstandingParams) ApplyLinkUnderstandingResult {
	result := RunLinkUnderstanding(RunLinkUnderstandingParams{
		ToolsConfig: params.ToolsConfig,
		Ctx:         params.Ctx,
		Verbose:     params.Verbose,
	})

	if len(result.Outputs) == 0 {
		return ApplyLinkUnderstandingResult{
			Outputs: result.Outputs,
			URLs:    result.URLs,
		}
	}

	// 更新上下文
	params.Ctx.Body = FormatLinkUnderstandingBody(params.Ctx.Body, result.Outputs)

	reply.FinalizeInboundContext(params.Ctx, &reply.FinalizeInboundContextOptions{
		ForceBodyForAgent:    true,
		ForceBodyForCommands: true,
	})

	return ApplyLinkUnderstandingResult{
		Outputs: result.Outputs,
		URLs:    result.URLs,
	}
}
