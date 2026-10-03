# SEO 与链接分享

> 后续操作统一使用 pnpm，见 [包管理约定](package-manager.md)。下文验收部分的 npm 命令保留为当时执行记录。

当前实现是 Vue + Vite SPA，没有引入 Nuxt SSR。构建插件从 Go 的公开文章 API 分页读取全部文章，生成首页、博客、音乐、关于和每篇文章的 HTML 快照。首个 HTTP 响应包含 title、description、canonical、Open Graph、Twitter Card；文章还有 BlogPosting JSON-LD、摘要和安全渲染的 Markdown 正文。Vue 挂载后接管页面，站内导航同步更新同一组元信息。文章发布、发布修订和归档由数据库持久记录，启用 [SEO Worker](seo-auto-refresh.md) 后自动重建并切换目录。

分享图片复用 `frontend/public/yhimhas-logo.jpg`，使用适合方形标志的 summary 卡片。canonical 和分享图片使用配置的公开站点 origin，去掉列表筛选参数和 fragment。`/login`、`/admin`、`/admin/local`、未知页面和文章加载失败状态设为 noindex，sitemap 仅包含公开页面与当前公开文章，文章 lastmod 使用 API 的 updatedAt。robots 允许爬虫访问登录等页面以读取 noindex，不将 robots 当作访问权限控制。

## 构建

正式构建必须配置以下两个环境变量，可通过进程环境或 Vite 的 `.env.production.local` 提供：

- `VITE_SITE_URL`：正式站点 origin，例如 `https://your-domain.example`，不能含子路径、凭据或参数。该变量会进入前端。
- `SEO_API_ORIGIN`：构建机器可访问的 Go API origin，例如 `http://127.0.0.1:8081`。插件只读取无凭据公开 API，不读取后台、草稿或本地示例 Markdown。该变量仅用于构建。API 必须迁移到 `000006_seo_publication` 并提供 `/api/v1/seo/revision`；旧 API 和内存 Demo 不能用于正式构建。

在 `frontend/` 执行，输出目录名按本次发布编号设置：

```sh
pnpm run test:seo
pnpm run build --outDir dist/releases/release-001
```

必须使用新的空输出目录，不能把新文件覆盖发布到包含旧 HTML 的目录。否则已下架的文章可能继续被抓取。插件发现目标目录非空时会拒绝正式构建；发布失败也应换新目录重试。不会删除旧目录。API 超时、请求失败、无效详情、重复 slug 或分页不完整都会中止构建。读取开始、结束和文件全部写完后检查公开版本，能检测总数不变的修改；正式输出包含记录 sourceId、字符串 revision、origin、preview 和文件清单的 `seo-release.json`。手工构建用于检查，正式切换统一交给 Worker，避免最终检查与切换之间出现新提交。

仅本地验证、没有 API 时可以运行：

```sh
pnpm run build --mode development --outDir dist/seo-preview
```

该模式输出 noindex、禁止抓取的 robots 和空 sitemap，不读取本地示例文章，不能作为正式发布。`vite preview` 仅检查静态产物；正式 HTTP 状态码与重定向须使用实际 Web 服务器验收。

## 部署与内容更新

将 [Nginx 路由片段](../deploy/seo.nginx.conf) 合并到现有 server，root 指向 `SEO_CURRENT_LINK`，`/assets/` 指向共享不可变资源库，保留原来的 API 和媒体代理。按实际 Nginx prefix 修改片段的 `seo-site` 路径。HTML/404 关闭 `open_file_cache`，公开文章 API 和版本接口不能缓存。公开 URL 直接命中各自的 index.html；未知路径返回 HTTP 404 和 404.html。登录与管理页也生成入口 HTML。

安装步骤、持久触发、失败重试、并发保护、目录切换、状态查看及回退统一见 [SEO 自动刷新操作说明](seo-auto-refresh.md)。构建期间可继续编辑和发布；发生公开版本变化时放弃过期构建，后续任务收敛。保存未发布修订不触发刷新。静态刷新存在构建耗时与调度延迟；下架涉及敏感内容时还需核验静态目录与 CDN 缓存。此方案是构建快照，不是实时 SSR。

后台文章管理显示独立的静态更新状态：待更新、上次更新失败、已更新或无法确认。发布/归档成功只确认数据库操作；随后立即复查状态并每 5 秒自动检查，只有服务器确认版本一致且无错误才显示静态页已更新。Worker 未启用时会保持待更新，新文章可能直接访问 404，归档旧 HTML 仍可能返回；状态接口不可用时也不会提示已撤回。页面「刷新静态状态」只重新读取状态，不会手工启动部署。

部署后验证：

1. 使用不运行 JavaScript 的 HTTP 客户端读取 `/blog/<slug>`，检查文章标题、摘要、canonical、OG、JSON-LD 和正文。
2. 检查 `/sitemap.xml`、`/robots.txt` 和分享图片返回正确内容类型及公开可访问地址。
3. 检查未知文章返回 HTTP 404；登录、管理页包含 noindex；站内文章切换和返回首页没有残留文章标签。
4. 完成发布、保存修订（静态仍为旧版）、发布修订、归档闭环；等待 revision=appliedRevision，确认正文、sitemap 和归档地址 404。构建中再次发布，确认过期产物不改变 current，后续任务收敛；停启任务验证 pending 不丢失。
5. 向搜索平台提交 sitemap，按各分享平台的缓存刷新机制验收链接预览。标签与 sitemap 不保证收录或立即更新平台缓存。

实现依据：[Google JavaScript SEO](https://developers.google.com/search/docs/crawling-indexing/javascript/javascript-seo-basics)、[Open Graph protocol](https://ogp.me/)。

## 最新验证

2026-10-03 的自动刷新验证、未执行范围与保留产物见 [刷新验收记录](seo-auto-refresh.md)。下文为历史记录，不代表当前版本生产环境已验收。

## 历史验证（2026-10-02）

- `npm run type-check`、`npm run test:seo`（5 项）、`npm run test:blog`（7 项）通过。
- 本地 development 构建与使用临时公开 API 的 production 构建通过；已检查正式产物的文章正文、唯一 title、canonical、JSON-LD、sitemap、私有页面 noindex、JS 资源入口和分享图片。
- 浏览器验证通过：文章首开、返回博客列表时清除文章时间与 JSON-LD、重新进入文章恢复 article 标签、不存在文章显示 noindex 且无 canonical。
- 临时 API 与浏览器验收使用测试文章和示例域名，未连接生产数据库、未部署服务器；真实 Nginx 状态码、搜索平台收录和分享平台缓存尚未验收。

未删除文件。保留的验证产物为 `frontend/dist/seo-preview/`、`frontend/dist/seo-validation-1790938719730/`；均非正式发布目录，可在人工确认后处理。类型检查还更新了既有 `frontend/node_modules/.tmp/` 增量缓存。临时验证服务已停止。
