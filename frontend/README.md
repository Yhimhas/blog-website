# vue-project

## 每日推荐 API

音乐页请求同源 `GET /api/v1/music/recommendations/today`，使用后端返回的上海日期及曲目快照，不再调用本地抽选函数。音乐库仍保留本地歌单，当前推荐也可被搜索、选择和收藏。

开发时先按 `../backend/README.md` 启动正式后端及 PostgreSQL，再运行 `pnpm dev`；Vite 将 `/api` 转发到 `http://127.0.0.1:8081`。生产环境需由反向代理将同域 `/api` 转发到 Go 服务（Vite 的开发代理不随构建发布）。

页面区分加载中、空推荐、尚未生成和请求失败，10 秒超时，可手动重试；页面可见时每 30 秒检查上海日期并重试失败请求，跨日重新读取推荐。`pnpm test:music` 验证接口解析与错误处理。后端未启动或尚无当天推荐时会显示真实状态。

## 文章维护

博客已接入 Go API：`/blog` 为公开列表，`/blog/:slug` 为详情，`/admin` 提供站长登录、草稿创建、Markdown 编辑预览、保存和发布。搜索匹配标题/摘要，分类和标签使用后端 slug，列表每页 20 篇。错误和空列表直接显示真实状态，不回退本地示例。

原 `src/content/posts/*.md` 保留并继续校验，但不再作为公开页面数据源，也不会自动导入数据库。

### 本地闭环

1. 按 [后端说明](../backend/README.md) 配置 PostgreSQL、迁移并创建站长账号，以 `DEMO_MODE=false` 启动 Go API。内存 Demo 不支持后台。
2. 在 frontend 运行 `npm run dev -- --host localhost --port 5173 --strictPort`，打开 `http://localhost:5173/admin`。后端 `ALLOWED_ORIGIN` 须为相同的 `http://localhost:5173`，不能混用 127.0.0.1 或其他端口。
3. 登录 → 新建文章 → 输入标题、唯一 slug、Markdown → 保存文章。此时公开地址应为 404。
4. 点击「保存并发布」，再点击公开访问链接。无登录浏览器也应能访问 `/blog/<slug>`，并在列表找到文章。
5. 已发布文章保存后立即更新公开内容；API 尚无独立的发布后修订草稿。修改提交 version，409 时保留输入并提示重新读取。重新读取会确认是否放弃未保存输入。

Cookie 为 HttpOnly，CSRF token 仅放内存。刷新恢复会话；401/CSRF 失效时重新登录，编辑内容暂留页面。内容不写 localStorage，刷新/关闭前提供未保存提示。分类和标签可留空，目前通过后端维护，页面不提供创建。

### 验证与部署

运行 `npm run test:blog` 和 `npm run build`（包含类型检查）。test:blog 模拟 HTTP 响应，验证登录/CSRF、创建/编辑/发布/公开访问的请求契约、版本号、筛选分页、异常/取消和安全 Markdown 渲染；不等于真实数据库或浏览器端到端验收。未配置数据库时 PostgreSQL 集成测试会 SKIP。

生产环境将同域 `/api/` 代理到 Go，保留 Origin 和 Cookie；前端其他路径使用 SPA fallback 到 index.html，确保刷新 `/admin` 和 `/blog/<slug>` 可用。后端配置 HTTPS ALLOWED_ORIGIN、APP_ENV=production 和 Secure Cookie。Vite dev proxy 不随构建部署。

构建已设置 `emptyOutDir: false`，保留旧 dist 输出；旧 hash 资源可能累积，发布时使用当前 index.html 引用的资源，清理须由站长确认。

This template should help get you started developing with Vue 3 in Vite.

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

```sh
pnpm build
```
