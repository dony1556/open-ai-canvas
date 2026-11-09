import { Alert, App, Button, Form, Image, Input, Select, Space, Table, Tag } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { deleteArkAsset, listArkAssets, renameArkAsset, type ArkAsset, type ArkAssetQuery } from "@/services/api/ark-assets";

const statusNames: Record<string, string> = { Active: "可用", Processing: "处理中", Failed: "失败" };

export default function ArkAssetsManager({ ready, disabled, scope }: { ready: boolean; disabled: boolean; scope: string }) {
    const { message, modal } = App.useApp();
    const [filters, setFilters] = useState<ArkAssetQuery>({ groupType: "AIGC" });
    const [tokens, setTokens] = useState([""]);
    const [page, setPage] = useState(0);
    const [working, setWorking] = useState("");
    const query = useQuery({
        queryKey: ["admin-ark-assets", scope, filters, tokens[page]],
        queryFn: ({ signal }) => listArkAssets({ ...filters, nextToken: tokens[page] }, signal),
        enabled: ready && !disabled,
        retry: false,
        staleTime: 0,
    });
    const busy = disabled || Boolean(working) || query.isFetching;

    const rename = async (asset: ArkAsset) => {
        let name = asset.name;
        await modal.confirm({
            title: "修改方舟素材名称",
            content: (
                <Input
                    aria-label="素材新名称"
                    defaultValue={name}
                    maxLength={64}
                    onChange={(event) => {
                        name = event.target.value;
                    }}
                />
            ),
            okText: "保存名称",
            cancelText: "取消",
            onOk: async () => {
                setWorking(asset.id);
                try {
                    await renameArkAsset(asset.id, name);
                    message.success("方舟素材名称已更新");
                    await query.refetch();
                } catch (error) {
                    message.error(error instanceof Error ? error.message : "修改失败");
                    throw error;
                } finally {
                    setWorking("");
                }
            },
        });
    };

    const remove = async (asset: ArkAsset) => {
        await modal.confirm({
            title: `删除方舟素材“${asset.name || asset.id}”？`,
            content: "删除不可恢复，旧方舟素材 ID 将不能再用于生成。系统原图会保留；本系统已记录的绑定不会自动重新入库，需要时可在画布手动重新上传。已有方舟 ID 引用或正在使用该素材的任务会阻止删除。",
            okText: "永久删除方舟素材",
            cancelText: "取消",
            okButtonProps: { danger: true },
            onOk: async () => {
                setWorking(asset.id);
                try {
                    await deleteArkAsset(asset.id);
                    message.success("方舟素材已删除，系统原图已保留");
                    await query.refetch();
                } catch (error) {
                    message.error(error instanceof Error ? error.message : "删除失败");
                    throw error;
                } finally {
                    setWorking("");
                }
            },
        });
    };

    if (!ready) return <Alert type="info" showIcon title="保存完整的方舟项目及 IAM 凭据后，即可管理已上传素材。关闭自动导入不影响手动上传和管理。" />;

    return (
        <div className="space-y-4">
            <p className="text-sm text-muted-foreground">读取当前方舟项目的实际素材，包括通过方舟控制台上传的素材。分组可在方舟控制台维护；这里可按组 ID 筛选。</p>
            <Form
                layout="inline"
                initialValues={{ groupType: "AIGC" }}
                disabled={busy}
                className="gap-y-3"
                onFinish={(values: ArkAssetQuery) => {
                    setFilters(values);
                    setTokens([""]);
                    setPage(0);
                }}
            >
                <Form.Item name="name">
                    <Input aria-label="搜索素材名称" placeholder="素材名称" maxLength={64} allowClear />
                </Form.Item>
                <Form.Item name="groupId">
                    <Input aria-label="筛选素材组 ID" placeholder="素材组 ID（可选）" maxLength={160} allowClear />
                </Form.Item>
                <Form.Item name="groupType">
                    <Select
                        aria-label="素材组类型"
                        style={{ width: 130 }}
                        options={[
                            { value: "AIGC", label: "虚拟人像" },
                            { value: "LivenessFace", label: "真人素材" },
                        ]}
                    />
                </Form.Item>
                <Form.Item name="status">
                    <Select aria-label="素材状态" placeholder="全部状态" allowClear style={{ width: 120 }} options={Object.entries(statusNames).map(([value, label]) => ({ value, label }))} />
                </Form.Item>
                <Form.Item>
                    <Button htmlType="submit" type="primary">
                        查询
                    </Button>
                </Form.Item>
                <Form.Item>
                    <Button onClick={() => void query.refetch()}>刷新素材</Button>
                </Form.Item>
            </Form>
            {disabled ? <Alert type="info" title="请先保存或撤销上方配置调整，再操作素材。" /> : null}
            {query.isError ? (
                <Alert
                    type="error"
                    showIcon
                    title="读取方舟素材失败"
                    description={query.error instanceof Error ? query.error.message : "请稍后重试"}
                    action={
                        <Button disabled={busy} onClick={() => void query.refetch()}>
                            重试
                        </Button>
                    }
                />
            ) : null}
            <Table<ArkAsset>
                rowKey="id"
                dataSource={query.data?.items || []}
                loading={query.isFetching}
                pagination={false}
                scroll={{ x: 1100 }}
                locale={{ emptyText: query.isError ? "读取失败，请重试" : "当前筛选条件下没有方舟素材" }}
                columns={[
                    {
                        title: "预览",
                        key: "preview",
                        width: 90,
                        render: (_, asset) =>
                            asset.assetType === "Image" && /^https?:\/\//i.test(asset.url) ? (
                                <Image src={asset.url} width={56} height={56} style={{ objectFit: "cover" }} preview={false} referrerPolicy="no-referrer" />
                            ) : (
                                <span>{asset.assetType || "—"}</span>
                            ),
                    },
                    {
                        title: "名称 / 素材 ID",
                        key: "name",
                        width: 260,
                        render: (_, asset) => (
                            <div>
                                <div>{asset.name || "未命名"}</div>
                                <small className="break-all text-muted-foreground">{asset.id}</small>
                            </div>
                        ),
                    },
                    { title: "素材组 ID", dataIndex: "groupId", width: 220 },
                    {
                        title: "状态",
                        key: "status",
                        width: 160,
                        render: (_, asset) => (
                            <div>
                                <Tag color={asset.status === "Active" ? "success" : asset.status === "Failed" ? "error" : "processing"}>{statusNames[asset.status] || asset.status}</Tag>
                                {asset.error?.message ? <p className="mt-1 text-xs text-red-500">{asset.error.message}</p> : null}
                            </div>
                        ),
                    },
                    { title: "上传时间", dataIndex: "createTime", width: 180, render: (value: string) => (value ? new Date(value).toLocaleString() : "—") },
                    { title: "最近用于生成", dataIndex: "lastInferenceTime", width: 180, render: (value: string) => (value ? new Date(value).toLocaleString() : "无记录") },
                    {
                        title: "操作",
                        key: "actions",
                        width: 160,
                        render: (_, asset) => (
                            <Space>
                                <Button size="small" disabled={busy || query.isError} onClick={() => rename(asset)}>
                                    改名
                                </Button>
                                <Button size="small" danger disabled={busy || query.isError} onClick={() => remove(asset)}>
                                    删除
                                </Button>
                            </Space>
                        ),
                    },
                ]}
            />
            <div className="flex flex-wrap items-center gap-4">
                <span className="text-sm text-muted-foreground">第 {page + 1} 页 · 每页最多 20 条</span>
                <Space>
                    <Button disabled={busy || page === 0 || query.isError} onClick={() => setPage(page - 1)}>
                        上一页
                    </Button>
                    <Button
                        disabled={busy || !query.data?.nextToken || query.isError}
                        onClick={() => {
                            setTokens([...tokens.slice(0, page + 1), query.data!.nextToken]);
                            setPage(page + 1);
                        }}
                    >
                        下一页
                    </Button>
                </Space>
            </div>
        </div>
    );
}
