// pw_ai_loop.go — AI-guided browser automation loop.
//
// Architecture: Google DeepMind Project Mariner "observe → plan → act" pattern.
//
// The AIBrowseLoop captures screenshots and ARIA snapshots, sends them to
// an LLM for analysis, and executes the returned actions. This loop repeats
// until the goal is achieved or limits are reached.
//
// Integration point: The actual LLM call is delegated via the AIPlanner
// interface, allowing the Agent Runner pipeline to provide the implementation.
//
// TS source: pw-ai.ts + pw-ai-module.ts (AI vision analysis)
package browser

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/anthropic/open-acosmi/pkg/i18n"
)

// AIBrowseAction represents a single browser action to execute.
type AIBrowseAction struct {
	Type      string `json:"type"`      // "click" | "fill" | "navigate" | "scroll" | "wait" | "done"
	Ref       string `json:"ref"`       // element ref for click/fill
	Value     string `json:"value"`     // text for fill, URL for navigate
	Reasoning string `json:"reasoning"` // LLM's explanation for this action
}

// AIBrowseState captures the current browser state for AI analysis.
type AIBrowseState struct {
	Screenshot   []byte         // PNG screenshot
	AriaSnapshot map[string]any // AI-friendly ARIA snapshot with refs
	CurrentURL   string         // current page URL
	StepNumber   int            // current step in the loop
}

// AIPlanner is the interface for LLM-based planning.
// The Agent Runner pipeline provides the actual implementation.
type AIPlanner interface {
	// Plan analyzes the current browser state and returns the next action.
	Plan(ctx context.Context, goal string, state AIBrowseState) (*AIBrowseAction, error)
}

// AIBrowseLoopConfig configures the AI browse loop.
type AIBrowseLoopConfig struct {
	MaxSteps          int           // maximum number of observe-plan-act cycles; default 20
	StepTimeout       time.Duration // timeout per step; default 30s
	ScreenshotEnabled bool          // capture screenshots; default true
	Logger            *slog.Logger
}

// AIBrowseResult is the outcome of an AI browse session.
type AIBrowseResult struct {
	Success    bool             `json:"success"`
	StepsTaken int              `json:"stepsTaken"`
	Actions    []AIBrowseAction `json:"actions"`
	FinalURL   string           `json:"finalUrl"`
	Error      string           `json:"error,omitempty"`
}

// AIBrowseLoop implements the observe→plan→act cycle for AI-guided browsing.
type AIBrowseLoop struct {
	tools   PlaywrightTools
	planner AIPlanner
	config  AIBrowseLoopConfig
	logger  *slog.Logger
}

// NewAIBrowseLoop creates a new AI browse loop.
func NewAIBrowseLoop(tools PlaywrightTools, planner AIPlanner, cfg AIBrowseLoopConfig) *AIBrowseLoop {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 20
	}
	if cfg.MaxSteps > 100 {
		cfg.MaxSteps = 100
	}
	if cfg.StepTimeout <= 0 {
		cfg.StepTimeout = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	// Default: screenshots enabled.
	if !cfg.ScreenshotEnabled {
		cfg.ScreenshotEnabled = true
	}

	return &AIBrowseLoop{
		tools:   tools,
		planner: planner,
		config:  cfg,
		logger:  cfg.Logger,
	}
}

// Run executes the AI browse loop for the given goal.
func (l *AIBrowseLoop) Run(ctx context.Context, goal string, target PWTargetOpts) (*AIBrowseResult, error) {
	result := &AIBrowseResult{}

	l.logger.Info(i18n.T("browser.ai.loop.start", map[string]string{
		"goal":     goal,
		"maxSteps": fmt.Sprintf("%d", l.config.MaxSteps),
	}))

	for step := 0; step < l.config.MaxSteps; step++ {
		stepCtx, cancel := context.WithTimeout(ctx, l.config.StepTimeout)

		// Phase 1: OBSERVE
		state, err := l.observe(stepCtx, target, step)
		if err != nil {
			cancel()
			l.logger.Warn(i18n.T("browser.ai.observe.failed", map[string]string{
				"step":  fmt.Sprintf("%d", step),
				"error": err.Error(),
			}))
			result.Error = fmt.Sprintf("observe step %d: %s", step, err)
			return result, nil
		}

		// Phase 2: PLAN
		action, err := l.planner.Plan(stepCtx, goal, *state)
		if err != nil {
			cancel()
			l.logger.Warn(i18n.T("browser.ai.plan.failed", map[string]string{
				"step":  fmt.Sprintf("%d", step),
				"error": err.Error(),
			}))
			result.Error = fmt.Sprintf("plan step %d: %s", step, err)
			return result, nil
		}

		result.Actions = append(result.Actions, *action)

		// Check for completion.
		if action.Type == "done" {
			cancel()
			result.Success = true
			result.StepsTaken = step + 1
			l.logger.Info(i18n.T("browser.ai.loop.done", map[string]string{
				"steps": fmt.Sprintf("%d", step+1),
			}))
			break
		}

		// Phase 3: ACT
		if err := l.act(stepCtx, target, action); err != nil {
			cancel()
			l.logger.Warn(i18n.T("browser.ai.act.failed", map[string]string{
				"step":   fmt.Sprintf("%d", step),
				"action": action.Type,
				"error":  err.Error(),
			}))
			// Don't abort on action failure; let the planner handle recovery.
		}

		cancel()
		result.StepsTaken = step + 1
	}

	if !result.Success {
		result.Error = fmt.Sprintf("max steps (%d) reached without completing goal", l.config.MaxSteps)
		l.logger.Warn(i18n.Tp("browser.ai.loop.max_steps"))
	}

	return result, nil
}

// observe captures the current browser state.
func (l *AIBrowseLoop) observe(ctx context.Context, target PWTargetOpts, step int) (*AIBrowseState, error) {
	state := &AIBrowseState{StepNumber: step}

	// Capture ARIA snapshot.
	snapshot, err := l.tools.SnapshotAI(ctx, PWSnapshotOpts{PWTargetOpts: target})
	if err != nil {
		l.logger.Debug("snapshot failed, continuing without", "err", err)
	} else {
		state.AriaSnapshot = snapshot
	}

	// Capture screenshot if enabled.
	if l.config.ScreenshotEnabled {
		data, err := l.tools.Screenshot(ctx, target)
		if err != nil {
			l.logger.Debug("screenshot failed, continuing without", "err", err)
		} else {
			state.Screenshot = data
		}
	}

	return state, nil
}

// act executes a single browser action.
func (l *AIBrowseLoop) act(ctx context.Context, target PWTargetOpts, action *AIBrowseAction) error {
	l.logger.Debug("executing action",
		"type", action.Type,
		"ref", action.Ref,
		"reasoning", action.Reasoning,
	)

	switch action.Type {
	case "click":
		return l.tools.Click(ctx, PWClickOpts{
			PWTargetOpts: target,
			Ref:          action.Ref,
		})
	case "fill":
		return l.tools.Fill(ctx, PWFillOpts{
			PWTargetOpts: target,
			Ref:          action.Ref,
			Value:        action.Value,
		})
	case "hover":
		return l.tools.Hover(ctx, target, action.Ref, 5000)
	case "navigate":
		cdp := NewCDPClient(target.CDPURL, nil)
		return cdp.Navigate(ctx, action.Value)
	case "scroll":
		// Scroll into view for the referenced element.
		return l.tools.Highlight(ctx, target, action.Ref)
	case "wait":
		// Brief pause for page loading.
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	case "done":
		return nil
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}
