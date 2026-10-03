# Vue 前端

当前能力与剩余验收统一见 [项目当前状态](../docs/project-status.md)。

## 每日推荐 API

音乐页请求同源 `GET /api/v1/music/recommendations/today`，使用后端返回的上海日期及曲目快照，不再调用本地抽选函数。音乐库从后端分页读取；本地歌单文件保留用于既有兼容逻辑。当前推荐可被搜索、选择和收藏。

开发时先按 `../backend/README.md` 启动正式后端及 PostgreSQL，再运行 `pnpm dev`；Vite 将 `/api` 转发到 `http://127.0.0.1:8081`。生产环境需由反向代理将同域 `/api` 转发到 Go 服务（Vite 的开发代理不随构建发布）。

页面区分加载中、空推荐、尚未生成和请求失败，10 秒超时，可手动重试；页面可见时每 30 秒检查上海日期并重试失败请求，跨日重新读取推荐。`pnpm test:music` 验证接口解析与错误处理。后端未启动或尚无当天推荐时会显示真实状态。

## 文章维护

博客已接入 Go API：`/blog` 为公开列表，`/blog/:slug` 为详情，`/admin` 提供站长登录、草稿创建、Markdown 编辑预览、保存和发布。搜索匹配标题/摘要，分类和标签使用后端 slug，列表每页 20 篇。错误和空列表直接显示真实状态，不回退本地示例。

原 `src/content/posts/*.md` 保留并继续校验，但不再作为公开页面数据源，也不会自动导入数据库。

### 本地闭环

1. 按 [后端说明](../backend/README.md) 配置 PostgreSQL、迁移并创建站长账号，以 `DEMO_MODE=false` 启动 Go API。内存 Demo 不支持后台。
2. 在 frontend 运行 `npm run dev -- --host localhost --port 5173 --strictPort`，打开 `http://localhost:5173/admin`。后端 `ALLOWED_ORIGIN` 须为相同的 `http://localhost:5173`，不能混用 127.0.0.1 或其他端口。
3. 登录 → 新建文章 → 输入标题、唯一 slug、Markdown → 保存文章。此时公开地址应为 404。
4. 点击「确认发布」，确认数据库操作成功及页面的静态更新状态。开发 SPA 可以通过公开 API 查看文章；生产地址直接打开须等静态状态确认已更新，新文章在更新前可能返回 404。
5. 已发布文章保存修改进入修订草稿，再次发布才更新公开版本。修改提交 version，409 时保留输入并提示重新读取。归档保留文章与修订，可再次发布；后台支持按状态筛选。
6. 按 [SEO 流程](../docs/seo-auto-refresh.md) 安装独立 Worker/timer 后，发布、发布修订和归档自动刷新静态目录；保存未发布修订不触发。管理页每 5 秒检查 `/api/v1/admin/seo`，展示待更新、上次失败、已更新或无法确认，发布/归档后立即复查。静态更新完成前，归档文章的旧页面仍可能公开返回。状态请求失败不会改变文章操作成功提示。

Cookie 为 HttpOnly，CSRF token 仅放内存。刷新恢复会话；401/CSRF 失效时重新登录，编辑内容暂留页面。内容不写 localStorage，刷新/关闭前提供未保存提示。分类和标签可留空，也可在后台新增、编辑或删除；被文章或修订引用的项目不能删除。

### 持续播放与进度定位

播放器由 App 提供，切换路由继续播放，离开音乐页后显示迷你播放器。有可用时长时可拖动进度，通过新会话与 FFmpeg 服务端定位；不使用 Range/206。刷新或关闭页面不会保持播放。详见 [播放实施记录](../docs/music-public-playback-implementation-20261002.md)。

### 验证与部署

运行 `npm run test:blog`、`npm run test:music`、`npm run test:seo` 和 `npm run type-check`。`npm run test:seo:build` 使用本机临时 API 验证真实 production 构建中的发布、更新、归档及过期构建拒绝，保留所有输出；不替代 PostgreSQL/Linux/systemd 验收。正式构建需先配置环境和新输出目录。test:blog 模拟 HTTP 响应，验证登录/CSRF、创建/编辑/发布/公开访问的请求契约、版本号、筛选分页、异常/取消和安全 Markdown 渲染。未配置数据库时 PostgreSQL 集成测试会 SKIP。

生产环境将同域 `/api/` 代理到 Go，保留 Origin 和 Cookie；前端使用构建生成的页面 HTML，未知路径返回 404。正式构建需要 VITE_SITE_URL、SEO_API_ORIGIN 和新的输出目录，详细步骤见 [SEO 与链接分享](../docs/seo-and-sharing.md)。后端配置 HTTPS ALLOWED_ORIGIN、APP_ENV=production 和 Secure Cookie。Vite dev proxy 不随构建部署。

构建已设置 `emptyOutDir: false`，保留旧 dist 输出；正式发布切换整个新目录，防止旧文章 HTML 残留在公开目录。旧产物保留，清理须由站长确认。

## 编辑器与工具参考

以下保留项目初始工具说明。

## Recommended IDE Setup

[VS Code](https://code.visualstudio.com/) + [Vue (Official)](https://marketplace.visualstudio.com/items?itemName=Vue.volar) (and disable Vetur).

## Recommended Browser Setup

- Chromium-based browsers (Chrome, Edge, Brave, etc.):
  - [Vue.js devtools](https://chromewebstore.google.com/detail/vuejs-devtools/nhdogjmejiglipccpnnnanhbledajbpd)
  - [Turn on Custom Object Formatter in Chrome DevTools](http://bit.ly/object-formatters)
- Firefox:
  - [Vue.js devtools](https://addons.mozilla.org/en-US/firefox/addon/vue-js-devtools/)
  - [Turn on Custom Object Formatter in Firefox DevTools](https://fxdx.dev/firefox-devtools-custom-object-formatters/)

## Type Support for `.vue` Imports in TS

TypeScript cannot handle type information for `.vue` imports by default, so we replace the `tsc` CLI with `vue-tsc` for type checking. In editors, we need [Volar](https://marketplace.visualstudio.com/items?itemName=Vue.volar) to make the TypeScript language service aware of `.vue` types.

## Customize configuration

See [Vite Configuration Reference](https://vite.dev/config/).

## Project Setup

```sh
pnpm install
```

### Compile and Hot-Reload for Development

```sh
pnpm dev
```

### Type-Check, Compile and Minify for Production

配置 VITE_SITE_URL、SEO_API_ORIGIN 后，使用新的空输出目录（每次发布更换目录名）：

```sh
pnpm build --outDir dist/releases/release-001
```

没有 API 的本地预览构建见 [SEO 文档](../docs/seo-and-sharing.md)。

## 历史分支迁移（2026-09-27）

> 以下记录当时的协作方式；当前整理基线已在同一 checkout 包含前后端，不要求使用两个 worktree。

前端在 `feat/frontend` 分支维护，Go 服务在 `feat/backend` 分支维护。启动联调时分别使用两个 worktree；本目录的 Vite 将 `/api` 代理至 `http://127.0.0.1:8081`。

- `/admin`：Go API 站长登录、文章编辑与发布。
- `/admin/local`：保留原本的本地草稿、预览与 Markdown 导出；从文章管理页进入，浏览器草稿 key 不变。
- `/about` 和音乐页人物布局保留。音乐推荐使用当前 Go API，同时保留开发环境显式 local fallback、超时取消及收藏快照。
- `npm run test:music` 同时执行两分支的音乐测试。
