package channels

// 配置 Schema 构建 — 继承自 src/channels/plugins/config-schema.ts (12L)

// ChannelConfigSchema 频道配置 JSON Schema
type ChannelConfigSchema struct {
	Schema map[string]interface{} `json:"schema"`
}

// BuildChannelConfigSchema 从 Go struct 构建 JSON Schema (简化)
// 注意: Go 不使用 Zod, 此处提供框架接口, 实际 schema 由具体频道提供
func BuildChannelConfigSchema(schema map[string]interface{}) ChannelConfigSchema {
	return ChannelConfigSchema{Schema: schema}
}
