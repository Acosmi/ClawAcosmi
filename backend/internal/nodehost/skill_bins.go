package nodehost

// skill_bins.go — Skills 可执行文件缓存
// 对应 TS: runner.ts L176-204 (SkillBinsCache)

import (
	"sync"
	"time"
)

// SkillBinsCache 缓存 skills.bins 查询结果，TTL 90 秒。
type SkillBinsCache struct {
	mu          sync.Mutex
	bins        map[string]struct{}
	lastRefresh time.Time
	ttl         time.Duration
	fetch       func() ([]string, error)
}

// NewSkillBinsCache 创建缓存实例。fetch 回调用于远程查询 skills.bins。
func NewSkillBinsCache(fetch func() ([]string, error)) *SkillBinsCache {
	return &SkillBinsCache{
		ttl:   90 * time.Second,
		fetch: fetch,
		bins:  make(map[string]struct{}),
	}
}

// Current 返回当前缓存的 bins 集合，过期时自动刷新。
func (c *SkillBinsCache) Current(force bool) map[string]struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	if force || time.Since(c.lastRefresh) > c.ttl {
		c.refresh()
	}
	// 返回副本
	result := make(map[string]struct{}, len(c.bins))
	for k, v := range c.bins {
		result[k] = v
	}
	return result
}

func (c *SkillBinsCache) refresh() {
	bins, err := c.fetch()
	if err != nil {
		if c.lastRefresh.IsZero() {
			c.bins = make(map[string]struct{})
		}
		return
	}
	m := make(map[string]struct{}, len(bins))
	for _, b := range bins {
		m[b] = struct{}{}
	}
	c.bins = m
	c.lastRefresh = time.Now()
}
