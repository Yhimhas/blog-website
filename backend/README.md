# Go 后端

当前能力与证据边界统一见 [项目当前状态](../docs/project-status.md)。当前包含迁移 000006、文章修订与分类标签管理、SEO 自动刷新、完整歌单同步和公开播放。

当前使用你已安装的 **Go 1.27.1**，保留原 `go.mod` 版本修改。正式服务为 Gin + GORM + PostgreSQL，数据库结构使用嵌入的版本化 SQL 和 golang-migrate 显式创建。无自动迁移、无默认管理密码、无静默内存回退。

已实现：公开文章/分类/标签、ready、草稿创建/修改/发布/归档、乐观版本控制、标签事务、站长会话、Origin/CSRF、登录限流、公开歌单/曲目、网易云异步同步与状态查询、人工音乐元数据导入、上海日期的每日推荐与历史。完整字段及权限见 [OpenAPI](../api/openapi.yaml)（JSON 格式也是合法 YAML 1.2）。

本机数据库测试需显式配置独立 TEST_DATABASE_URL，未配置时会 SKIP。此前隔离服务器已验证完整歌单同步、数据库集成与公开样本播放；当前版本生产上线、备份恢复及新增行为的完整验收仍需落实，具体范围见状态文档。

## 启动正式后端

迁移 `000006_seo_publication` 在文章事务中持久记录公开内容版本；发布、发布修订和归档后，由独立 `cmd/seo-refresh` 和 systemd timer 自动重建并原子切换 Linux 静态目录。保存草稿/修订不触发；失败保留旧目录，重启后继续处理 pending。API 本身不启动构建进程。

公开 `GET /api/v1/seo/revision` 返回不可缓存的 sourceId/字符串 revision，管理员 `GET /api/v1/admin/seo` 查看目标/已发布版本、时间及失败信息。安装、互斥、共享资源、源码强制发布及回退见 [SEO 自动刷新操作说明](../docs/seo-auto-refresh.md)。Worker 与 API 必须使用同一数据库。部署模板需在服务器安装启用后才生效。

先安装 PostgreSQL，在本地建立专用 `blog_user` 角色及其拥有的 `blog` 数据库。不要复用生产库进行测试。`.env.example` 仅说明变量，程序不会自动加载 `.env`。

```powershell
cd backend
go version
# 输入自己本机的 PostgreSQL URL；密码不会回显或保存为命令历史字面值。
$env:DATABASE_URL = Read-Host 'PostgreSQL URL' -MaskInput
$env:APP_ENV = 'development'
$env:ALLOWED_ORIGIN = 'http://localhost:5173'
$env:SESSION_TTL = '24h'
$env:COOKIE_SECURE = 'false'
$env:DEMO_MODE = 'false'
go run ./cmd/manage migrate
$env:ADMIN_USERNAME = Read-Host '站长用户名'
$env:ADMIN_PASSWORD = Read-Host '站长密码（12–72 UTF-8 字节）' -MaskInput
go run ./cmd/manage create-admin
$env:ADMIN_PASSWORD = $null
Get-NetTCPConnection -LocalPort 8081 -State Listen -ErrorAction SilentlyContinue
go run ./cmd/api
```

数据库 URL 示例结构为 `postgres://blog_user:<本地密码>@127.0.0.1:5432/blog?sslmode=disable`；密码中特殊字符须 URL 编码。生产必须设置 HTTPS `ALLOWED_ORIGIN`、`APP_ENV=production`，并开启 Secure Cookie；`sslmode=disable` 仅用于可信本地开发连接。Cookie 默认 24h，可设置 1m–720h。密码使用 bcrypt cost 12，数据库仅存密码哈希和随机 session token 的 SHA-256 哈希。

迁移命令仅支持向前 `up`，不提供删除表、回滚或重置命令。迁移失败后先检查 `schema_migrations` 中的 version/dirty 和数据库错误，不自动 force。分类/标签可在 `/admin` 新增、编辑和删除；已被文章或修订引用的项目不能删除。

数据库不可用时 `/health` 仍返回 200，`/ready` 返回 503；未迁移也不能 ready。业务接口返回统一 JSON 错误。服务不会记录 DSN、密码、Cookie 或上游原始响应。

## 调用与写入闭环

```powershell
curl.exe -i http://127.0.0.1:8081/api/v1/health
curl.exe -i http://127.0.0.1:8081/api/v1/ready
curl.exe -i 'http://127.0.0.1:8081/api/v1/posts?page=1&pageSize=20'

# PowerShell 会话保持 Cookie；Origin 必须与配置完全一致。
$base = 'http://127.0.0.1:8081/api/v1'
$headers = @{ Origin = 'http://localhost:5173' }
$credentials = @{ username = $env:ADMIN_USERNAME; password = (Read-Host '密码' -MaskInput) }
Invoke-RestMethod "$base/admin/session" -Method Post -Headers $headers -ContentType 'application/json' -Body ($credentials | ConvertTo-Json) -SessionVariable owner
$credentials = $null
$session = Invoke-RestMethod "$base/admin/session" -WebSession $owner
$headers['X-CSRF-Token'] = $session.data.csrfToken
$payload = @{ slug='hello-backend'; title='后端第一篇文章'; summary='接口闭环验证'; contentMarkdown='# Hello Go'; tagIds=@() } | ConvertTo-Json
$post = Invoke-RestMethod "$base/admin/posts" -Method Post -Headers $headers -WebSession $owner -ContentType 'application/json' -Body ([Text.Encoding]::UTF8.GetBytes($payload))
$publish = @{ version=$post.data.version } | ConvertTo-Json
Invoke-RestMethod "$base/admin/posts/$($post.data.id)/publish" -Method Post -Headers $headers -WebSession $owner -ContentType 'application/json' -Body $publish
Invoke-RestMethod "$base/posts/hello-backend"
```

PATCH 必须提交 version；省略字段不变，`categoryId:null` 清空分类，`tagIds:[]` 清空标签，其他字段拒绝 null。slug 创建后不可修改。标题 1–160 个 Unicode 字符、摘要最多 500 字符、Markdown 最多 200 KiB UTF-8。发布要求正文非空白；归档后再次发布保留首次发布时间。所有状态修改递增 version，旧版本返回 409。

### 日常管理与修订草稿

部署此版本前，在 `backend` 目录执行 `go run ./cmd/manage migrate`，应用 `000005_post_revisions.up.sql` 和 `000006_seo_publication.up.sql`，然后更新后端和前端。新增修订表、SEO 状态与事务 trigger，不改写已有文章；首次 SEO 状态为 pending。未迁移时 `/ready` 返回 503。

后台支持全部、草稿、已发布、已归档筛选。归档停止公开访问，保留文章和待发布修订；重新发布保留首次发布时间。存在未保存输入时须先保存再归档。

已发布文章的 PATCH 只写入独立修订，公开标题、摘要、正文、分类、标签和更新时间不变。管理接口的顶层字段仍为原版本，`revision` 为待发布修订；编辑器优先加载修订。保存和发布共用文章 version，过期写入返回 409。确认发布时在一个事务中应用修订并清除待发布记录；发布失败时保留原公开内容和修订。归档后的修订继续保留并可编辑。这里的审核是管理员预览后手动确认发布，不涉及多角色审批。

分类标签接口为 `POST /admin/categories`、`PUT/DELETE /admin/categories/{id}`，标签将 categories 替换为 tags。写入字段为 name、slug；需要管理员会话及 Origin/CSRF。名称与 slug 修改立即影响公开分类标签，旧 slug 筛选链接不自动重定向。引用保护覆盖草稿、公开文章、归档文章及待发布修订；删除仍被引用的项返回 `409 TERM_IN_USE`。

新增数据库闭环测试：`go test ./internal/storage -run TestBlogManagement -v -count=1`。须先配置独立测试库 `TEST_DATABASE_URL` 和 `ALLOW_TEST_SCHEMA_CREATE=true`；测试保留隔离 schema 供检查。

## 音乐与任务

迁移登记 `netease:595975585`，初始 pending，无伪造曲目。登录后以相同 Cookie/Origin/CSRF 调用 `POST /admin/music/playlists/netease:595975585/sync`，返回 202 和真实 runId，再查询 `GET /admin/music/sync-runs/{runId}`。同来源运行中返回 409。全局最多 2 个同步任务，手动触发最多 4 次/分钟；登录最多 10 次/分钟。

元数据适配器读取完整歌单 ID，分批补齐缺失详情，按原顺序重建并在提交前复查。拒绝访问、超时、不完整详情或复查变化均不替换成功快照；单次最多 2000 首。此链路不携带账号 Cookie，元数据不代表可播。接口和校验细节见 [完整歌单实现](../docs/netease-playlist-completion-implementation-20261002.md)。

快照事务将旧关联标记 inactive，再 upsert 新关联；失败回滚。相同 hash 不重复写曲目。任务状态落库，进程退出取消上游；遗留 running 超过 2 分钟由调度器标记 `SYNC_INTERRUPTED`。首次没有成功快照的曲目 API 固定返回 **200 + 空数组**，以歌单 syncStatus 区分 pending/failed/ready。已失败但有旧快照仍返回旧曲目。

人工验证的网易云或 Bilibili 元数据可通过 `go run ./cmd/manage import-music <本地 JSON 文件>` 导入。格式：

```json
{
  "source": {
    "id": "netease:595975585", "provider": "netease", "externalId": "595975585",
    "title": "我喜欢的音乐", "sourceUrl": "https://music.163.com/playlist?id=595975585", "embedUrl": null
  },
  "tracks": []
}
```

这是格式示例，**执行空 tracks 导入会将此来源现有曲目关联标记 inactive**；导入真实曲库应填写已核验 Track DTO，不要照抄空示例。Track 字段见 OpenAPI。Bilibili `externalId` 使用 BVID，`partId` 使用正整数分 P/cid，id 为 `bilibili:<BVID>:<partId>`；本次没有实现 Bilibili 远端同步。

仅 `availability=available`、启用来源、active 关联且有成功快照的曲目进入推荐；unknown 不参选。默认取 3 首，优先避开近 7 日记录，逐步缩短窗口，尽量避免相邻同作者。无候选也保存完整空结果。推荐保存曲目 DTO 快照，刷新、重启、后续导入不会改写同日推荐。

服务启动只补今天；之后在上海时间 00:10 起执行（分钟级轮询），数据库失败最多尝试 3 次，间隔 1/2 分钟。使用日期唯一约束和事务级 advisory lock 保证多实例幂等。推荐 GET 不临时重抽。可用 `$env:MUSIC_AUTO_SYNC='true'` 显式开启每日来源同步；默认关闭，上线确认真实上游后再启用。开启后生成推荐前先尝试同步，同一来源每天自动尝试最多一次，失败使用旧快照。不补历史日期，不保存媒体。

## 验证

以下 2026-09-25 结果为历史基线，最新证据入口见 [项目当前状态](../docs/project-status.md)。测试命令仍可使用，但必须区分 PASS 和 SKIP。

2026-09-25 本机实际结果：`go test ./...`、`go vet ./...`、`go build ./...` 均通过；OpenAPI 的 21 个操作及本地 `$ref` 引用校验通过。`TestPostgresContracts` 因未配置 `TEST_DATABASE_URL` 明确 SKIP，数据库事务/并发和真实上游仍待运行验收。

```powershell
go test ./...
go vet ./...
go build ./...
# 安装 PostgreSQL 后，用专门测试库验证事务和锁。
$env:TEST_DATABASE_URL = Read-Host '独立测试库 PostgreSQL URL' -MaskInput
$env:ALLOW_TEST_SCHEMA_CREATE = 'true'
go test ./internal/storage -run TestPostgresContracts -v -count=1
```

数据库集成测试覆盖公开草稿隔离、版本竞争、标签失败回滚、会话过期、同步并发、上游失败保留快照、中途写入失败回滚、同日并发推荐与历史不变。未设置测试库时明确 SKIP，不等于通过了数据库验收。每次创建 `backend_test_<随机值>` schema，输出名称并保留，不 DROP，不触及其他 schema。

现有内存演示和测试保留。需要只看旧 B01 演示时设置 `$env:DEMO_MODE='true'` 再启动；演示仅有 health/posts，不能验收数据库或新接口。正式启动请恢复 false。

新增源码、SQL、OpenAPI、测试及 `.env.example` 均为正式项目文件。Go 下载依赖和编译缓存位于 `GOPATH/pkg/mod` 与 `go env GOCACHE` 指示目录，没有删除任何文件；没有生成测试媒体或数据库实例。旧便携工具链说明保留在下方作历史参考。

---

## B01 内存演示参考（启动命令已更新为当前环境）

最初的 B01 演示使用 Go 标准库 `net/http`，当时没有第三方依赖。当前项目已引入 Gin/GORM 等依赖并包含 `go.sum`，module 仍为 `blog-website/backend`，`go.mod` 声明的 Go 版本为 **1.27.1**。

仅在 `DEMO_MODE=true` 时使用内存示例：两篇公开文章 `building`、`hello-go`，以及不可公开读取的草稿 `draft-note`。演示模式重启后重新加载示例，无写入或持久化能力。

## 运行

你已安装 Go 1.27.1 且 PATH 可用，无需重新安装或降级。在仓库根目录运行内存演示（PowerShell）：

```powershell
cd backend
go version
$env:APP_ENV = 'development'
$env:DEMO_MODE = 'true'
# 如有输出，先确认占用进程；服务不会自动结束其他进程。
Get-NetTCPConnection -LocalPort 8081 -State Listen -ErrorAction SilentlyContinue
go run ./cmd/api
```

默认监听 `127.0.0.1:8081`。可在启动前通过 `$env:HTTP_ADDR = '127.0.0.1:8082'` 修改地址；端口占用会报错并退出。Ctrl+C 触发关闭，最多等待 5 秒。访问日志使用 JSON，包含与响应 `X-Request-ID` 相同的 `requestId`。

使用 PATH 中已有 Go，检查版本与实际命中的可执行文件：

```powershell
go version
(Get-Command go).Source
go env GOROOT
```

历史便携工具链 `../../.tools/go1.26.7/go` 仅作为旧文件保留，不应再加入当前 PATH。恢复正式后端前设置 `$env:DEMO_MODE = 'false'`，并按本文上方配置数据库。

## API

| 方法 | 路径 | 行为 |
| --- | --- | --- |
| GET | `/api/v1/health` | 存活检查 |
| GET | `/api/v1/posts` | 已发布文章摘要、筛选与分页 |
| GET | `/api/v1/posts/{slug}` | 已发布文章及 `contentMarkdown`；草稿和不存在的 slug 均为 404 |

新终端执行：

```powershell
curl.exe -i http://127.0.0.1:8081/api/v1/health
curl.exe -i "http://127.0.0.1:8081/api/v1/posts?page=1&pageSize=20&category=development&tag=go"
curl.exe -i "http://127.0.0.1:8081/api/v1/posts?q=hello"
curl.exe -i http://127.0.0.1:8081/api/v1/posts/building
curl.exe -i http://127.0.0.1:8081/api/v1/posts/draft-note
curl.exe -i "http://127.0.0.1:8081/api/v1/posts?page=0"
```

健康检查返回 `200`：

```json
{"data":{"status":"ok"}}
```

筛选 `category=development&tag=go` 返回：

```json
{"data":[{"id":"post-001","slug":"building","title":"从一个组件开始","summary":"记录网站开发过程。","category":{"id":"cat-001","slug":"development","name":"开发笔记"},"tags":[{"id":"tag-001","slug":"go","name":"Go"}],"publishedAt":"2026-09-21T02:00:00Z","updatedAt":"2026-09-21T02:00:00Z"}],"pagination":{"page":1,"pageSize":20,"total":1}}
```

详情返回相同文章字段，并在 `data` 增加 `contentMarkdown`；不带 pagination。草稿和缺失文章返回 `404`：

```json
{"error":{"code":"POST_NOT_FOUND","message":"文章不存在"},"requestId":"每次请求生成的32位十六进制ID"}
```

- `page` 默认 1；`pageSize` 默认 20、最大 50。显式空值、非正整数、整数溢出、重复参数均返回 400 `INVALID_QUERY`。合法的超出末页请求返回 `data: []`，保留筛选后的 `total`。
- `q` 去除首尾空白后最多 100 个 Unicode 字符（不是字节），不区分大小写地匹配标题或摘要。`%`、`_` 按字面匹配。category/tag 使用 slug，与 q 为 AND 关系；空值不筛选，未知 slug 返回空数组。重复筛选参数或非法 UTF-8 返回 400。
- 列表按 `publishedAt` 降序、字符串 `id` 降序；不返回正文或内部状态。时间为 UTC RFC3339，未分类返回 null，无标签返回 `[]`。
- 未知路径返回 JSON 404 `NOT_FOUND`；已知路径的非 GET 方法返回 JSON 405 `METHOD_NOT_ALLOWED` 和 `Allow: GET`。

## 基础验证

在 `backend/` 执行：

```powershell
go test ./...
go vet ./...
```

测试通过 `httptest` 直接验证 HTTP handler，无需先启动服务；覆盖响应格式、requestId、状态码、草稿/归档隔离、正文隔离、排序、分页边界、筛选与 Unicode 参数限制。`internal/blog` 是内存业务规则和 DTO；`internal/platform` 是 HTTP 边界；`cmd/api` 负责装配、配置和服务生命周期。

以上接口与测试说明仅针对保留的 B01 演示。当前正式后端已实现数据库、认证、Gin、ready、分类/标签、音乐 API 和 OpenAPI；前端现已接入，详细状态以本文上方及项目状态文档为准。

旧开发下载包与缓存保留在 `../../.tools/`，不属于 Git 仓库；其中 zip 和 Go 1.26.7 便携工具链为历史遗留，当前 Go 1.27.1 不依赖它们。本次没有删除这些文件，也没有生成新的试验产物。

## 站内音乐播放

实现、实测结果、部署步骤及未完成验收见 [最新播放实施记录](../docs/music-public-playback-implementation-20261002.md)。默认 `MUSIC_PLAYBACK_ENABLED=false`。启用后同一进程增加回环媒体监听 8082；反向代理须优先将 `/api/v1/music/streams/` 路由至该监听。

导入本地 Bilibili 快照（先迁移，默认拒绝空列表和含歧义的非 P1 旧数据）：

```sh
go run ./cmd/manage migrate
go run ./cmd/manage import-bilibili ../frontend/src/data/bilibili-playlist.json
```

导入只保存元数据，availability 为 unknown；实际媒体校验成功后才标记 available。不修改当天已保存的推荐。

## 网易云音源与 FFmpeg

配置示例已加入 `MUSIC_FFMPEG=ffmpeg` 和 `MUSIC_FORCE_TRANSCODE=false`。启用播放时，yt-dlp 与 FFmpeg 必须可执行；FFmpeg 构建需包含 libmp3lame。正常 MP3 / 已确认 AAC-M4A 直接转发；FLAC、Ogg/Opus、WebM 音轨、WAV、ADTS AAC 等使用最多两个 FFmpeg Worker 输出 128 kbps MP3。设置 `MUSIC_FORCE_TRANSCODE=true` 可统一采用 MP3 兼容路径。

网易云 resolver 使用官方公开外链，仍受曲目权限和来源平台响应限制。同步 metadata 不代表音源可用，拒绝响应不会冒充空音频。实现、测试和环境限制见 [网易云与转码验收记录](../docs/music-netease-transcode-validation.md)。
