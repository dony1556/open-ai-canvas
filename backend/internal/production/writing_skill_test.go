package production

import (
	"strings"
	"testing"
)

func TestWritingSkillSelectionAndBudget(t *testing.T) {
	source := "# 总则\n通用\n## 资产\n提取角色\n### 细节\n保留服饰\n```md\n## 非标题\n```\n## 分镜\n保持轴线\n"
	got, err := SelectWritingSkillSection(source, "## 资产")
	if err != nil || !strings.Contains(got, "保留服饰") || strings.Contains(got, "保持轴线") {
		t.Fatalf("%q %v", got, err)
	}
	for _, body := range []string{"## Description: <br>\n介绍\n## Publisher: <br>\n作者\n## License/Terms of Use: <br>\nMIT", "\ufeff## Description:\n介绍\n## Publisher:\n作者\n## Ethical Considerations:\n说明"} {
		if _, err := SelectWritingSkillSection(body, ""); err == nil {
			t.Fatal("metadata document accepted")
		}
	}
	if _, err := SelectWritingSkillSection("## 资产\na\n## 资产\nb", "## 资产"); err == nil {
		t.Fatal("ambiguous section accepted")
	}
	if _, err := SelectWritingSkillSection(source, "## 已删除"); err == nil {
		t.Fatal("stale section accepted")
	}
	for _, body := range []string{""} {
		if ValidateWritingSkillContent(body) == nil {
			t.Fatal("invalid budget accepted")
		}
	}
	if err := ValidateWritingSkillContent(strings.Repeat("字", 32000)); err != nil {
		t.Fatal(err)
	}
}

func TestWritingSkillPromptDoesNotSilentlyTruncate(t *testing.T) {
	text := strings.Repeat("字", 32000) + "正文末尾"
	skill := &WritingSkill{Name: "阶段规范", Path: "stage.md", Content: text}
	for _, prompt := range []string{structuredSkillBlock("资产提取", skill), SegmentWriterPrompt(SegmentPromptInput{}, "", "", skill)} {
		if !strings.Contains(prompt, "正文末尾") {
			t.Fatal("prompt silently truncated; loader must reject before submission")
		}
	}
}
