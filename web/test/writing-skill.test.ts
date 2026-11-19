import { expect, test } from "bun:test";
import { selectSkillSection, skillHeadings, skillRequestEvidence, writingSkillPreview } from "../src/lib/drama/writing-skill";

test("step sections keep child rules and ignore fenced headings", () => {
    const source = "# 总则\n## 资产\n提取角色\n### 细节\n保留服饰\n```md\n## 代码示例\n```\n## 分镜\n保持轴线";
    expect(skillHeadings(source)).not.toContain("## 代码示例");
    expect(selectSkillSection(source, "## 资产")).toContain("保留服饰");
    expect(selectSkillSection(source, "## 资产")).not.toContain("保持轴线");
    expect(() => selectSkillSection(source, "## 已删除")).toThrow();
    expect(() => selectSkillSection("## 资产\na\n## 资产\nb", "## 资产")).toThrow();
});
test("preview rejects metadata and preserves long bundles without a fixed character cap", () => {
    expect(() => selectSkillSection("## Description: <br>\na\n## Publisher: <br>\nb\n## License/Terms of Use: <br>\nMIT", "")).toThrow("介绍");
    expect(writingSkillPreview([{ path: "SKILL.md", section: "", content: "字".repeat(32000) + "正文末尾" }])).toContain("正文末尾");
    const text = writingSkillPreview([
        { path: "stage.md", section: "", content: "规则" },
        { path: "references/a.md", section: "", content: "模板末尾" },
    ]);
    expect(text).toContain("模板末尾");
    expect(skillRequestEvidence(`<skill>\n${text}\n</skill>`)).toContain("references/a.md");
    expect(skillRequestEvidence("<skill>旧正文</skill>")).toContain("无法确认");
    expect(skillRequestEvidence("内置提示词")).toContain("未附加");
});
