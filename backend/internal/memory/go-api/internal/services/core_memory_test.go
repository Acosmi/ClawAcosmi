// Package services — core_memory 单元测试。
package services

import (
	"testing"

	"github.com/uhms/go-api/internal/models"
)

func TestValidCoreSections(t *testing.T) {
	valid := []string{
		models.CoreMemSectionPersona,
		models.CoreMemSectionPreferences,
		models.CoreMemSectionInstructions,
	}
	for _, s := range valid {
		if !validCoreSections[s] {
			t.Errorf("分区 %q 应在有效列表中", s)
		}
	}
}

func TestValidCoreSections_Invalid(t *testing.T) {
	invalid := []string{"notes", "history", "", "random"}
	for _, s := range invalid {
		if validCoreSections[s] {
			t.Errorf("分区 %q 不应在有效列表中", s)
		}
	}
}

func TestCoreMemoryMap_Defaults(t *testing.T) {
	cm := &CoreMemoryMap{}
	if cm.Persona != "" || cm.Preferences != "" || cm.Instructions != "" {
		t.Fatal("默认 CoreMemoryMap 应为空")
	}
}

func TestGetCoreMemory_EmptyUserID(t *testing.T) {
	_, err := GetCoreMemory(nil, "")
	if err == nil {
		t.Fatal("空 user_id 应返回错误")
	}
}

func TestUpdateCoreMemory_InvalidSection(t *testing.T) {
	err := UpdateCoreMemory(nil, "user1", "invalid_section", "content", "agent")
	if err == nil {
		t.Fatal("无效 section 应返回错误")
	}
}

func TestUpdateCoreMemory_EmptyUserID(t *testing.T) {
	err := UpdateCoreMemory(nil, "", "persona", "content", "agent")
	if err == nil {
		t.Fatal("空 user_id 应返回错误")
	}
}
