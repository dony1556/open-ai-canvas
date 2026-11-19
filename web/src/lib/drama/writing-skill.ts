export function skillHeadings(content: string) {
    let fenced = false;
    return content
        .replace(/\r\n/g, "\n")
        .split("\n")
        .filter((line) => {
            if (/^\s*(```|~~~)/.test(line)) {
                fenced = !fenced;
                return false;
            }
            return !fenced && /^#{1,6}\s+.+/.test(line);
        })
        .map((line) => line.trim());
}

export function selectSkillSection(content: string, section: string) {
    content = content.replace(/^\uFEFF/, "").trim();
    if ((content.match(/^#{1,6}\s*(?:Description|Publisher|License\/Terms of Use|Deployment Geography for Use|Ethical Considerations)\s*:/gim) || []).length >= 3) throw new Error("这是技能介绍/许可文档，请选择包含操作规则的文件。");
    if (!content) throw new Error("技能正文为空。");
    if (!section) return content;
    const headings = skillHeadings(content);
    if (headings.filter((heading) => heading === section).length !== 1) throw new Error("章节不存在或标题重复，请重新选择。");
    const lines = content.replace(/\r\n/g, "\n").split("\n");
    let start = -1,
        end = lines.length,
        fenced = false;
    const level = section.match(/^#+/)![0].length;
    for (let i = 0; i < lines.length; i++) {
        if (/^\s*(```|~~~)/.test(lines[i])) {
            fenced = !fenced;
            continue;
        }
        if (fenced) continue;
        if (start < 0 && lines[i].trim() === section) {
            start = i;
            continue;
        }
        const heading = lines[i].match(/^(#{1,6})\s+/);
        if (start >= 0 && heading && heading[1].length <= level) {
            end = i;
            break;
        }
    }
    return lines.slice(start, end).join("\n").trim();
}

export function writingSkillPreview(files: { path: string; section: string; content: string }[]) {
    const preview = files.map((file) => `【技能文件：${file.path}｜章节：${file.section || "全文"}｜SHA256：${"0".repeat(64)}】\n${selectSkillSection(file.content, file.section)}`).join("\n\n");
    return preview;
}

export function skillRequestEvidence(prompt: string) {
    const body = prompt.match(/<skill>\n?([\s\S]*?)\n?<\/skill>/)?.[1];
    if (!body) return "本次请求未附加写作技能。";
    const files = body.match(/^【技能文件：.+】$/gm);
    return files ? `本次请求已附加技能正文（${[...body].length} 字符）。以下是提交时的文件与内容摘要；不代表输出已通过技能规则验收。\n${files.join("\n")}` : "本次历史请求包含技能文本，但没有文件摘要记录，无法确认是否完整加载；请查看当时的提示词正文。";
}
