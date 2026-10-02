# SEO 与链接分享

当前实现是 Vue + Vite SPA，没有引入 Nuxt SSR。构建插件从 Go 的公开文章 API 分页读取全部文章，生成首页、博客、音乐、关于和每篇文章的 HTML 快照。首个 HTTP 响应包含 title、description、canonical、Open Graph、Twitter Card；文章还有 BlogPosting JSON-LD、摘要和安全渲染的 Markdown 正文。Vue 挂载后接管页面，站内导航同步更新同一组元信息。

分享图片复用 `frontend/public/yhimhas-logo.jpg`，使用适合方形标志的 summary 卡片。canonical 和分享图片使用配置的公开站点 origin，去掉列表筛选参数和 fragment。`/login`、`/admin`、`/admin/local`、未知页面和文章加载失败状态设为 noindex，sitemap 仅包含公开页面与当前公开文章，文章 lastmod 使用 API 的 updatedAt。robots 允许爬虫访问登录等页面以读取 noindex，不将 robots 当作访问权限控制。

## 构建

正式构建必须配置以下两个环境变量，可通过进程环境或 Vite 的 `.env.production.local` 提供：

- `VITE_SITE_URL`：正式站点 origin，例如 `https://your-domain.example`，不能含子路径、凭据或参数。该变量会进入前端。
- `SEO_API_ORIGIN`：构建机器可访问的 Go API origin，例如 `http://127.0.0.1:8081`。插件只读取无凭据公开 API，不读取后台、草稿或本地示例 Markdown。该变量仅用于构建。

在 `frontend/` 执行，输出目录名按本次发布编号设置：

```sh
npm run test:seo
npm run build -- --outDir dist/releases/release-001
```

必须使用新的空输出目录，不能把新文件覆盖发布到包含旧 HTML 的目录。否则已下架的文章可能继续被抓取。插件发现目标目录非空时会拒绝正式构建；发布失败也应换新目录重试。不会删除旧目录。部署时应切换整个发布目录，而不是将多个发布目录合并。API 超时、请求失败、无效详情、重复 slug 或分页不完整都会中止构建，不会静默使用旧快照。

仅本地验证、没有 API 时可以运行：

```sh
npm run build -- --mode development --outDir dist/seo-preview
```

该模式输出 noindex、禁止抓取的 robots 和空 sitemap，不读取本地示例文章，不能作为正式发布。`vite preview` 仅检查静态产物；正式 HTTP 状态码与重定向须使用实际 Web 服务器验收。

## 部署与内容更新

将 [Nginx 路由片段](../deploy/seo.nginx.conf) 合并到现有 server，root 指向新发布目录，保留原来的 API 和媒体代理。公开 URL 应直接命中各自的 index.html；未知路径必须返回 HTTP 404 和 404.html，而不是首页的 200 SPA fallback。登录与管理页也生成入口 HTML，支持直接打开与刷新。

每次公开文章发布、修改、下架后都需要重新构建并切换目录；当前没有自动发布 hook。构建期间应暂停内容发布，避免快照跨越不同版本。下架涉及敏感内容时应同时更新静态发布与 CDN 缓存；只在后台下架无法撤回已发布的静态副本。此方案是构建快照，不是实时 SSR。

部署后验证：

1. 使用不运行 JavaScript 的 HTTP 客户端读取 `/blog/<slug>`，检查文章标题、摘要、canonical、OG、JSON-LD 和正文。
2. 检查 `/sitemap.xml`、`/robots.txt` 和分享图片返回正确内容类型及公开可访问地址。
3. 检查未知文章返回 HTTP 404；登录、管理页包含 noindex；站内文章切换和返回首页没有残留文章标签。
4. 完成一次文章发布、更新、下架及重新部署闭环；确认旧目录不再作为 Web root。
5. 向搜索平台提交 sitemap，按各分享平台的缓存刷新机制验收链接预览。标签与 sitemap 不保证收录或立即更新平台缓存。

实现依据：[Google JavaScript SEO](https://developers.google.com/search/docs/crawling-indexing/javascript/javascript-seo-basics)、[Open Graph protocol](https://ogp.me/)。

## 本次验证（2026-10-02）

- `npm run type-check`、`npm run test:seo`（5 项）、`npm run test:blog`（7 项）通过。
- 本地 development 构建与使用临时公开 API 的 production 构建通过；已检查正式产物的文章正文、唯一 title、canonical、JSON-LD、sitemap、私有页面 noindex、JS 资源入口和分享图片。
- 浏览器验证通过：文章首开、返回博客列表时清除文章时间与 JSON-LD、重新进入文章恢复 article 标签、不存在文章显示 noindex 且无 canonical。
- 临时 API 与浏览器验收使用测试文章和示例域名，未连接生产数据库、未部署服务器；真实 Nginx 状态码、搜索平台收录和分享平台缓存尚未验收。

未删除文件。保留的验证产物为 `frontend/dist/seo-preview/`、`frontend/dist/seo-validation-1790938719730/`；均非正式发布目录，可在人工确认后处理。类型检查还更新了既有 `frontend/node_modules/.tmp/` 增量缓存。临时验证服务已停止。
