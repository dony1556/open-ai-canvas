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

test("gufeng romance preset uses warm backlight and preserves character designs", () => {
    const style = findDramaStyle("live-gufeng-romance")!;
    assert.equal(style.name, "古风短剧");
    assert.deepEqual(getDramaStyleComposition(style.id), { formId: "realistic", themeId: "palace" });
    for (const query of ["古风", "甜宠", "古装", "仙侠", "宫廷"]) assert.ok(filterDramaStyles("live", query).includes(style), query);
    for (const text of Object.values(style.detail)) assert.ok(style.prompt.includes(text), text);
    for (const text of ["暖金高位侧逆光直射镜头", "宽软光晕与半透明圆形炫光", "光束在薄雾中散开", "暗部留冷蓝环境反光", "眼神、发丝与衣料纹理可辨", "配饰与具体妆容随角色设定"]) assert.ok(style.prompt.includes(text), text);
    assert.match(style.detail.wardrobe, /奶油肌.*甜美女主.*桃杏.*水润玫瑰唇.*清俊男主.*暖棕红眼尾.*随角色设定/);
    assert.ok(style.prompt.length <= 360);
    assert.doesNotMatch(style.prompt, /冷白侧逆光|深冷青和蓝灰留在背景|薄刘海|垂珠|花簪|发冠|长眉凤眼|月白交领袍|粉白花绣|9:16/);
});
