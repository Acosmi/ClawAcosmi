package config

// Config includes: $include 指令支持模块化配置
//
// 对应 src/config/includes.ts (250L)
//
// 用法:
//   {"$include": "./base.json5"}           // 单文件
//   {"$include": ["./a.json5", "./b.json5"]} // 合并多文件

import (
	"fmt"
	"path/filepath"
	"strings"
)

const IncludeKey = "$include"
const MaxIncludeDepth = 10

// ── 错误类型 ──

type ConfigIncludeError struct {
	Message     string
	IncludePath string
	Cause       error
}

func (e *ConfigIncludeError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *ConfigIncludeError) Unwrap() error { return e.Cause }

type CircularIncludeError struct {
	Chain []string
}

func (e *CircularIncludeError) Error() string {
	return fmt.Sprintf("Circular include detected: %s", strings.Join(e.Chain, " -> "))
}

// ── IncludeResolver 接口 ──

type IncludeResolver struct {
	ReadFile  func(path string) (string, error)
	ParseJSON func(raw string) (interface{}, error)
}

// ── DeepMerge ──

// DeepMergeValues 深度合并: 数组拼接, 对象递归合并, 原始值以 source 为准
func DeepMergeValues(target, source interface{}) interface{} {
	tArr, tIsArr := target.([]interface{})
	sArr, sIsArr := source.([]interface{})
	if tIsArr && sIsArr {
		result := make([]interface{}, 0, len(tArr)+len(sArr))
		result = append(result, tArr...)
		result = append(result, sArr...)
		return result
	}

	tMap, tIsMap := target.(map[string]interface{})
	sMap, sIsMap := source.(map[string]interface{})
	if tIsMap && sIsMap {
		result := make(map[string]interface{}, len(tMap)+len(sMap))
		for k, v := range tMap {
			result[k] = v
		}
		for k, v := range sMap {
			if existing, has := result[k]; has {
				result[k] = DeepMergeValues(existing, v)
			} else {
				result[k] = v
			}
		}
		return result
	}
	return source
}

// ── IncludeProcessor ──

type includeProcessor struct {
	basePath string
	resolver IncludeResolver
	visited  map[string]bool
	depth    int
}

func newIncludeProcessor(basePath string, resolver IncludeResolver) *includeProcessor {
	visited := map[string]bool{filepath.Clean(basePath): true}
	return &includeProcessor{basePath: basePath, resolver: resolver, visited: visited, depth: 0}
}

func (p *includeProcessor) process(obj interface{}) (interface{}, error) {
	// 数组: 递归处理每个元素
	if arr, ok := obj.([]interface{}); ok {
		result := make([]interface{}, len(arr))
		for i, item := range arr {
			processed, err := p.process(item)
			if err != nil {
				return nil, err
			}
			result[i] = processed
		}
		return result, nil
	}

	// 非 map: 原样返回
	m, ok := obj.(map[string]interface{})
	if !ok {
		return obj, nil
	}

	// 无 $include: 递归处理所有 key
	if _, has := m[IncludeKey]; !has {
		return p.processObject(m)
	}

	return p.processInclude(m)
}

func (p *includeProcessor) processObject(obj map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(obj))
	for key, value := range obj {
		processed, err := p.process(value)
		if err != nil {
			return nil, err
		}
		result[key] = processed
	}
	return result, nil
}

func (p *includeProcessor) processInclude(obj map[string]interface{}) (interface{}, error) {
	includeValue := obj[IncludeKey]

	// 收集非 $include 的 sibling keys
	var otherKeys []string
	for k := range obj {
		if k != IncludeKey {
			otherKeys = append(otherKeys, k)
		}
	}

	included, err := p.resolveInclude(includeValue)
	if err != nil {
		return nil, err
	}

	if len(otherKeys) == 0 {
		return included, nil
	}

	// sibling keys 要求 included 是对象
	if _, ok := included.(map[string]interface{}); !ok {
		incPath := IncludeKey
		if s, ok := includeValue.(string); ok {
			incPath = s
		}
		return nil, &ConfigIncludeError{
			Message: "Sibling keys require included content to be an object", IncludePath: incPath,
		}
	}

	rest := make(map[string]interface{}, len(otherKeys))
	for _, key := range otherKeys {
		processed, err := p.process(obj[key])
		if err != nil {
			return nil, err
		}
		rest[key] = processed
	}
	return DeepMergeValues(included, rest), nil
}

func (p *includeProcessor) resolveInclude(value interface{}) (interface{}, error) {
	if s, ok := value.(string); ok {
		return p.loadFile(s)
	}

	if arr, ok := value.([]interface{}); ok {
		var merged interface{} = map[string]interface{}{}
		for _, item := range arr {
			s, ok := item.(string)
			if !ok {
				return nil, &ConfigIncludeError{
					Message:     fmt.Sprintf("Invalid $include array item: expected string, got %T", item),
					IncludePath: fmt.Sprintf("%v", item),
				}
			}
			loaded, err := p.loadFile(s)
			if err != nil {
				return nil, err
			}
			merged = DeepMergeValues(merged, loaded)
		}
		return merged, nil
	}

	return nil, &ConfigIncludeError{
		Message:     fmt.Sprintf("Invalid $include value: expected string or array, got %T", value),
		IncludePath: fmt.Sprintf("%v", value),
	}
}

func (p *includeProcessor) loadFile(includePath string) (interface{}, error) {
	resolved := p.resolvePath(includePath)

	// 循环检测
	if p.visited[resolved] {
		chain := make([]string, 0, len(p.visited)+1)
		for k := range p.visited {
			chain = append(chain, k)
		}
		chain = append(chain, resolved)
		return nil, &CircularIncludeError{Chain: chain}
	}

	// 深度检测
	if p.depth >= MaxIncludeDepth {
		return nil, &ConfigIncludeError{
			Message:     fmt.Sprintf("Maximum include depth (%d) exceeded at: %s", MaxIncludeDepth, includePath),
			IncludePath: includePath,
		}
	}

	// 读文件
	raw, err := p.resolver.ReadFile(resolved)
	if err != nil {
		return nil, &ConfigIncludeError{
			Message:     fmt.Sprintf("Failed to read include file: %s (resolved: %s)", includePath, resolved),
			IncludePath: includePath, Cause: err,
		}
	}

	// 解析
	parsed, err := p.resolver.ParseJSON(raw)
	if err != nil {
		return nil, &ConfigIncludeError{
			Message:     fmt.Sprintf("Failed to parse include file: %s (resolved: %s)", includePath, resolved),
			IncludePath: includePath, Cause: err,
		}
	}

	// 嵌套处理
	return p.processNested(resolved, parsed)
}

func (p *includeProcessor) resolvePath(includePath string) string {
	if filepath.IsAbs(includePath) {
		return filepath.Clean(includePath)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(p.basePath), includePath))
}

func (p *includeProcessor) processNested(resolvedPath string, parsed interface{}) (interface{}, error) {
	nested := &includeProcessor{
		basePath: resolvedPath,
		resolver: p.resolver,
		visited:  make(map[string]bool, len(p.visited)+1),
		depth:    p.depth + 1,
	}
	for k := range p.visited {
		nested.visited[k] = true
	}
	nested.visited[resolvedPath] = true
	return nested.process(parsed)
}

// ── 公开 API ──

// ResolveConfigIncludes 解析配置中所有 $include 指令
func ResolveConfigIncludes(obj interface{}, configPath string, resolver IncludeResolver) (interface{}, error) {
	return newIncludeProcessor(configPath, resolver).process(obj)
}
