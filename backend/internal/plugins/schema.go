package plugins

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateJsonSchemaValue 使用 JSON Schema 校验值
// 对应 TS: schema-validator.ts validateJsonSchemaValue
// 注意：Go 版使用内建基础校验替代 AJV。
// 如需完整 JSON Schema Draft-07 支持，可引入 github.com/santhosh-tekuri/jsonschema/v5
func ValidateJsonSchemaValue(schema map[string]interface{}, value interface{}) (bool, []string) {
	if schema == nil {
		return true, nil
	}

	// 基础类型校验
	schemaType, _ := schema["type"].(string)
	switch schemaType {
	case "object":
		obj, ok := toMap(value)
		if !ok {
			return false, []string{"expected object"}
		}
		return validateObjectSchema(schema, obj)
	case "string":
		if _, ok := value.(string); !ok {
			return false, []string{"expected string"}
		}
		return true, nil
	case "number", "integer":
		if _, ok := toFloat64(value); !ok {
			return false, []string{fmt.Sprintf("expected %s", schemaType)}
		}
		return true, nil
	case "boolean":
		if _, ok := value.(bool); !ok {
			return false, []string{"expected boolean"}
		}
		return true, nil
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return false, []string{"expected array"}
		}
		return true, nil
	}

	return true, nil
}

func validateObjectSchema(schema map[string]interface{}, obj map[string]interface{}) (bool, []string) {
	var errors []string

	// required
	if required, ok := schema["required"].([]interface{}); ok {
		for _, r := range required {
			key, _ := r.(string)
			if key == "" {
				continue
			}
			if _, exists := obj[key]; !exists {
				errors = append(errors, fmt.Sprintf("%s: is required", key))
			}
		}
	}

	// properties
	props, _ := schema["properties"].(map[string]interface{})

	// additionalProperties
	additionalProps := true
	if ap, ok := schema["additionalProperties"]; ok {
		if b, ok := ap.(bool); ok {
			additionalProps = b
		}
	}

	if !additionalProps && props != nil {
		for key := range obj {
			if _, defined := props[key]; !defined {
				errors = append(errors, fmt.Sprintf("%s: additional property not allowed", key))
			}
		}
	}

	if len(errors) > 0 {
		return false, errors
	}
	return true, nil
}

// EmptyPluginConfigSchema 创建空 schema
// 对应 TS: config-schema.ts emptyPluginConfigSchema
func EmptyPluginConfigSchema() PluginConfigSchema {
	return PluginConfigSchema{
		SafeParse: func(value interface{}) PluginConfigValidation {
			if value == nil {
				return PluginConfigValidation{OK: true, Value: nil}
			}
			obj, ok := toMap(value)
			if !ok {
				return PluginConfigValidation{OK: false, Errors: []string{"expected config object"}}
			}
			if len(obj) > 0 {
				return PluginConfigValidation{OK: false, Errors: []string{"config must be empty"}}
			}
			return PluginConfigValidation{OK: true, Value: value}
		},
		JsonSchema: map[string]interface{}{
			"type":                 "object",
			"additionalProperties": false,
			"properties":           map[string]interface{}{},
		},
	}
}

// --- helpers ---

func toMap(v interface{}) (map[string]interface{}, bool) {
	if v == nil {
		return nil, false
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m, true
	}
	// 尝试 JSON 反序列化
	data, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	return m, true
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func formatSchemaErrors(errors []string) string {
	if len(errors) == 0 {
		return "invalid config"
	}
	return strings.Join(errors, "; ")
}
