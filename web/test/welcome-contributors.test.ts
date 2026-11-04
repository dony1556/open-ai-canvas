import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { parseContributors } from "../src/pages/welcome/contributors-parse";

const identityAvatar = (path: string) => path;

test("OCTV-AIGC README remains concise without a contributors section", () => {
    const readme = readFileSync(resolve(import.meta.dir, "../../README.md"), "utf8");
    expect(readme).toContain("OCTV-AIGC");
    expect(readme).toContain("## 核心能力");
    expect(readme).toContain("## 快速开始");
    expect(parseContributors(readme, identityAvatar)).toEqual([]);
});

test("welcome contributors parse HTML tables independently of README content", () => {
    const markdown = `## 贡献者与团队
<table><tr>
<td><img src="assets/user-example.jpg" alt="Example"><strong>Example<br>项目发起者 · 微信：example-id</strong><a href="mailto:founder@example.com">邮箱</a><em>把故事搬上银幕</em></td>
<td><img src="assets/user-ken.jpg" alt="ken"><strong>ken</strong><a href="mailto:ken@example.com">邮箱</a><em>走自己的路</em></td>
</tr></table>
## 下一章
`;
    expect(parseContributors(markdown, identityAvatar)).toEqual([
        { name: "Example", avatar: "assets/user-example.jpg", email: "founder@example.com", wechat: "example-id", signature: "把故事搬上银幕", isFounder: true },
        { name: "ken", avatar: "assets/user-ken.jpg", email: "ken@example.com", wechat: undefined, signature: "走自己的路", isFounder: false },
    ]);
});

test("welcome contributors still parse markdown pipe tables", () => {
    const markdown = `## 贡献者与团队

| 头像 | 昵称 | 邮箱 | 签名 |
| --- | --- | --- | --- |
| <img src="assets/user-example.jpg" alt="Example"> | Example<br>项目发起者 · 微信：example-id | founder@example.com | 把故事搬上银幕 |
| <img src="assets/user-ken.jpg" alt="ken"> | ken | ken@example.com | 走自己的路 |

## 下一章
`;
    expect(parseContributors(markdown, identityAvatar)).toEqual([
        {
            name: "Example",
            avatar: "assets/user-example.jpg",
            signature: "把故事搬上银幕",
            email: "founder@example.com",
            wechat: "example-id",
            isFounder: true,
        },
        {
            name: "ken",
            avatar: "assets/user-ken.jpg",
            signature: "走自己的路",
            email: "ken@example.com",
            wechat: undefined,
            isFounder: false,
        },
    ]);
});
