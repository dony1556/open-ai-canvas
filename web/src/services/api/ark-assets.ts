import { compactApiParams, http } from "./request";

export type ArkAsset = {
    id: string;
    name: string;
    url: string;
    groupId: string;
    assetType: string;
    status: string;
    projectName: string;
    createTime: string;
    lastInferenceTime: string;
    error: { code: string; message: string };
};

export type ArkAssetQuery = { name?: string; groupId?: string; groupType?: string; status?: string; nextToken?: string };

export type ArkExistingGroup = { id: string; name: string; groupType: string };

export function listExistingArkGroups(groupType: string, nextToken?: string, signal?: AbortSignal) {
    return http.get<{ items: ArkExistingGroup[]; nextToken: string }>("/ark-assets/groups", { params: compactApiParams({ groupType, nextToken }), signal });
}

export function listArkAssets(query: ArkAssetQuery, signal?: AbortSignal) {
    return http.get<{ items: ArkAsset[]; nextToken: string }>("/admin/ark-assets", { params: compactApiParams(query), signal });
}

export function listExistingArkAssets(query: ArkAssetQuery, signal?: AbortSignal) {
    return http.get<{ items: ArkAsset[]; nextToken: string }>("/ark-assets", { params: compactApiParams(query), signal });
}

export function renameArkAsset(id: string, name: string) {
    return http.patch(`/admin/ark-assets/${encodeURIComponent(id)}`, { name });
}

export function deleteArkAsset(id: string) {
    return http.delete(`/admin/ark-assets/${encodeURIComponent(id)}`);
}
