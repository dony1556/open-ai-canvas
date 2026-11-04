import { expect, test } from "bun:test";
import { canvasTaskBindingStatus, isStaleCanvasTaskObservation } from "@/lib/canvas/canvas-task-state";
import { CanvasNodeType, type CanvasNodeData } from "@/types/canvas";
const node: CanvasNodeData = {
    id: "layer",
    type: CanvasNodeType.Image,
    position: { x: 0, y: 0 },
    width: 20,
    height: 20,
    metadata: { taskId: "extract", content: "/opaque.png", status: "idle", layerExtraction: { sourceNodeId: "source", groupId: "group", index: 1, phase: "background-removal-required", extractionTaskId: "extract" } },
};
test("同一任务的迟到排队状态不能覆盖终态，新任务仍可绑定", () => {
    for (const taskStatus of ["succeeded", "failed", "cancelled"]) {
        const completed = { ...node, metadata: { ...node.metadata, taskStatus } };
        for (const status of ["queued", "running"] as const) {
            expect(isStaleCanvasTaskObservation(completed, { id: "extract", status })).toBe(true);
            expect(isStaleCanvasTaskObservation(completed, { id: "new-task", status })).toBe(false);
        }
        expect(isStaleCanvasTaskObservation(completed, { id: "extract", status: "succeeded" })).toBe(false);
    }
    expect(isStaleCanvasTaskObservation(node, { id: "extract", status: "succeeded" })).toBe(false);
});
test("待去背景的原始提取成功仅为中间结果", () => {
    expect(canvasTaskBindingStatus(node, { id: "extract", status: "succeeded" })).toBe("idle");
});
test("去背景任务上游成功而未验收时，旧图不能充当新结果", () => {
    const candidate = { ...node, metadata: { ...node.metadata, taskId: "remove", layerExtraction: { ...node.metadata!.layerExtraction!, phase: "remove-background" as const } } };
    expect(canvasTaskBindingStatus(candidate, { id: "remove", status: "succeeded" })).toBe("loading");
});
test("被本地透明度验收拒绝的成功任务不能抹掉失败状态", () => {
    const candidate = { ...node, metadata: { ...node.metadata, status: "error" as const, layerExtraction: { ...node.metadata!.layerExtraction!, rejectedTaskId: "remove" } } };
    expect(canvasTaskBindingStatus(candidate, { id: "remove", status: "succeeded" })).toBe("error");
});
test("验收完成须与同一任务绑定，普通节点维持已有规则", () => {
    const validated = { ...node, metadata: { ...node.metadata, taskId: "remove", layerExtraction: { ...node.metadata!.layerExtraction!, phase: "complete" as const } } };
    expect(canvasTaskBindingStatus(validated, { id: "remove", status: "succeeded" })).toBe("success");
    expect(canvasTaskBindingStatus(validated, { id: "older", status: "succeeded" })).toBe("loading");
    expect(canvasTaskBindingStatus({ ...node, metadata: { content: "/ordinary.png" } }, { id: "ordinary", status: "succeeded" })).toBe("success");
    expect(canvasTaskBindingStatus(validated, { id: "remove", status: "failed" })).toBe("error");
});
