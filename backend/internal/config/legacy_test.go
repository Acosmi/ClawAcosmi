package config

import (
	"strings"
	"testing"
)

func TestFindLegacyConfigIssues(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		issues := FindLegacyConfigIssues(nil)
		if len(issues) != 0 {
			t.Fatal("expected no issues")
		}
	})

	t.Run("detects whatsapp", func(t *testing.T) {
		raw := map[string]interface{}{"whatsapp": map[string]interface{}{"token": "abc"}}
		issues := FindLegacyConfigIssues(raw)
		if len(issues) != 1 {
			t.Fatalf("expected 1 issue, got %d", len(issues))
		}
		if issues[0].Path != "whatsapp" {
			t.Fatalf("path=%s", issues[0].Path)
		}
	})

	t.Run("match function filters", func(t *testing.T) {
		// agent.model as string → issue; agent.model as object → no issue
		raw1 := map[string]interface{}{"agent": map[string]interface{}{"model": "gpt-4"}}
		issues1 := FindLegacyConfigIssues(raw1)
		found := false
		for _, i := range issues1 {
			if i.Path == "agent.model" {
				found = true
			}
		}
		if !found {
			t.Fatal("expected agent.model issue for string value")
		}

		raw2 := map[string]interface{}{"agent": map[string]interface{}{"model": map[string]interface{}{"primary": "gpt-4"}}}
		issues2 := FindLegacyConfigIssues(raw2)
		for _, i := range issues2 {
			if i.Path == "agent.model" {
				t.Fatal("should NOT flag agent.model when it's an object")
			}
		}
	})
}

func TestApplyLegacyMigrations(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		r := ApplyLegacyMigrations(nil)
		if r.Next != nil || len(r.Changes) > 0 {
			t.Fatal("expected empty result")
		}
	})

	t.Run("no changes for clean config", func(t *testing.T) {
		raw := map[string]interface{}{"agents": map[string]interface{}{"defaults": map[string]interface{}{}}}
		r := ApplyLegacyMigrations(raw)
		if r.Next != nil {
			t.Fatal("expected nil next for clean config")
		}
	})

	t.Run("providers to channels", func(t *testing.T) {
		raw := map[string]interface{}{
			"whatsapp": map[string]interface{}{"token": "abc"},
			"telegram": map[string]interface{}{"botToken": "xyz"},
		}
		r := ApplyLegacyMigrations(raw)
		if r.Next == nil {
			t.Fatal("expected migration")
		}
		channels := getRecord(r.Next["channels"])
		if channels == nil {
			t.Fatal("expected channels")
		}
		wa := getRecord(channels["whatsapp"])
		if wa == nil || wa["token"] != "abc" {
			t.Fatal("whatsapp not migrated")
		}
		tg := getRecord(channels["telegram"])
		if tg == nil || tg["botToken"] != "xyz" {
			t.Fatal("telegram not migrated")
		}
		if r.Next["whatsapp"] != nil {
			t.Fatal("old whatsapp key should be removed")
		}
	})

	t.Run("gateway token migration", func(t *testing.T) {
		raw := map[string]interface{}{
			"gateway": map[string]interface{}{"token": "secret", "port": 8080},
		}
		r := ApplyLegacyMigrations(raw)
		if r.Next == nil {
			t.Fatal("expected migration")
		}
		gw := getRecord(r.Next["gateway"])
		auth := getRecord(gw["auth"])
		if auth["token"] != "secret" {
			t.Fatalf("token=%v", auth["token"])
		}
		if auth["mode"] != "token" {
			t.Fatalf("mode=%v", auth["mode"])
		}
		if _, has := gw["token"]; has {
			t.Fatal("old token should be removed")
		}
	})

	t.Run("agent to agents.defaults", func(t *testing.T) {
		raw := map[string]interface{}{
			"agent": map[string]interface{}{"contextTokens": 100000, "timeout": 300},
		}
		r := ApplyLegacyMigrations(raw)
		if r.Next == nil {
			t.Fatal("expected migration")
		}
		agents := getRecord(r.Next["agents"])
		defaults := getRecord(agents["defaults"])
		if defaults["contextTokens"] != float64(100000) && defaults["contextTokens"] != 100000 {
			t.Fatalf("contextTokens=%v", defaults["contextTokens"])
		}
		if r.Next["agent"] != nil {
			t.Fatal("old agent key should be removed")
		}
	})

	t.Run("does not mutate original", func(t *testing.T) {
		raw := map[string]interface{}{
			"gateway": map[string]interface{}{"token": "secret"},
		}
		ApplyLegacyMigrations(raw)
		gw := getRecord(raw["gateway"])
		if gw["token"] != "secret" {
			t.Fatal("original should not be mutated")
		}
	})

	t.Run("changes list", func(t *testing.T) {
		raw := map[string]interface{}{
			"whatsapp": map[string]interface{}{"token": "t"},
		}
		r := ApplyLegacyMigrations(raw)
		if len(r.Changes) == 0 {
			t.Fatal("expected changes")
		}
		hasChannel := false
		for _, c := range r.Changes {
			if strings.Contains(c, "channels.whatsapp") {
				hasChannel = true
			}
		}
		if !hasChannel {
			t.Fatal("expected channels.whatsapp in changes")
		}
	})
}

func TestDeepCloneMap(t *testing.T) {
	src := map[string]interface{}{
		"a": "hello",
		"b": map[string]interface{}{"c": 42},
		"d": []interface{}{1, map[string]interface{}{"e": true}},
	}
	dst := deepCloneMap(src)
	// Modify dst, shouldn't affect src
	getRecord(dst["b"])["c"] = 99
	if getRecord(src["b"])["c"] == 99 {
		t.Fatal("deep clone failed - modifying dst affected src")
	}
}
