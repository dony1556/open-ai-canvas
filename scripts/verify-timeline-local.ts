import { spawnSync } from "node:child_process";
import { copyFileSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { resolve } from "node:path";
import { buildTimelineRenderPlan } from "../web/src/lib/timeline/timeline-to-ffmpeg";
import type { TimelineProject } from "../web/src/types/timeline";

// Uses an existing local clip only; never submits generation requests.
const [sourceArg, outputArg] = process.argv.slice(2);
if (!sourceArg || !outputArg) throw new Error("Usage: bun scripts/verify-timeline-local.ts <existing.mp4> <new-output-directory>");
const source = resolve(sourceArg);
const output = resolve(outputArg);
mkdirSync(output, { recursive: false });
const records: unknown[] = [];
function run(command: string, args: string[]) {
    const started = performance.now();
    const result = spawnSync(command, args, { cwd: output, encoding: "utf8" });
    records.push({ command, args, exitCode: result.status, elapsedMs: performance.now() - started, stderr: result.stderr, stdout: result.stdout, error: result.error?.message });
    writeFileSync(resolve(output, "commands.json"), JSON.stringify(records, null, 2));
    if (result.error || result.status !== 0) throw new Error(`${command} failed; see commands.json`);
    return result.stdout;
}
copyFileSync(source, resolve(output, "source.mp4"));
const project: TimelineProject = {
    version: 2, tracks: [], durationMs: 6000,
    clips: [
        { id: "A", kind: "video", nodeId: "source", trackId: "video", title: "A", startMs: 0, durationMs: 2000, sourceStartMs: 0, sourceDurationMs: 4000 },
        { id: "B", kind: "video", nodeId: "source", trackId: "video", title: "B", startMs: 4000, durationMs: 2000, sourceStartMs: 2000, sourceDurationMs: 4000 },
    ],
};
const plan = buildTimelineRenderPlan(project, [{ nodeId: "source", fileName: "source.mp4", durationMs: 4000 }], { width: 1344, height: 768, fps: 24, burnSubtitles: false, outputName: "timeline-output.mp4" });
writeFileSync(resolve(output, "input.json"), JSON.stringify({ classification: "real_local_execution_with_technical_fixture", source, project, plan }, null, 2));
writeFileSync(resolve(output, "concat.txt"), plan.concatEntries.map((name) => `file '${name}'`).join("\n") + "\n");
for (const step of plan.steps) run("ffmpeg", ["-nostdin", "-n", ...step.args]);
const probe = JSON.parse(run("ffprobe", ["-v", "error", "-show_format", "-show_streams", "-of", "json", plan.finalOutput]));
writeFileSync(resolve(output, "probe.json"), JSON.stringify(probe, null, 2));
if (Math.abs(Number(probe.format.duration) - 6) > 0.1) throw new Error("Unexpected output duration");
const finalVideo = readFileSync(resolve(output, plan.finalOutput));
writeFileSync(resolve(output, "result.json"), JSON.stringify({ status: "passed", durationSeconds: Number(probe.format.duration), sha256: createHash("sha256").update(finalVideo).digest("hex"), sizeBytes: finalVideo.length, providerCharge: 0, scope: "Local FFmpeg execution of the frontend planner; not the authenticated server render flow" }, null, 2));
console.log(JSON.stringify({ status: "passed", output, steps: plan.steps.length, seconds: probe.format.duration }));
