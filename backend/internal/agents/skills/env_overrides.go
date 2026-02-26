package skills

// env_overrides.go — 技能环境变量覆盖
// 对应 TS: agents/skills/env-overrides.ts (90L)
//
// 在技能激活时注入环境变量，支持还原。

import (
	"os"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// envUpdate 记录一次环境变量更新。
type envUpdate struct {
	Key  string
	Prev *string // nil = 之前不存在
}

// ApplySkillEnvOverrides 应用技能环境变量覆盖。返回还原函数。
// 对应 TS: applySkillEnvOverrides
func ApplySkillEnvOverrides(entries []SkillEntry, config *types.OpenAcosmiConfig) func() {
	var updates []envUpdate

	for _, entry := range entries {
		skillKey := ResolveSkillKey(entry.Skill.Name, entry.Metadata)
		skillConfig := ResolveSkillConfig(config, skillKey)
		if skillConfig == nil {
			continue
		}

		// 从 skillConfig.Env 注入
		if skillConfig.Env != nil {
			for envKey, envValue := range skillConfig.Env {
				if envValue == "" || os.Getenv(envKey) != "" {
					continue
				}
				prev := getEnvPtr(envKey)
				updates = append(updates, envUpdate{Key: envKey, Prev: prev})
				os.Setenv(envKey, envValue)
			}
		}

		// 从 primaryEnv + apiKey 注入
		if entry.Metadata != nil && entry.Metadata.PrimaryEnv != "" {
			primaryEnv := entry.Metadata.PrimaryEnv
			if skillConfig.APIKey != "" && os.Getenv(primaryEnv) == "" {
				prev := getEnvPtr(primaryEnv)
				updates = append(updates, envUpdate{Key: primaryEnv, Prev: prev})
				os.Setenv(primaryEnv, skillConfig.APIKey)
			}
		}
	}

	// 返回还原函数
	return func() {
		for i := len(updates) - 1; i >= 0; i-- {
			u := updates[i]
			if u.Prev == nil {
				os.Unsetenv(u.Key)
			} else {
				os.Setenv(u.Key, *u.Prev)
			}
		}
	}
}

// ApplySkillEnvOverridesFromSnapshot 从快照应用环境变量覆盖。
// 对应 TS: applySkillEnvOverridesFromSnapshot
func ApplySkillEnvOverridesFromSnapshot(snapshot *SkillSnapshot, config *types.OpenAcosmiConfig) func() {
	if snapshot == nil {
		return func() {}
	}

	var updates []envUpdate

	for _, skill := range snapshot.Skills {
		skillConfig := ResolveSkillConfig(config, skill.Name)
		if skillConfig == nil {
			continue
		}

		if skillConfig.Env != nil {
			for envKey, envValue := range skillConfig.Env {
				if envValue == "" || os.Getenv(envKey) != "" {
					continue
				}
				prev := getEnvPtr(envKey)
				updates = append(updates, envUpdate{Key: envKey, Prev: prev})
				os.Setenv(envKey, envValue)
			}
		}

		if skill.PrimaryEnv != "" && skillConfig.APIKey != "" && os.Getenv(skill.PrimaryEnv) == "" {
			prev := getEnvPtr(skill.PrimaryEnv)
			updates = append(updates, envUpdate{Key: skill.PrimaryEnv, Prev: prev})
			os.Setenv(skill.PrimaryEnv, skillConfig.APIKey)
		}
	}

	return func() {
		for i := len(updates) - 1; i >= 0; i-- {
			u := updates[i]
			if u.Prev == nil {
				os.Unsetenv(u.Key)
			} else {
				os.Setenv(u.Key, *u.Prev)
			}
		}
	}
}

func getEnvPtr(key string) *string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	return &v
}
