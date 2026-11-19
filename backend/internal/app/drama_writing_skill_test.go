package app

import (
	"strings"
	"testing"
	"yingce/backend/internal/model"
)

func TestDramaWritingSkillLoadsSelectedSectionAndReferences(t *testing.T) {
	s, db, _, _ := creationTestService(t)
	skillAdministratorFixture(t, db, "user")
	created, err := s.CreateSkill("user", SkillMutationRequest{SkillName: "分步骤规范", Description: "测试", Instruction: "# 工作法\n## 资产\n保留服饰\n## 视频\n保留时间码", Tag: "others", IsPrivate: true})
	if err != nil {
		t.Fatal(err)
	}
	refText := "# 声音\n备注不念出台词"
	_, err = s.EditSkillPackageFiles("user", created.SkillID, SkillPackageEditRequest{BaseVersionID: created.VersionID, Operations: []SkillPackageFileOperation{{Op: "write", Path: "references/audio.md", Content: &refText}}})
	if err != nil {
		t.Fatal(err)
	}
	ref := &DramaSkillRef{SkillID: created.SkillID, Path: "SKILL.md", Section: "## 视频", ReferencePaths: []string{"references/audio.md"}}
	skill, err := s.dramaWritingSkill("user", ref, "视频提示词")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(skill.Content, "SHA256：") || !strings.Contains(skill.Content, refText) || strings.Contains(skill.Content, "保留服饰") {
		t.Fatalf("invalid request snapshot: %s", skill.Content)
	}
	if err := db.Model(&model.User{}).Where("id = ?", "user").Update("role", model.UserRoleUser).Error; err != nil {
		t.Fatal(err)
	}
	if selected, err := s.dramaWritingSkill("user", ref, "视频提示词"); err != nil || selected == nil || selected.Content != skill.Content {
		t.Fatalf("ordinary user lost selected section or references: %+v %v", selected, err)
	}
	if err := db.Model(&model.User{}).Where("id = ?", "user").Update("role", model.UserRoleAdmin).Error; err != nil {
		t.Fatal(err)
	}
	for _, paths := range [][]string{{"references/missing.md"}, {"../SKILL.md"}, {"references/audio.md", "references/audio.md"}} {
		ref.ReferencePaths = paths
		if _, err := s.dramaWritingSkill("user", ref, "视频提示词"); err == nil {
			t.Fatalf("invalid references accepted: %v", paths)
		}
	}
	if _, err := s.dramaWritingSkill("other", ref, "视频提示词"); err == nil {
		t.Fatal("skill access check bypassed")
	}
	for i, body := range []string{"## Description: <br>\n介绍\n## Publisher: <br>\n作者\n## License/Terms of Use: <br>\nMIT"} {
		item, err := s.CreateSkill("user", SkillMutationRequest{SkillName: "无效规范" + strings.Repeat("X", i), Description: "测试", Instruction: body, Tag: "others", IsPrivate: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.dramaWritingSkill("user", &DramaSkillRef{SkillID: item.SkillID, Path: "SKILL.md"}, "资产提取"); err == nil {
			t.Fatal("invalid content silently accepted")
		}
	}
}

func TestDramaSkillHistoryKeepsFullSubmittedPrompt(t *testing.T) {
	prompt := "<skill>\n【技能文件：stage.md｜SHA256：test】\n" + strings.Repeat("规则", 5000) + "\n</skill>"
	view := dramaJobTaskView(&model.Task{Prompt: prompt, InputJSON: "{}", ResultJSON: "{}"}, nil)
	if view.Prompt != prompt {
		t.Fatal("historical prompt was truncated and lost skill evidence")
	}
}
