package understanding

import "strings"

// TS 对照: media-understanding/providers/index.ts (57L)

// NormalizeMediaProviderId 规范化 Provider ID（小写、去空白）。
// TS 对照: providers/index.ts L8-10
func NormalizeMediaProviderId(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

// Registry Provider 注册表。
type Registry struct {
	providers map[string]*Provider
}

// NewRegistry 创建空注册表。
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]*Provider),
	}
}

// Register 注册 Provider。
func (r *Registry) Register(p *Provider) {
	r.providers[NormalizeMediaProviderId(p.ID)] = p
}

// Get 按 ID 获取 Provider。
// TS 对照: providers/index.ts L35-40
func (r *Registry) Get(id string) *Provider {
	return r.providers[NormalizeMediaProviderId(id)]
}

// All 返回所有已注册的 Provider。
func (r *Registry) All() []*Provider {
	result := make([]*Provider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}

// FindForCapability 查找支持指定能力的 Provider 列表。
func (r *Registry) FindForCapability(kind Kind) []*Provider {
	var result []*Provider
	for _, p := range r.providers {
		for _, cap := range p.Capabilities {
			if cap.Kind == kind {
				result = append(result, p)
				break
			}
		}
	}
	return result
}

// BuildDefaultRegistry 构建包含所有内置 Provider 的注册表。
// TS 对照: providers/index.ts L12-33
func BuildDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(NewOpenAIProvider())
	r.Register(NewGoogleProvider())
	r.Register(NewAnthropicProvider())
	r.Register(NewDeepgramProvider())
	r.Register(NewGroqProvider())
	r.Register(NewMiniMaxProvider())
	return r
}
