package models

import (
	"context"
	"errors"
	"fmt"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ---------- 模型失败切换 ----------

// TS 参考: src/agents/model-fallback.ts (395 行)

// ModelCandidate 候选模型。
type ModelCandidate struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// FallbackAttempt 失败切换尝试记录。
type FallbackAttempt struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Error    string `json:"error"`
	Reason   string `json:"reason,omitempty"`
	Status   int    `json:"status,omitempty"`
	Code     string `json:"code,omitempty"`
}

// FallbackResult 失败切换执行结果。
type FallbackResult[T any] struct {
	Result   T
	Provider string
	Model    string
	Attempts []FallbackAttempt
}

// ---------- 候选模型解析 ----------

// ResolveFallbackCandidates 解析失败切换候选列表。
// TS 参考: model-fallback.ts → resolveFallbackCandidates()
// 规则:
// 1. 主模型始终在首位
// 2. 如果有 fallbacksOverride 则使用它
// 3. 否则使用 cfg.agents.defaults.model.fallbacks
// 4. 去重
func ResolveFallbackCandidates(cfg *types.OpenAcosmiConfig, provider, model string, fallbacksOverride []string) []ModelCandidate {
	seen := make(map[string]bool)
	candidates := []ModelCandidate{}

	// BUG-7: 构建白名单 (nil = allowAny)
	allowlistKeys := BuildConfiguredAllowlistKeys(cfg, DefaultProvider)

	addCandidate := func(c ModelCandidate, enforceAllowlist bool) {
		if c.Provider == "" || c.Model == "" {
			return
		}
		key := ModelKey(c.Provider, c.Model)
		if seen[key] {
			return
		}
		// BUG-7: 白名单强制检查
		if enforceAllowlist && allowlistKeys != nil && !allowlistKeys[key] {
			return
		}
		seen[key] = true
		candidates = append(candidates, c)
	}

	// 1. 主模型
	addCandidate(ModelCandidate{Provider: provider, Model: model}, false)

	// 2. 收集 fallbacks
	var rawFallbacks []string
	if fallbacksOverride != nil {
		rawFallbacks = fallbacksOverride
	} else if cfg != nil && cfg.Agents != nil && cfg.Agents.Defaults != nil && cfg.Agents.Defaults.Model != nil && cfg.Agents.Defaults.Model.Fallbacks != nil {
		rawFallbacks = *cfg.Agents.Defaults.Model.Fallbacks
	}

	// 3. 解析每个 fallback
	for _, raw := range rawFallbacks {
		ref := ParseModelRef(raw, provider)
		if ref == nil {
			continue
		}
		addCandidate(ModelCandidate{Provider: ref.Provider, Model: ref.Model}, true)
	}

	// BUG-8: TS L200-202 — 配置的 primary 作为最终后备
	if fallbacksOverride == nil {
		primary := ResolveConfiguredModelRef(cfg, DefaultProvider, DefaultModel)
		addCandidate(ModelCandidate{Provider: primary.Provider, Model: primary.Model}, false)
	}

	return candidates
}

// ResolveImageFallbackCandidates 解析图像模型失败切换候选。
// TS 参考: model-fallback.ts → resolveImageFallbackCandidates()
func ResolveImageFallbackCandidates(cfg *types.OpenAcosmiConfig, defaultProvider string, modelOverride string) []ModelCandidate {
	seen := make(map[string]bool)
	candidates := []ModelCandidate{}

	addCandidate := func(c ModelCandidate) {
		key := ModelKey(c.Provider, c.Model)
		if seen[key] {
			return
		}
		seen[key] = true
		candidates = append(candidates, c)
	}

	// modelOverride 优先
	if modelOverride != "" {
		ref := ParseModelRef(modelOverride, defaultProvider)
		if ref != nil {
			addCandidate(ModelCandidate{Provider: ref.Provider, Model: ref.Model})
		}
	}

	// 配置的 imageModel
	if cfg != nil && cfg.Agents != nil && cfg.Agents.Defaults != nil && cfg.Agents.Defaults.ImageModel != nil {
		im := cfg.Agents.Defaults.ImageModel
		if im.Primary != "" {
			ref := ParseModelRef(im.Primary, defaultProvider)
			if ref != nil {
				addCandidate(ModelCandidate{Provider: ref.Provider, Model: ref.Model})
			}
		}
		if im.Fallbacks != nil {
			for _, fb := range *im.Fallbacks {
				ref := ParseModelRef(fb, defaultProvider)
				if ref != nil {
					addCandidate(ModelCandidate{Provider: ref.Provider, Model: ref.Model})
				}
			}
		}
	}

	return candidates
}

// ---------- 失败切换执行器 ----------

// AuthProfileChecker 认证 profile 冷却检查接口（避免与 runner 包循环导入）。
// 由调用方将完整的 AuthProfileStore 适配传入。
type AuthProfileChecker interface {
	ResolveProfileOrder(cfg *types.OpenAcosmiConfig, provider string, preferred string) []string
	IsInCooldown(profileID string) bool
}

// RunFunc 执行函数类型。
type RunFunc[T any] func(ctx context.Context, provider, model string) (T, error)

// OnErrorFunc 错误回调类型。
type OnErrorFunc func(provider, model string, err error, attempt, total int)

// RunWithModelFallback 带失败切换的模型执行。
// TS 参考: model-fallback.ts → runWithModelFallback()
// authStore 可为 nil，当非 nil 时会跳过所有 profile 均在冷却中的候选。
func RunWithModelFallback[T any](
	ctx context.Context,
	cfg *types.OpenAcosmiConfig,
	provider, model string,
	fallbacksOverride []string,
	authStore AuthProfileChecker,
	run RunFunc[T],
	onError OnErrorFunc,
) (*FallbackResult[T], error) {
	candidates := ResolveFallbackCandidates(cfg, provider, model, fallbacksOverride)
	if len(candidates) == 0 {
		candidates = []ModelCandidate{{Provider: provider, Model: model}}
	}

	var attempts []FallbackAttempt
	total := len(candidates)

	for i, c := range candidates {
		// TS L242-260: 跳过所有 auth profile 均在冷却中的候选
		if authStore != nil {
			profileIDs := authStore.ResolveProfileOrder(cfg, c.Provider, "")
			if len(profileIDs) > 0 {
				anyAvailable := false
				for _, id := range profileIDs {
					if !authStore.IsInCooldown(id) {
						anyAvailable = true
						break
					}
				}
				if !anyAvailable {
					attempts = append(attempts, FallbackAttempt{
						Provider: c.Provider,
						Model:    c.Model,
						Error:    fmt.Sprintf("Provider %s is in cooldown (all profiles unavailable)", c.Provider),
						Reason:   string(FailoverRateLimit),
					})
					continue
				}
			}
		}

		result, err := run(ctx, c.Provider, c.Model)
		if err == nil {
			return &FallbackResult[T]{
				Result:   result,
				Provider: c.Provider,
				Model:    c.Model,
				Attempts: attempts,
			}, nil
		}

		// BUG-6: TS shouldRethrowAbort — 用户取消/超时直接返回
		if isAbortError(ctx, err) {
			return nil, err
		}

		descMsg, descReason, descStatus, descCode := DescribeFailoverError(err)
		attempt := FallbackAttempt{
			Provider: c.Provider,
			Model:    c.Model,
			Error:    descMsg,
			Reason:   string(descReason),
			Status:   descStatus,
			Code:     descCode,
		}
		attempts = append(attempts, attempt)

		if onError != nil {
			onError(c.Provider, c.Model, err, i+1, total)
		}

		// 最后一个候选 — 不再重试
		if i == len(candidates)-1 {
			return nil, fmt.Errorf("所有模型均失败 (尝试 %d 次): 最后错误: %w", len(attempts), err)
		}

		// 非可切换错误直接返回
		if !ShouldFailover(err) {
			return nil, fmt.Errorf("不可恢复错误 (%s/%s): %w", c.Provider, c.Model, err)
		}
	}

	// 不应到达此处
	var zero T
	return &FallbackResult[T]{Result: zero}, fmt.Errorf("无候选模型")
}

// isAbortError 检查是否为用户主动取消或超时。
// TS 参考: model-fallback.ts L37-52 — shouldRethrowAbort()
func isAbortError(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return true
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return false
}
