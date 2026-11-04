import { describe, expect, it, spyOn } from "bun:test";
import localforage from "localforage";

import { buildCanvasAgentMentionReferences } from "@/lib/canvas/canvas-resource-references";
import { cloudAgentConversationTitle, loadCloudAgentConversations } from "@/services/cloud-agent-conversations";
import { CanvasNodeType } from "@/types/canvas";

describe("cloud Agent conversations", () => {
    it("removes legacy temporary approval errors when restoring cache, preserving real history", async () => {
        const originalWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
        Object.defineProperty(globalThis, "window", { configurable: true, value: { localStorage: {} } });
        const messages = [
            { id: "approval-error-123", role: "error", text: "旧审批失败" },
            { id: "task-error-123", role: "error", text: "生成失败" },
            { id: "answer-123", role: "assistant", text: "已完成" },
            { id: "approval-error-user", role: "user", text: "用户原文" },
        ];
        const conversation = { id: "conversation", title: "测试", messages, run: null, permissionMode: "request_approval", createdAt: "2026-10-29", updatedAt: "2026-10-29" };
        const read = spyOn(localforage, "getItem").mockResolvedValue(JSON.stringify({ version: 1, activeId: "conversation", conversations: [conversation] }));
        try {
            const restored = await loadCloudAgentConversations("test-user");
            expect(restored.activeId).toBe("conversation");
            expect(restored.conversations[0]).toEqual({ ...conversation, messages: messages.slice(1) });
        } finally {
            read.mockRestore();
            if (originalWindow) Object.defineProperty(globalThis, "window", originalWindow);
            else Reflect.deleteProperty(globalThis, "window");
        }
    });

    it("exposes every canvas node through stable @ mention tokens", () => {
        const references = buildCanvasAgentMentionReferences([
            { id: "frame-1", type: CanvasNodeType.Frame, title: "第一幕", position: { x: 0, y: 0 }, width: 640, height: 360 },
            { id: "config-1", type: CanvasNodeType.Config, title: "生成配置", position: { x: 700, y: 0 }, width: 320, height: 240, metadata: { prompt: "电影感" } },
        ]);

        expect(references).toHaveLength(2);
        expect(references[0]).toMatchObject({ nodeId: "frame-1", label: "第一幕", active: true, mentionToken: "@[node:frame-1]" });
        expect(references[1]).toMatchObject({ nodeId: "config-1", text: "电影感", mentionToken: "@[node:config-1]" });
    });

    it("uses the first user prompt as a compact history title", () => {
        expect(cloudAgentConversationTitle([{ id: "1", role: "assistant", text: "你好" }])).toBe("新对话");
        expect(cloudAgentConversationTitle([{ id: "2", role: "user", text: "  帮我整理分镜  " }])).toBe("帮我整理分镜");
        expect(cloudAgentConversationTitle([{ id: "3", role: "user", text: "一".repeat(40) }])).toBe(`${"一".repeat(28)}...`);
        expect(cloudAgentConversationTitle([{ id: "4", role: "user", text: "规划短剧 @[skill:internal-skill-id]" }])).toBe("规划短剧 技能包");
    });
});
