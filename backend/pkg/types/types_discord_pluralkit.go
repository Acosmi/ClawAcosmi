package types

// PluralKit 配置类型 — 继承自 src/discord/pluralkit.ts

// DiscordPluralKitConfig PluralKit 配置
type DiscordPluralKitConfig struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Token   string `json:"token,omitempty"`
}
