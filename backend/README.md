# 最小 Go 后端

使用 Go 标准库 `net/http`，module 为 `blog-website/backend`，无第三方依赖，因此没有 `go.sum`。工具链固定为 Go 1.26.7，官方下载及校验信息见 [Go downloads](https://go.dev/dl/)。

当前数据是内存示例：两篇公开文章 `building`、`hello-go`，以及不可公开读取的草稿 `draft-note`。重启重新加载示例，无写入或持久化能力。

## 运行

安装 Go 1.26.7 并加入 PATH 后，在仓库根目录执行（PowerShell）：

```powershell
cd backend
go version
# 如有输出，先确认占用进程；服务不会自动结束其他进程。
Get-NetTCPConnection -LocalPort 8081 -State Listen -ErrorAction SilentlyContinue
go run ./cmd/api
```

默认监听 `127.0.0.1:8081`。可在启动前通过 `$env:HTTP_ADDR = '127.0.0.1:8082'` 修改地址；端口占用会报错并退出。Ctrl+C 触发关闭，最多等待 5 秒。访问日志使用 JSON，包含与响应 `X-Request-ID` 相同的 `requestId`。

本次开发使用的便携工具链位于仓库外的 `D:\blog-website\.tools\go1.26.7\go`，本机未配置 PATH 时可在当前 PowerShell 使用：

```powershell
$env:Path = 'D:\blog-website\.tools\go1.26.7\go\bin;' + $env:Path
$env:GOCACHE = 'D:\blog-website\.tools\go-cache'
$env:GOTOOLCHAIN = 'local'
```

这些设置只影响当前终端，不需要全局安装或修改系统环境变量。

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

本阶段未实现数据库、认证、Gin、ready、分类/标签独立接口、音乐 API、OpenAPI 或前端联调；Vue/Vite 保持现状，不引入 Nuxt。开始前端联调前按开发指南补充 OpenAPI。

开发下载包与缓存保留在 `D:\blog-website\.tools\`，不属于 Git 仓库；其中 zip 为安装残留，可在确认后清理，解压工具链用于运行，go-cache 用于后续测试加速。本次不删除任何文件。
