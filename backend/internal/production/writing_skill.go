package production

import (
	"fmt"
	"regexp"
	"strings"
)

var skillHeading = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
var skillMetadata = regexp.MustCompile(`(?mi)^#{1,6}\s*(?:Description|Publisher|License/Terms of Use|Deployment Geography for Use|Ethical Considerations)\s*:`)

// SelectWritingSkillSection selects an explicit heading and its children, never a guessed task route.
func SelectWritingSkillSection(content, section string) (string, error) {
	content = strings.TrimSpace(strings.TrimPrefix(content, "\ufeff"))
	if len(skillMetadata.FindAllString(content, -1)) >= 3 {
		return "", fmt.Errorf("所选文件是技能介绍/许可文档，请选择包含操作规则的 SKILL.md 或步骤文件")
	}
	if section == "" {
		return content, nil
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	start, end, level, matches := -1, len(lines), 0, 0
	fenced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") || strings.HasPrefix(strings.TrimSpace(line), "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		m := skillHeading.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if start >= 0 && end == len(lines) && len(m[1]) <= level {
			end = i
		}
		if strings.TrimSpace(line) == section {
			matches++
			if start < 0 {
				start = i
				level = len(m[1])
			}
		}
	}
	if matches != 1 {
		return "", fmt.Errorf("所选章节不存在或标题重复，请重新选择章节：%s", section)
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n")), nil
}

func ValidateWritingSkillContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("技能正文为空")
	}
	return nil
}
