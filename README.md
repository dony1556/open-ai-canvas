<p align="center">
  <img src="web/public/logo.svg" width="88" alt="OCTV-AIGC logo">
</p>

<h1 align="center">OCTV-AIGC</h1>

## 核心能力

- **自由画布**：节点、连线、框选、缩放、小地图、撤销重做、导入导出和只读分享。
- **影视创作工作流**：剧本、角色、场景、风格板、参考素材、结构化分镜和 3D 导演台。
- **多媒体生成**：文本、图片、视频、音频任务，支持参考图、首尾帧、运镜、续写、局部修改和批量生成。
- **任务与素材管理**：异步队列、进度与日志、取消/重试、素材库、资源引用校验和登录后的跨设备同步。
- **时间线剪辑**：片段编排、拆分、修剪、字幕转写和服务端成片导出，并支持插件化编辑面板。
- **云端 Agent**：支持持久化对话、画布摘要和流式事件回放；当前为只读阶段，真实环境能力以文档和验收清单为准。
- **Agent 技能生态**：内置技能覆盖生产流程；社区技能库 [judian-skills](https://github.com/itsWyatt-K/judian-skills) 提供 34 个场景域包索引，完整卡片通过 GitHub 安装后按需读取。安装：技能页 → 安装技能 → GitHub 标签 → 仓库地址填 `https://github.com/itsWyatt-K/judian-skills`，子目录填 `skills/drama/story-structure-engine` 这类域包路径。
- **管理与渠道**：系统渠道、逻辑模型、用量/积分、功能开关、对象存储、响应拦截和管理后台。

完整功能以[功能清单](docs/content/docs/overview/features.mdx)为准。

## 快速开始

### 环境要求

- [Bun](https://bun.sh/)：前端和文档站
- [Go 1.25](https://go.dev/)：后端
- Docker Compose：仅在使用容器开发或部署时需要

### 宿主机启动

```bash
git clone https://github.com/ddcat-ai/open-ai-canvas.git
cd open-ai-canvas

# 使用 Git 忽略的目录保存本地开发数据和缓存
mkdir -p .local/project-workbench-debug .local/cache/go-build .local/cache/go-mod

# 终端一：后端
cd backend
CANVAS_BACKEND_ADDR=127.0.0.1:8080 \
CANVAS_BACKEND_DATA_DIR=../.local/project-workbench-debug \
GOCACHE=../.local/cache/go-build \
GOMODCACHE=../.local/cache/go-mod \
go run ./cmd/server

# 终端二：前端
cd ../web
bun install --frozen-lockfile
bun run dev
```

打开 <http://localhost:3000>。首次使用时注册管理员账号，并在设置中配置模型渠道。前端默认将 `/api` 代理到 `http://127.0.0.1:8080`；如需修改代理目标，可设置 `VITE_API_PROXY_TARGET`。

Windows PowerShell 用户可在仓库根目录执行：

```powershell
.\scripts\start-local.ps1
```

### Docker 开发与本地构建

以下命令用于本地开发或试运行。部署服务器并使用后台在线更新，请参考[Docker 部署文档](docs/content/docs/overview/docker.mdx)。

源码热更新：

```bash
LOCAL_UID=$(id -u) LOCAL_GID=$(id -g) \
  docker compose -f docker-compose.dev.yml up --build
```

本地构建并运行 release 镜像：

```bash
docker compose -f docker-compose.local.yml up -d --build
```

热更新开发默认前端端口为 `3000`、后端端口为 `8080`；端口冲突时可通过 `CANVAS_WEB_HOST_PORT` 和 `CANVAS_BACKEND_HOST_PORT` 覆盖。本地构建编排只暴露 web 的 `3000`，后端留在容器网络内，不安装 Host Updater。

更多本地开发说明（包括时间线字幕转写）见[本地开发文档](docs/content/docs/backend/local-development.mdx)。
