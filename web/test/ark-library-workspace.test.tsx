import { afterEach, expect, test } from "bun:test";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToString } from "react-dom/server";
import { I18nextProvider } from "react-i18next";
import { i18n } from "../src/i18n";
import ArkLibraryPage from "../src/pages/ark-library";
import { useUserStore } from "../src/stores/use-user-store";

const original = useUserStore.getState();
afterEach(() => useUserStore.setState(original));

function renderLibrary(role: "admin" | "user") {
    useUserStore.setState({ user: { id: "test-user", username: "test", displayName: "Test", role, status: "active" } });
    const client = new QueryClient();
    client.setQueryData(["ark-existing-assets", "test-user", role === "admin", { groupType: "AIGC" }, ""], {
        items: [{ id: "asset-history", name: "历史上传图片", groupId: "group-history", assetType: "Image", status: "Active", url: "", error: {} }],
        nextToken: "",
    });
    client.setQueryData(["ark-existing-groups", "test-user", "AIGC"], { pages: [{ items: [{ id: "group-history", name: "已有素材组", groupType: "AIGC" }], nextToken: "" }], pageParams: [""] });
    // Zustand 服务端渲染读取初始快照，使用本用例的登录身份。
    const snapshot = useUserStore.getInitialState();
    const previousUser = snapshot.user;
    snapshot.user = useUserStore.getState().user;
    const html = renderToString(
        <QueryClientProvider client={client}>
            <I18nextProvider i18n={i18n}>
                <ArkLibraryPage />
            </I18nextProvider>
        </QueryClientProvider>,
    );
    client.clear();
    snapshot.user = previousUser;
    return html;
}

test("visible users render one library with a group dropdown and no administrator actions", () => {
    const html = renderLibrary("user");
    expect(html).toContain("方舟素材库");
    expect(html).not.toContain("正在加载平台素材管理");
    expect(html).not.toContain('aria-label="平台素材管理"');
    expect(html).toContain("历史上传图片");
    expect(html).not.toMatch(/改\s*名/);
    expect(html).not.toContain('role="tablist"');
    expect(html).toContain('aria-label="筛选素材组"');
    expect(html).not.toContain('aria-label="筛选素材组 ID"');
});

test("administrators see existing remote assets first with management actions", () => {
    const html = renderLibrary("admin");
    expect(html).toContain("历史上传图片");
    expect(html).toMatch(/改\s*名/);
    expect(html).toContain("方舟素材库");
    expect(html).not.toContain("素材组与授权");
    expect(html).not.toContain('role="tablist"');
    expect(html).not.toContain("正在加载平台素材管理");
});
