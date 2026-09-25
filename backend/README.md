# Go 后端（2026-09-25）

当前使用你已安装的 **Go 1.27.1**，保留原 `go.mod` 版本修改。正式服务为 Gin + GORM + PostgreSQL，数据库结构使用嵌入的版本化 SQL 和 golang-migrate 显式创建。无自动迁移、无默认管理密码、无静默内存回退。

已实现：公开文章/分类/标签、ready、草稿创建/修改/发布/归档、乐观版本控制、标签事务、站长会话、Origin/CSRF、登录限流、公开歌单/曲目、网易云异步同步与状态查询、人工音乐元数据导入、上海日期的每日推荐与历史。完整字段及权限见 [OpenAPI](../api/openapi.yaml)（JSON 格式也是合法 YAML 1.2）。

当前机器没有 PostgreSQL，数据库集成测试需要安装后单独运行；真实网易云歌单同步、前端接入、纯音频、上线和备份恢复尚未验收。测试 fixture 不是在线同步成功的证明。

## 启动正式后端

先安装 PostgreSQL，在本地建立专用 `blog_user` 角色及其拥有的 `blog` 数据库。不要复用生产库进行测试。`.env.example` 仅说明变量，程序不会自动加载 `.env`。

```powershell
cd D:\blog-website\blog-website\backend
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

迁移命令仅支持向前 `up`，不提供删除表、回滚或重置命令。迁移失败后先检查 `schema_migrations` 中的 version/dirty 和数据库错误，不自动 force。分类/标签目前通过本地参数化 SQL 或管理工具维护（id、slug、name），尚无对应写 API。

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

## 音乐与任务

迁移登记 `netease:595975585`，初始 pending，无伪造曲目。登录后以相同 Cookie/Origin/CSRF 调用 `POST /admin/music/playlists/netease:595975585/sync`，返回 202 和真实 runId，再查询 `GET /admin/music/sync-runs/{runId}`。同来源运行中返回 409。全局最多 2 个同步任务，手动触发最多 4 次/分钟；登录最多 10 次/分钟。

适配器只向固定 `https://music.163.com/api/playlist/detail` 请求公开 JSON，禁用跳转和环境代理，不带 Cookie、签名或媒体下载。这个上游并非本项目可保证稳定的开放 API：拒绝访问、超时、HTML、超过 2 MiB、trackCount 与 tracks 数量不一致均失败，不替换快照。单次最多 2000 首；重复 ID 保留首条；无可靠时长/作者/iframe 返回 null；同步曲目默认 availability=unknown，不冒充可播。

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

当前 Go 安装目录为 `D:\Golang`。检查版本与实际命中的可执行文件：

```powershell
go version
(Get-Command go).Source
go env GOROOT
```

历史便携工具链 `D:\blog-website\.tools\go1.26.7\go` 仅作为旧文件保留，不应再加入当前 PATH。恢复正式后端前设置 `$env:DEMO_MODE = 'false'`，并按本文上方配置数据库。

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

以上接口与测试说明仅针对保留的 B01 演示。当前正式后端已实现数据库、认证、Gin、ready、分类/标签、音乐 API 和 OpenAPI；前端联调仍待完成，详细状态以本文上方为准。

旧开发下载包与缓存保留在 `D:\blog-website\.tools\`，不属于 Git 仓库；其中 zip 和 Go 1.26.7 便携工具链为历史遗留，当前 Go 1.27.1 不依赖它们。本次没有删除这些文件，也没有生成新的试验产物。
