# Go 后端开发与前后端协作手册

更新：2026-09-21。状态：待你实现的开发契约，不代表 API 已存在。

## 1. 当前基线与分工

实际仓库为 `D:/blog-website/blog-website`。前端位于 `frontend/`，使用 Vue 3、TypeScript、Vite、Vue Router 和 FIELD UI；没有执行早期方案中的 Nuxt 迁移。已有 `/`、`/blog`、`/blog/:id`、`/music` 及 404 页面。博客读取 `src/content.ts`，音乐读取本地歌单 JSON，收藏保存在浏览器；音乐仍是官方 iframe，纯音频尚未实现。当前没有 backend 工程、数据库或管理页面。

从现在开始：助手负责前端实现、交互、API 适配与前端验证；你负责 Go 后端实现，同时学习语言与数据库；助手提供设计、讲解、定位和 Review。旧学习计划中“所有实现都由你完成”和前端目录描述以本说明为准。保留旧文档作为历史，不删除文件。

第一阶段沿用 Go + Gin + PostgreSQL + GORM + SQL 版本化迁移的原规划。先用 Go 标准库理解 HTTP，再引入 Gin。不迁移当前前端、不引入微服务。SSR/SEO、管理页面、纯音频分别作为后续交付。

## 2. 建议目录与请求流

以下目录由你逐步创建，不需要第一天全部建好：

```text
backend/
  cmd/api/main.go             # 配置、依赖装配、启动和退出
  internal/platform/         # 配置、数据库、日志、HTTP 中间件
  internal/blog/             # handler、service、repository、DTO
  internal/auth/             # 单站长会话
  internal/music/provider/   # 来源适配器
  internal/recommendation/   # 每日推荐规则
  migrations/                # 编号 SQL；不启动时隐式改表
  go.mod
api/openapi.yaml             # 开始联调前将下面的契约转为 OpenAPI
```

一次请求：浏览器 → handler（参数、状态码）→ service（发布规则）→ repository（参数化查询）→ PostgreSQL。只有真的需要替换存储或做测试时才提取小 interface；不要先写一套通用框架。数据库模型不直接作为公开 JSON 返回。

## 3. 统一 HTTP 约定

- 路径前缀 `/api/v1`；JSON 字段使用 camelCase。ID 为字符串；时间戳使用 UTC RFC3339，日期为 `YYYY-MM-DD`。前端负责显示本地日期。
- 成功返回 `{ "data": ... }`；列表另带 `pagination: { page, pageSize, total }`。`page` 从 1 开始，`pageSize` 默认 20，上限 50；非法值返回 400，合法但超出末页返回空数组。
- 失败返回 `{ "error": { "code": "POST_NOT_FOUND", "message": "文章不存在" }, "requestId": "..." }`。日志使用同一 requestId；不向用户输出 SQL、堆栈或凭据。
- 400 参数错误；401 未登录；403 权限或 CSRF 失败；404 资源不存在（公开查询草稿也返回 404）；409 slug/版本冲突；429 限流；500 内部错误；503 暂时不可用。
- 正文设长度上限；SQL 使用绑定参数；handler 将请求 context 传到数据库和外部请求。客户端断开后停止无用工作。

## 4. 第一批接口：先让博客联通

| 方法与路径 | 输入 | 响应与规则 |
| --- | --- | --- |
| `GET /api/v1/health` | 无 | `data.status = "ok"`，存活检查，不暴露配置 |
| `GET /api/v1/ready` | 无 | 数据库可用为 200，否则 503 |
| `GET /api/v1/posts` | `page,pageSize,q,category,tag` | 已发布文章摘要列表；按 publishedAt 降序、id 降序稳定排序 |
| `GET /api/v1/posts/{slug}` | URL 中的 slug | 已发布文章详情，未找到为 404 |
| `GET /api/v1/categories` | 无 | `data: [{id,slug,name}]`，按 name、id 排序 |
| `GET /api/v1/tags` | 无 | `data: [{id,slug,name}]`，按 name、id 排序 |

`q` 去除首尾空白后最多 100 字符，第一版匹配标题和摘要；category/tag 接收 slug，空值表示不筛选，未知 slug 返回空列表。标签与分类过滤和关键词为 AND 关系。使用参数化 ILIKE，并转义 `%`、`_`，使关键词按字面匹配。

文章列表示例：

```json
{
  "data": [{
    "id": "post-001",
    "slug": "building",
    "title": "从一个组件开始",
    "summary": "记录网站开发过程。",
    "category": { "id": "cat-001", "slug": "development", "name": "开发笔记" },
    "tags": [{ "id": "tag-001", "slug": "go", "name": "Go" }],
    "publishedAt": "2026-09-21T02:00:00Z",
    "updatedAt": "2026-09-21T02:00:00Z"
  }],
  "pagination": { "page": 1, "pageSize": 20, "total": 1 }
}
```

详情 `data` 包含上述字段并增加 `contentMarkdown`，不在列表传正文。分类可为 null，tags 为空时返回 `[]`。公开响应不含草稿状态、内部备注或管理账号。

前端适配由助手完成：当前路由 `:id` 的值将对应 API slug；category.name 映射展示名称，tags 映射名称数组，publishedAt 格式化为日期。当前 `body: [{title,text}]` 只是示例结构，不能假装能无损表达 Markdown；正式接入时替换为禁用原始 HTML并清洗输出的 Markdown 阅读组件。后端失败时显示错误和重试，不能静默回退示例内容冒充真实文章。

## 5. 数据库与文章写入闭环

| 表 | 必要字段与约束 |
| --- | --- |
| posts | id PK、slug UNIQUE NOT NULL、title、summary、content_markdown、category_id 可空 FK、status、published_at 可空、created_at、updated_at、version |
| categories / tags | id PK、slug UNIQUE、name |
| post_tags | post_id + tag_id 联合主键和外键 |
| admin_users | id、username UNIQUE、password_hash、created_at |
| sessions | token_hash UNIQUE、admin_id FK、expires_at、created_at |

status 使用 CHECK 限制为 draft/published/archived；published 状态必须有 published_at。列表查询考虑 `(status,published_at,id)` 索引。标签关联与文章写入放同一事务。slug 建议限制 `[a-z0-9]+(-[a-z0-9]+)*`，最长 100 字符；标题 1–160 字符、摘要最多 500 字符、Markdown 最多 200 KiB。长度单位写入 OpenAPI，服务端统一校验。

| 管理接口（全部需会话，写入需 CSRF/Origin 校验） | 行为 |
| --- | --- |
| `POST /api/v1/admin/session` | username/password 登录，成功 200，设置会话 Cookie，不在 JSON 返回 token |
| `GET /api/v1/admin/session` | 当前站长和 CSRF token；未登录 401 |
| `POST /api/v1/admin/session/logout` | 使服务端会话失效并过期 Cookie，成功 204 |
| `GET /api/v1/admin/posts` | 分页查询，可按 status 过滤 |
| `GET /api/v1/admin/posts/{id}` | 获取草稿或已发布内容供编辑/预览 |
| `POST /api/v1/admin/posts` | 创建 draft，返回 201 和完整管理 DTO |
| `PATCH /api/v1/admin/posts/{id}` | 更新 title/summary/contentMarkdown/categoryId/tagIds；提交 version，过期版本返回 409 |
| `POST /api/v1/admin/posts/{id}/publish` | 校验内容，首次发布设置发布时间；重复发布保持发布时间 |
| `POST /api/v1/admin/posts/{id}/archive` | 归档后公开接口不可见，不删除记录 |

管理 DTO 使用 categoryId、tagIds、status、version，加上文章内容与时间字段。PATCH 中省略表示不变，categoryId 为 null 表示清空分类，tagIds 为 [] 表示清空标签；其他字段不接受 null。发布/归档也提交 version 并原子递增，防止旧编辑覆盖新状态。归档文章可重新发布，保留原首次发布时间。

单站长不开放注册。密码使用成熟哈希实现；随机会话 token 的哈希存数据库，配置有效期。生产 Cookie 使用 HttpOnly、Secure、SameSite=Lax、Path=/，开发 HTTP 的 Secure 行为单独配置。登录也校验 Origin 并限流；管理写入检查允许的 Origin 和 `X-CSRF-Token`。会话响应与管理接口使用 no-store。种子账号通过本机管理命令创建，不提交明文密码。

## 6. 音乐：先元数据，再真实音频

沿用 [纯音频流式方案](music-audio-streaming-design.md)，这份手册不把 iframe 成功等同于音频服务成功。

第一阶段公开只读接口：

| 路径 | data 内容 |
| --- | --- |
| `GET /api/v1/music/playlists` | 歌单数组：id/provider/title/sourceUrl/syncedAt/trackCount |
| `GET /api/v1/music/playlists/{id}/tracks` | 分页曲目：id/provider/externalId/partId/title/author/sourceUrl/durationSeconds/availability/embedUrl |
| `GET /api/v1/music/recommendations/today` | date/timezone/items/status；当天尚未生成返回 503，已生成但无候选返回 200 + 空 items |
| `GET /api/v1/music/recommendations?date=YYYY-MM-DD` | 只允许今天及前 30 个自然日，缺失记录返回 404，不临时重抽 |

provider 为 bilibili/netease；durationSeconds、partId、embedUrl 无可靠值时为 null。availability 为 available/unavailable/unknown，表示来源可用性，不承诺纯音频可播。embedUrl 仅允许已校验的官方地址。前端将 author 映射为当前 artist、sourceUrl 映射 url；来源 URL、临时音频 URL 和播放器 URL 是不同字段。

数据表采用 music_sources、music_items、source_items；曲目唯一标识包含 provider/external_id/part_key，part_key NOT NULL（无分 P 使用固定空字符串），避免 PostgreSQL NULL 唯一约束产生重复。同步完整成功后事务更新关联，失败保留旧快照和 syncedAt；不存在于新快照的条目标记失效，不删除文件或静默清空曲库。

每日推荐按 Asia/Shanghai 业务日期，默认 3 首；用事务级锁与日期唯一约束保存主记录及子项，重启只补今天。空曲库允许完整保存空结果；规则和可控随机源写纯函数测试。收藏仍保存在浏览器，第一版不增加访客账号接口。

纯音频为后续独立里程碑：先验证一首公开曲目，之后实现流式方案里的 playback-sessions、streams、stop 和状态接口；明确 Range、取消、超时、限流和无媒体落盘。不能用模拟时间或假音频 URL 宣布联调完成。

## 7. 边学 Go 边实现的顺序

每次约 2 小时：20 分钟读概念，70 分钟写代码，20 分钟运行验证，10 分钟记录问题。以下是里程碑，按掌握程度分多天完成。

| 阶段 | 你写什么 | Go 学习重点 | 完成标准 |
| --- | --- | --- | --- |
| B01 | health 与内存文章列表 | struct、slice、函数、encoding/json、net/http | 能解释请求→JSON；错误路径返回正确状态码 |
| B02 | Gin 路由与参数验证 | error、指针、方法、context | 分页非法/空结果/找不到都有明确结果 |
| B03 | PostgreSQL 和迁移 | SQL、资源关闭、事务、依赖注入 | 重启数据仍在；只读公开查询排除草稿 |
| B04 | 发布与标签事务 | interface 按需使用、表驱动测试 | 发布/归档/唯一冲突/事务回滚经过验证 |
| B05 | 登录、会话、管理接口 | 中间件、时间、随机数、哈希 | 未登录不可修改；CSRF 与过期会话被拒绝 |
| B06 | 博客前端联调 | HTTP 调试、日志、DTO | 加载/成功/空/404/错误/重试均可演示 |
| B07 | 音乐元数据与同步 | 外部 HTTP、超时、取消 | 同步失败保留上次完整结果 |
| B08 | 每日推荐 | 纯函数、并发、事务锁、时区 | 同日并发只产生一组完整推荐 |
| B09 | 单曲音频 PoC | io.Reader、流、进程生命周期 | 真正出声；切歌与断开释放资源 |
| B10 | 部署与恢复 | 配置、信号、优雅退出 | 健康检查、备份恢复、日志可定位故障 |

第一天只做 B01：自行建立 go.mod 和 main.go，写 health，再返回两篇内存文章；通过 curl.exe 观察状态码、Content-Type 和 JSON。完成后发来代码与结果，我先讲问题和修改方向，让你亲手改后端。

## 8. 本地联调、测试与交付

建议 Go 监听 `127.0.0.1:8081`（启动前检查占用），前端沿用 Vite。正式联调时由助手为 Vite 配 `/api` 代理到 Go；浏览器始终访问同源 `/api/v1`，不在页面写数据库地址。生产 Nginx 把 `/api/v1` 交给 Go，其余路径回退 SPA index.html；未知 API 不能回退 HTML。此处都是待实施配置。

后端配置建议：APP_ENV、HTTP_ADDR、DATABASE_URL、ALLOWED_ORIGIN、SESSION_TTL；凭据不进入 VITE_*、Git 或日志。Go 和依赖版本在你开始建工程时核实并固定，这里不预设已安装版本。

你每次交付：接口路径、请求/响应示例、启动命令、实际 go test ./... 结果、未完成项。我负责对应前端接入、取消旧请求、防止搜索竞态、加载/空/错误状态及浏览器验收。契约变更先同步本文和 OpenAPI，再修改调用方。

后端重点测试：公开草稿隔离、PATCH 未传与空值区别、发布版本冲突、事务失败回滚、来源超时保留数据、推荐同日并发和跨日、流取消释放资源。数据库测试使用独立库；不对生产库运行破坏性测试。

前端检查：`pnpm run type-check`，`pnpm run build-only -- --emptyOutDir=false`，以及桌面/手机导航、返回、深链接、键盘焦点。构建保留旧产物，不自动清理。新增测试资源和无用残留在每次交付时列出。

## 9. 音乐页现状补充（2026-09-21）

仅 `/music` 采用用户视频参考的斜切菜单、高对比黑白与蓝/粉配色。发现、音乐库、收藏使用独立背景；用户可在三个预设中切换，各栏目偏好保存在 `yhimhas:music-backgrounds:v1`。收藏沿用 `lin-music-favorites`，没有新增后端写接口。

“发现”的每日推荐目前由 `frontend/src/musicDaily.ts` 从已收录真实曲目生成：曲目 ID 去重，对 `Asia/Shanghai` 日期与 ID 做稳定 hash 排序，取前 3 条；同日期、同曲库结果相同，源列表重排不影响结果。曲库不足则按实际数量返回；曲库内容变更可能改变当天结果。这是可运行的前端过渡方案，没有调用尚未实现的 Go API，也没有实现后端的近 7 天排除规则。页面每 30 秒及恢复可见时检查日期，播放器已选曲目不会随日期更新而被替换。

你完成 `GET /api/v1/music/recommendations/today` 后，由助手将本地 dailySelection 替换为接口数据；生产环境以后端保存的结果与业务日期为准，不用客户端当前时间重新抽取。建议响应为 `data: { date, timezone: "Asia/Shanghai", status: "ready", items: [...] }`，items 使用第 6 节曲目 DTO，并携带 playlistId。后端默认数量 3，按日期事务保存；前端需要 loading/error/empty 三种状态，失败不能悄悄切回本地推荐。

播放仍使用真实官方 iframe 和原平台链接，页面不模拟播放进度；打开播放器不等于平台已经成功播放。iframe 放在音乐栏目过渡之外，切换栏目或背景保留当前实例，离开 `/music` 则卸载。纯音频仍按独立设计文档后续实现。

## 10. 文档导航

- [原始开发方案](personal-website-development-plan.md)：总体业务与部署背景。
- [旧每日学习计划](daily-development-learning-plan.md)：保留概念参考，分工与前端技术栈以本文为准。
- [代码自查清单](code-notes-and-review-checklist.md)：后端 Review 依据。
- [音频流式设计](music-audio-streaming-design.md)：纯音频阶段的实现和验收细节。
