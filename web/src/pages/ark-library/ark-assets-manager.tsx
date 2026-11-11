import { Alert, App, Button, Form, Image, Input, Select, Space, Table, Tag } from "antd";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { deleteArkAsset, listExistingArkAssets, listExistingArkGroups, renameArkAsset, type ArkAsset, type ArkAssetQuery } from "@/services/api/ark-assets";

const statusNames: Record<string, string> = { Active: "可用", Processing: "处理中", Failed: "失败" };

export default function ArkAssetsManager({ scope, canManage = false }: { scope: string; canManage?: boolean }) {
    const { message, modal } = App.useApp();
    const [filters, setFilters] = useState<ArkAssetQuery>({ groupType: "AIGC" });
    const [tokens, setTokens] = useState([""]);
    const [page, setPage] = useState(0);
    const [working, setWorking] = useState("");
    const [form] = Form.useForm();
    const groupType = Form.useWatch("groupType", form) || "AIGC";
    const groups = useInfiniteQuery({
        queryKey: ["ark-existing-groups", scope, groupType],
        initialPageParam: "",
        queryFn: ({ pageParam, signal }) => listExistingArkGroups(groupType, pageParam, signal),
        getNextPageParam: (page) => page.nextToken || undefined,
        retry: false,
    });
    const query = useQuery({
        queryKey: ["ark-existing-assets", scope, canManage, filters, tokens[page]],
        queryFn: ({ signal }) => listExistingArkAssets({ ...filters, nextToken: tokens[page] }, signal),
        retry: false,
        staleTime: 0,
    });
    const busy = Boolean(working) || query.isFetching;

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

    return (
        <div className="space-y-4">
            <p className="text-sm text-muted-foreground">直接读取当前方舟项目中的全部素材，包括画布和方舟控制台上传的素材。可见范围由管理员统一配置。</p>
            <Form
                form={form}
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
                    <Select
                        aria-label="筛选素材组"
                        placeholder="全部素材组"
                        allowClear
                        showSearch
                        optionFilterProp="label"
                        style={{ width: 280 }}
                        loading={groups.isFetching}
                        disabled={groups.isError}
                        options={groups.isError ? [] : [...new Map(groups.data?.pages.flatMap((page) => page.items).map((group) => [group.id, { value: group.id, label: `${group.name || "未命名"} (${group.id})` }])).values()]}
                        onPopupScroll={(event) => {
                            const target = event.target as HTMLElement;
                            if (target.scrollTop + target.clientHeight >= target.scrollHeight - 40 && groups.hasNextPage && !groups.isFetchingNextPage) void groups.fetchNextPage();
                        }}
                    />
                </Form.Item>
                <Form.Item name="groupType">
                    <Select
                        aria-label="素材组类型"
                        onChange={() => form.setFieldValue("groupId", undefined)}
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
            {groups.isError && !query.isError ? (
                <Alert type="error" showIcon title="读取素材组失败" description={groups.error instanceof Error ? groups.error.message : "请重试"} action={<Button onClick={() => void groups.refetch()}>重试素材组</Button>} />
            ) : null}
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
                dataSource={query.isError ? [] : query.data?.items || []}
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
                        hidden: !canManage,
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
