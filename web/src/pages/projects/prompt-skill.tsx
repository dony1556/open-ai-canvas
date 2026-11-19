import { useUserStore } from "@/stores/use-user-store";
import { useSkillAdministrationAccess, useSkillLibraryAccess } from "@/lib/skill-access";
import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueries } from "@tanstack/react-query";
import { Button, Input, Select } from "antd";
import { selectSkillSection, skillHeadings, writingSkillPreview } from "@/lib/drama/writing-skill";
import { BookMarked, FileText, PenLine, Search, X } from "lucide-react";
import { Link } from "react-router";

import { AppModal } from "@/components/ui/product/app-modal";
import type { DramaSkillRef } from "@/services/api/drama";
import { getSkillFile, listAddedSkills, listSkillFiles } from "@/services/api/skills";

/** 加载写作技能的文字步骤：资产提取（作品圣经）、分镜拆分（每集剧本）、剧情段视频提示词。 */
export type DramaSkillStep = "bible" | "script" | "prompt";

const STEP_COPY: Record<DramaSkillStep, { title: string; detail: string; empty: string; foot: string }> = {
    bible: {
        title: "选择资产提取技能",
        detail: "把所选正文或章节与引用文件交给文本模型，用于提取角色、道具与场景；输出结构由系统固定。",
        empty: "未选择技能，按内置规则提取角色、道具与场景。",
        foot: "对之后的资产提取或新设定补全生效；修改设置不会自动重提取已有资产。",
    },
    script: {
        title: "选择分镜拆分技能",
        detail: "把所选正文或章节与引用文件交给文本模型拆分镜头；输出结构由系统固定。直接解析现成分镜时不执行。",
        empty: "未选择技能，按内置规则拆分镜头。",
        foot: "对之后改编的集生效；已改编的集可在剧本页“重新改编”。",
    },
    prompt: {
        title: "选择视频提示词技能",
        detail: "把所选正文或章节与引用文件交给文本模型，按本步骤结构和写法撰写剧情段提示词。",
        empty: "未选择技能，按内置格式撰写（视觉与首帧、站位轴线、角色道具、光源声音、逐镜时间码）。",
        foot: "修改后对之后撰写的剧情段生效；已写好的段可在分镜页多选后“重新撰写提示词”。",
    },
};

/**
 * 文字步骤的写作技能：显式选择正文范围与引用文件，执行时完整发送并校验预算。
 * 保存的是技能与文件引用，在技能库里修改自己的技能后，下一次执行就会读到新内容。
 */
export function PromptSkillField(props: Parameters<typeof AuthorizedPromptSkillField>[0]) {
    const allowed = useSkillLibraryAccess();
    const hydrated = useUserStore((state) => state.hydrated);
    const userId = useUserStore((state) => state.user?.id);
    useEffect(() => {
        if (hydrated && !allowed && props.value) props.onChange(undefined);
    }, [allowed, hydrated, props.value, props.onChange]);
    return allowed ? <AuthorizedPromptSkillField key={userId} {...props} /> : null;
}

function AuthorizedPromptSkillField({ value, onChange, step = "prompt", compact, aspectRatio }: { value?: DramaSkillRef; onChange: (value?: DramaSkillRef) => void; step?: DramaSkillStep; compact?: boolean; aspectRatio?: string }) {
    const canManage = useSkillAdministrationAccess();
    const [open, setOpen] = useState(false);
    const copy = STEP_COPY[step];
    return (
        <div className={`drama-pskill ${compact ? "is-compact" : ""}`}>
            {value ? (
                <div className="drama-pskill-current">
                    <BookMarked className="size-4 shrink-0 text-[var(--workspace-accent)]" aria-hidden />
                    <div className="min-w-0">
                        <strong>{value.skillName || value.skillId}</strong>
                        <span title={value.path}>
                            {value.path}
                            {value.section ? ` · ${value.section}` : ""}
                            {value.referencePaths?.length ? ` · ${value.referencePaths.length} 份引用` : ""}
                        </span>
                    </div>
                    {canManage ? (
                        <Link to={`/skills/${encodeURIComponent(value.skillId)}/edit`} target="_blank" rel="noreferrer" className="drama-pskill-edit" aria-label="在技能工作台编辑" title="在技能工作台编辑">
                            <PenLine className="size-3.5" />
                        </Link>
                    ) : null}
                    <Button size="small" onClick={() => setOpen(true)}>
                        更换
                    </Button>
                    <Button size="small" type="text" icon={<X className="size-3.5" />} aria-label="不使用技能" onClick={() => onChange(undefined)} />
                </div>
            ) : (
                <div className="drama-pskill-current is-empty">
                    <span>{compact ? "内置规则" : copy.empty}</span>
                    <Button size="small" icon={<BookMarked className="size-3.5" />} onClick={() => setOpen(true)}>
                        选择技能
                    </Button>
                </div>
            )}
            <PromptSkillModal
                aspectRatio={aspectRatio}
                open={open}
                step={step}
                initial={value}
                onClose={() => setOpen(false)}
                onPick={(ref) => {
                    onChange(ref);
                    setOpen(false);
                }}
            />
        </div>
    );
}

function PromptSkillModal({ open, step, initial, onClose, onPick, aspectRatio }: { open: boolean; step: DramaSkillStep; initial?: DramaSkillRef; onClose: () => void; onPick: (value: DramaSkillRef) => void; aspectRatio?: string }) {
    const copy = STEP_COPY[step];
    const [keyword, setKeyword] = useState("");
    const [skillId, setSkillId] = useState(initial?.skillId || "");
    const [path, setPath] = useState(initial?.path || "");
    const [section, setSection] = useState(initial?.section || "");
    const [referencePaths, setReferencePaths] = useState<string[]>(initial?.referencePaths || []);
    useEffect(() => {
        if (!open) return;
        setSkillId(initial?.skillId || "");
        setPath(initial?.path || "");
        setSection(initial?.section || "");
        setReferencePaths(initial?.referencePaths || []);
    }, [initial, open]);
    const skillsQuery = useQuery({ queryKey: ["skills", "added"], queryFn: listAddedSkills, enabled: open, staleTime: 30_000 });
    const skills = useMemo(() => (skillsQuery.data?.skills || []).filter((skill) => !keyword.trim() || `${skill.skillName} ${skill.description}`.toLowerCase().includes(keyword.trim().toLowerCase())), [keyword, skillsQuery.data]);
    const skill = skillsQuery.data?.skills.find((item) => item.skillId === skillId);
    const filesQuery = useQuery({ queryKey: ["skills", skillId, "files"], queryFn: () => listSkillFiles(skillId), enabled: open && Boolean(skillId), staleTime: 60_000 });
    const files = (filesQuery.data?.files || []).filter((file) => file.kind === "markdown" || file.kind === "text");
    // 选中技能后默认定位到 SKILL.md，这是技能的主说明。
    useEffect(() => {
        if (!skillId || path || !files.length) return;
        setPath((files.find((file) => file.path.toLowerCase() === "skill.md") || files[0]).path);
    }, [files, path, skillId]);
    const fileQuery = useQuery({ queryKey: ["skills", skillId, "file", path], queryFn: () => getSkillFile(skillId, path), enabled: open && Boolean(skillId && path), staleTime: 60_000 });
    const content = fileQuery.data?.file.content || "";
    const references = useQueries({ queries: referencePaths.map((referencePath) => ({ queryKey: ["skills", skillId, "file", referencePath], queryFn: () => getSkillFile(skillId, referencePath), enabled: open && Boolean(skillId), staleTime: 60_000 })) });
    const busy = fileQuery.isFetching || references.some((query) => query.isFetching || !query.data);
    let preview = "",
        problem = "";
    try {
        if (fileQuery.error || references.some((query) => query.error)) throw new Error("文件读取失败，请重新选择或重试。");
        if (content && !busy) preview = writingSkillPreview([{ path, section, content }, ...referencePaths.map((referencePath, i) => ({ path: referencePath, section: "", content: references[i].data!.file.content || "" }))]);
        else if (content) selectSkillSection(content, section);
    } catch (error) {
        problem = error instanceof Error ? error.message : "技能正文无效";
    }
    return (
        <AppModal open={open} onCancel={onClose} footer={null} width={1080} flush title={null} centered>
            <div className="drama-skill-import">
                <header className="drama-skill-import-head">
                    <div>
                        <strong>{copy.title}</strong>
                        <span>{copy.detail}</span>
                    </div>
                </header>
                <div className="drama-skill-import-body">
                    <aside>
                        <Input size="small" prefix={<Search className="size-3.5" />} allowClear placeholder="搜索已添加的技能" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
                        <div className="drama-skill-import-list thin-scrollbar">
                            {skills.map((item) => (
                                <button
                                    key={item.skillId}
                                    type="button"
                                    aria-pressed={item.skillId === skillId}
                                    onClick={() => {
                                        setSkillId(item.skillId);
                                        setPath("");
                                        setSection("");
                                        setReferencePaths([]);
                                    }}
                                >
                                    <strong>{item.skillName}</strong>
                                    <span>{item.description || "—"}</span>
                                </button>
                            ))}
                            {!skillsQuery.isLoading && !skills.length ? <div className="drama-skill-import-empty">还没有添加技能，可在「技能」页面添加后再选择。</div> : null}
                        </div>
                    </aside>
                    <aside>
                        <div className="drama-skill-import-caption">{skill ? `${skill.skillName} · ${files.length} 份文本` : "文件"}</div>
                        <div className="drama-skill-import-list thin-scrollbar">
                            {files.map((file) => (
                                <button
                                    key={file.path}
                                    type="button"
                                    className="is-file"
                                    aria-pressed={file.path === path}
                                    onClick={() => {
                                        setPath(file.path);
                                        setSection("");
                                        setReferencePaths([]);
                                    }}
                                >
                                    <FileText className="size-3.5 shrink-0" />
                                    <span title={file.path}>{file.path}</span>
                                </button>
                            ))}
                            {skillId && !filesQuery.isLoading && !files.length ? <div className="drama-skill-import-empty">这个技能没有文本文件。</div> : null}
                            {!skillId ? <div className="drama-skill-import-empty">先在左侧选择技能</div> : null}
                        </div>
                    </aside>
                    <section>
                        <div className="drama-skill-import-caption">
                            {path || "原文"}
                            {content ? <span>{[...content].length.toLocaleString("zh-CN")} 字符 · 按实际模型检查整条请求预算</span> : null}
                        </div>
                        <Select aria-label="本步骤使用的章节" className="w-full" value={section} onChange={setSection} options={[{ value: "", label: "使用全文" }, ...skillHeadings(content).map((heading) => ({ value: heading, label: heading }))]} />
                        <Select
                            aria-label="附加引用文件"
                            className="mt-2 w-full"
                            mode="multiple"
                            maxCount={8}
                            placeholder="显式附加本步骤需要的引用文件（最多 8 份）"
                            value={referencePaths}
                            onChange={setReferencePaths}
                            options={files.filter((file) => file.path !== path).map((file) => ({ value: file.path, label: file.path }))}
                        />
                        <p className="text-xs text-foreground/60">
                            只发送所选正文与引用，不自动执行脚本。提交任务前按实际模型检查系统提示词、剧本、技能、引用及输出预留的总预算；容量未知时无法确认是否放得下，会完整发送并在任务详情提示。Token
                            为保守估算，实际费用依上游用量。作品画幅、镜头时间与输出结构仍由系统约束。预览中的摘要占位符会在提交时替换为实际 SHA256。
                        </p>
                        {problem ? (
                            <p role="alert" className="text-red-500">
                                {problem}
                            </p>
                        ) : null}
                        {aspectRatio && preview.split("\n").some((line) => /画面比例|画幅|竖屏|横屏/.test(line) && (line.match(/\b\d+:\d+\b/g) || []).some((ratio) => ratio !== aspectRatio)) ? (
                            <p role="status" className="text-amber-600">
                                技能中出现与作品 {aspectRatio} 不同的画幅要求，请检查对应规则。实际生成沿用作品画幅，不会执行技能中相冲突的比例设置。
                            </p>
                        ) : null}
                        <pre className="drama-skill-import-content thin-scrollbar">{busy ? "正在读取…" : preview || content || "选择文件后在这里查看全文。"}</pre>
                    </section>
                </div>
                <footer className="drama-skill-import-foot">
                    <span>{copy.foot}</span>
                    <Button onClick={onClose}>取消</Button>
                    <Button type="primary" disabled={!skill || !path || !preview || busy || Boolean(problem)} onClick={() => skill && onPick({ skillId: skill.skillId, skillName: skill.skillName, path, section: section || undefined, referencePaths })}>
                        使用这个文件
                    </Button>
                </footer>
            </div>
        </AppModal>
    );
}
