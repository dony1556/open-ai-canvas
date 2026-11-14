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
    assert.deepEqual(style.tags, ["现代", "日常", "青春", "2.5D半写实", "国漫CG"]);
    for (const query of ["都市日常", "现代", "青春", "2.5D半写实", "国漫CG"]) assert.ok(filterDramaStyles("3d", query).includes(style), query);
    assert.match(style.detail.world, /纯白摄影棚背景（用于角色立绘展示，配柔光箱打光）/);
    assert.match(style.detail.look, /强烈的轮廓光（Rim light）/);
    assert.match(style.detail.wardrobe, /避免真实衣物的褶皱粗糙感/);
    for (const text of ["2.5D 半写实精致 CG", "BJD娃娃质感", "二次元精致五官", "无瑕透亮肌肤", "皮肤带陶瓷微反光", "绝对不要真人照片质感，保持3D动漫CG感", "粗糙毛孔", "文字水印"]) assert.ok(style.prompt.includes(text), text);
    assert.doesNotMatch(JSON.stringify(style), /UE5|真实的皮肤纹理|3D写实/);
});
