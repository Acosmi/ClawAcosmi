// Package schema 提供 Agent 工具 JSON Schema 构建与清洗工具。
// TS 参考：src/agents/schema/typebox.ts (43L)
package schema

// StringEnum 创建一个 string enum JSON Schema 定义。
// TS 中使用 TypeBox Type.Unsafe，Go 中直接构造 map。
func StringEnum(values []string, opts ...map[string]any) map[string]any {
	schema := map[string]any{
		"type": "string",
		"enum": toAnySlice(values),
	}
	if len(opts) > 0 && opts[0] != nil {
		for k, v := range opts[0] {
			schema[k] = v
		}
	}
	return schema
}

// OptionalStringEnum 创建一个可选的 string enum，设置 optional 标记。
func OptionalStringEnum(values []string, opts ...map[string]any) map[string]any {
	schema := StringEnum(values, opts...)
	// TypeBox 的 Optional 只是在属性定义时不加入 required
	// Go 端通过不将该字段加入 required 列表来实现
	return schema
}

// ChannelTargetSchema 创建频道目标 schema（字符串类型）。
func ChannelTargetSchema(description string) map[string]any {
	if description == "" {
		description = "Channel target identifier in the format 'channel:target' (e.g. 'telegram:123456789', 'discord:guild:channel')"
	}
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}

// ChannelTargetsSchema 创建频道目标数组 schema。
func ChannelTargetsSchema(description string) map[string]any {
	if description == "" {
		description = "Array of channel target identifiers"
	}
	return map[string]any{
		"type":  "array",
		"items": ChannelTargetSchema(description),
	}
}

// TypeObject 构建 JSON Schema type: object。
func TypeObject(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = toAnySlice(required)
	}
	return schema
}

// TypeString 构建 JSON Schema type: string。
func TypeString(opts ...map[string]any) map[string]any {
	schema := map[string]any{"type": "string"}
	if len(opts) > 0 && opts[0] != nil {
		for k, v := range opts[0] {
			schema[k] = v
		}
	}
	return schema
}

// TypeNumber 构建 JSON Schema type: number。
func TypeNumber(opts ...map[string]any) map[string]any {
	schema := map[string]any{"type": "number"}
	if len(opts) > 0 && opts[0] != nil {
		for k, v := range opts[0] {
			schema[k] = v
		}
	}
	return schema
}

// TypeBoolean 构建 JSON Schema type: boolean。
func TypeBoolean(opts ...map[string]any) map[string]any {
	schema := map[string]any{"type": "boolean"}
	if len(opts) > 0 && opts[0] != nil {
		for k, v := range opts[0] {
			schema[k] = v
		}
	}
	return schema
}

// TypeArray 构建 JSON Schema type: array。
func TypeArray(items map[string]any, opts ...map[string]any) map[string]any {
	schema := map[string]any{
		"type":  "array",
		"items": items,
	}
	if len(opts) > 0 && opts[0] != nil {
		for k, v := range opts[0] {
			schema[k] = v
		}
	}
	return schema
}

// TypeOptional 返回 schema 本身（Go 中在 required 列表控制可选性）。
func TypeOptional(schema map[string]any) map[string]any {
	return schema
}

func toAnySlice[T any](s []T) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}
