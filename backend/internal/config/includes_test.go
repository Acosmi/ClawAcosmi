package config

import (
	"fmt"
	"testing"
)

func mockResolver(files map[string]string) IncludeResolver {
	return IncludeResolver{
		ReadFile: func(path string) (string, error) {
			content, ok := files[path]
			if !ok {
				return "", fmt.Errorf("file not found: %s", path)
			}
			return content, nil
		},
		ParseJSON: func(raw string) (interface{}, error) {
			// 简易 JSON 解析 (测试用): 使用 Go stdlib
			var result interface{}
			err := parseJSON5([]byte(raw), &result)
			return result, err
		},
	}
}

func TestDeepMergeValues(t *testing.T) {
	t.Run("arrays concat", func(t *testing.T) {
		r := DeepMergeValues([]interface{}{1, 2}, []interface{}{3, 4})
		arr := r.([]interface{})
		if len(arr) != 4 {
			t.Fatalf("len=%d", len(arr))
		}
	})

	t.Run("objects merge", func(t *testing.T) {
		r := DeepMergeValues(
			map[string]interface{}{"a": 1, "b": 2},
			map[string]interface{}{"b": 3, "c": 4},
		)
		m := r.(map[string]interface{})
		if m["a"] != float64(1) && m["a"] != 1 {
			t.Fatalf("a=%v", m["a"])
		}
		if m["b"] != float64(3) && m["b"] != 3 {
			t.Fatalf("b=%v (source should win)", m["b"])
		}
		if m["c"] != float64(4) && m["c"] != 4 {
			t.Fatalf("c=%v", m["c"])
		}
	})

	t.Run("primitive source wins", func(t *testing.T) {
		r := DeepMergeValues("old", "new")
		if r != "new" {
			t.Fatalf("got %v", r)
		}
	})
}

func TestResolveConfigIncludes(t *testing.T) {
	t.Run("no include", func(t *testing.T) {
		obj := map[string]interface{}{"key": "value"}
		r, err := ResolveConfigIncludes(obj, "/config/main.json", mockResolver(nil))
		if err != nil {
			t.Fatal(err)
		}
		m := r.(map[string]interface{})
		if m["key"] != "value" {
			t.Fatalf("key=%v", m["key"])
		}
	})

	t.Run("single include", func(t *testing.T) {
		files := map[string]string{
			"/config/base.json": `{"name": "base", "port": 8080}`,
		}
		obj := map[string]interface{}{IncludeKey: "./base.json"}
		r, err := ResolveConfigIncludes(obj, "/config/main.json", mockResolver(files))
		if err != nil {
			t.Fatal(err)
		}
		m := r.(map[string]interface{})
		if m["name"] != "base" {
			t.Fatalf("name=%v", m["name"])
		}
	})

	t.Run("include with siblings", func(t *testing.T) {
		files := map[string]string{
			"/config/base.json": `{"name": "base", "port": 8080}`,
		}
		obj := map[string]interface{}{IncludeKey: "./base.json", "port": float64(9090)}
		r, err := ResolveConfigIncludes(obj, "/config/main.json", mockResolver(files))
		if err != nil {
			t.Fatal(err)
		}
		m := r.(map[string]interface{})
		if m["port"] != float64(9090) {
			t.Fatalf("port=%v (sibling should win)", m["port"])
		}
	})

	t.Run("array include merges", func(t *testing.T) {
		files := map[string]string{
			"/config/a.json": `{"x": 1}`,
			"/config/b.json": `{"y": 2}`,
		}
		obj := map[string]interface{}{IncludeKey: []interface{}{"./a.json", "./b.json"}}
		r, err := ResolveConfigIncludes(obj, "/config/main.json", mockResolver(files))
		if err != nil {
			t.Fatal(err)
		}
		m := r.(map[string]interface{})
		if m["x"] == nil || m["y"] == nil {
			t.Fatalf("merge failed: %v", m)
		}
	})

	t.Run("circular detection", func(t *testing.T) {
		files := map[string]string{
			"/config/a.json": `{"$include": "./main.json"}`,
		}
		obj := map[string]interface{}{IncludeKey: "./a.json"}
		_, err := ResolveConfigIncludes(obj, "/config/main.json", mockResolver(files))
		if err == nil {
			t.Fatal("expected circular error")
		}
		_, ok := err.(*CircularIncludeError)
		if !ok {
			t.Fatalf("expected CircularIncludeError, got %T: %v", err, err)
		}
	})

	t.Run("depth exceeded", func(t *testing.T) {
		files := make(map[string]string)
		for i := 0; i <= MaxIncludeDepth+1; i++ {
			files[fmt.Sprintf("/d/f%d.json", i)] = fmt.Sprintf(`{"$include": "./f%d.json"}`, i+1)
		}
		obj := map[string]interface{}{IncludeKey: "./f0.json"}
		_, err := ResolveConfigIncludes(obj, "/d/main.json", mockResolver(files))
		if err == nil {
			t.Fatal("expected depth error")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		obj := map[string]interface{}{IncludeKey: "./missing.json"}
		_, err := ResolveConfigIncludes(obj, "/config/main.json", mockResolver(nil))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
