import assert from "node:assert/strict";
import { test } from "node:test";

import { DEFAULT_DRAMA_STYLE_ID, DRAMA_STYLE_CATEGORIES, DRAMA_STYLES, filterDramaStyles, findDramaStyle, getDramaStyleComposition } from "./drama-styles";

test("style ids are unique and the default exists", () => {
    assert.equal(new Set(DRAMA_STYLES.map((style) => style.id)).size, DRAMA_STYLES.length);
    assert.ok(findDramaStyle(DEFAULT_DRAMA_STYLE_ID));
});

test("every category has styles and prompts fit the server limit", () => {
    for (const category of DRAMA_STYLE_CATEGORIES) assert.ok(filterDramaStyles(category.key, "").length > 0, category.label);
    for (const style of DRAMA_STYLES) assert.ok(style.prompt.length <= 1200, style.name);
});

test("search matches names and tags across categories", () => {
    assert.ok(filterDramaStyles("hot", "仙侠").some((style) => style.id === "3d-guoman"));
    assert.ok(filterDramaStyles("all", "霸总").length >= 2);
});

test("urban daily 3D preset preserves its art direction and is searchable", () => {
    const style = findDramaStyle("3d-urban-daily")!;
    assert.deepEqual(getDramaStyleComposition(style.id), { formId: "3d", themeId: "urban" });
    for (const query of ["都市日常", "现代", "青春", "3D写实"]) assert.ok(filterDramaStyles("3d", query).includes(style), query);
    for (const text of ["国产 3D 写实 CG", "UE5", "商场", "地铁站", "厨房", "纯白摄影棚", "仅用于角色立绘展示", "电影级景深", "具体穿戴由分镜中的角色设定决定", "真人照片质感", "2D平涂", "穿模", "文字水印"]) assert.ok(style.prompt.includes(text), text);
});
