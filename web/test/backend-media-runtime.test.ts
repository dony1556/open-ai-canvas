import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";

test("backend final image installs FFmpeg for timeline rendering", () => {
    const dockerfile = readFileSync(new URL("../../backend/Dockerfile", import.meta.url), "utf8");
    const runtime = dockerfile.split("FROM node:22.19-alpine").at(-1)!;
    expect(runtime).toMatch(/RUN apk add --no-cache[^\n]*\bffmpeg\b/);
});
