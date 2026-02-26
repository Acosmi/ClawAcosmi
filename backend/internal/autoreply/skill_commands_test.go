package autoreply

import (
	"testing"
)

func TestNormalizeSkillCommandLookup(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"MySkill", "myskill"},
		{"my skill", "my-skill"},
		{"my_skill", "my-skill"},
		{"  My_Skill Name ", "my-skill-name"},
		{"", ""},
	}
	for _, tt := range tests {
		got := NormalizeSkillCommandLookup(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeSkillCommandLookup(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFindSkillCommand(t *testing.T) {
	specs := []SkillCommandSpec{
		{Name: "code-review"},
		{Name: "deploy app"},
		{Name: "run_tests"},
	}

	// 精确匹配
	got := FindSkillCommand(specs, "code-review")
	if got == nil || got.Name != "code-review" {
		t.Errorf("expected code-review, got %v", got)
	}

	// 模糊匹配（空格→连字符）
	got = FindSkillCommand(specs, "deploy-app")
	if got == nil || got.Name != "deploy app" {
		t.Errorf("expected deploy app via fuzzy, got %v", got)
	}

	// 下划线→连字符
	got = FindSkillCommand(specs, "run-tests")
	if got == nil || got.Name != "run_tests" {
		t.Errorf("expected run_tests via fuzzy, got %v", got)
	}

	// 无匹配
	got = FindSkillCommand(specs, "nonexistent")
	if got != nil {
		t.Errorf("expected nil for nonexistent, got %v", got)
	}

	// 空输入
	got = FindSkillCommand(specs, "")
	if got != nil {
		t.Errorf("expected nil for empty, got %v", got)
	}
}

func TestResolveSkillCommandInvocation(t *testing.T) {
	specs := []SkillCommandSpec{
		{Name: "deploy"},
		{Name: "code-review"},
	}

	// /skill deploy args
	inv := ResolveSkillCommandInvocation(specs, "/skill deploy staging")
	if inv == nil {
		t.Fatal("expected invocation for '/skill deploy staging'")
	}
	if inv.Spec.Name != "deploy" || inv.Args != "staging" {
		t.Errorf("got spec=%q args=%q", inv.Spec.Name, inv.Args)
	}

	// /deploy args (直接名称)
	inv = ResolveSkillCommandInvocation(specs, "/deploy prod")
	if inv == nil {
		t.Fatal("expected invocation for '/deploy prod'")
	}
	if inv.Args != "prod" {
		t.Errorf("expected args=prod, got %q", inv.Args)
	}

	// /code-review (无参数)
	inv = ResolveSkillCommandInvocation(specs, "/code-review")
	if inv == nil {
		t.Fatal("expected invocation for '/code-review'")
	}
	if inv.Args != "" {
		t.Errorf("expected empty args, got %q", inv.Args)
	}

	// 无匹配
	inv = ResolveSkillCommandInvocation(specs, "/unknown arg")
	if inv != nil {
		t.Errorf("expected nil for unknown command, got %v", inv)
	}

	// 空输入
	inv = ResolveSkillCommandInvocation(specs, "")
	if inv != nil {
		t.Errorf("expected nil for empty, got %v", inv)
	}

	// 非命令
	inv = ResolveSkillCommandInvocation(specs, "hello world")
	if inv != nil {
		t.Errorf("expected nil for non-command, got %v", inv)
	}
}
