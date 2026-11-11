import { PageHeader, WorkspacePage } from "@/components/layout/workspace-page";
import { useUserStore } from "@/stores/use-user-store";
import ArkAssetsManager from "./ark-assets-manager";

export default function ArkLibraryPage() {
    const user = useUserStore((state) => state.user);
    return (
        <WorkspacePage>
            <PageHeader title="方舟素材库" description="查看方舟项目中的全部素材，按素材组筛选。" />
            <ArkAssetsManager scope={user?.id || ""} canManage={user?.role === "admin"} />
        </WorkspacePage>
    );
}
