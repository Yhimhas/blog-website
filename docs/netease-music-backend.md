# 网易云音乐专用后端技术文档

2026-09-25 实施补充：适配器、完整性校验、快照事务、异步任务及公开/管理接口已加入 backend；首次无快照固定返回 200 + 空曲目数组，配合歌单 syncStatus 展示。每日自动同步默认关闭，可设置 MUSIC_AUTO_SYNC=true 启用。本机尚无 PostgreSQL，真实歌单同步与数据库集成验收尚未执行，不能将 fixture 测试视为同步成功。运行方法见 [backend/README.md](../backend/README.md)。

版本：v1.0  
日期：2026-09-21  
状态：实现契约，尚未创建 backend 工程  
适用项目：`D:/blog-website/blog-website`

本文把现有 Go 后端契约中的 `provider = netease` 落实为一个可实施的来源适配器。目标是同步网易云歌单的**曲目元数据**，向前端提供稳定的只读 API；不把网易云网页当作浏览器可直接调用的开放 API，也不把官方 iframe 当作后端同步结果。

## 1. 当前问题与边界

当前前端已登记歌单：

```text
playlistId: netease:595975585
title: 我喜欢的音乐
sourceUrl: https://music.163.com/m/playlist?id=595975585&creatorId=417762656
embedUrl: https://music.163.com/outchain/player?type=0&id=595975585&auto=0&height=430
syncStatus: pending
tracks: []
```

公开网页或未登录接口可能返回未授权、风控、空数据或结构变化。后端因此必须满足：

- 网易云请求只在服务端执行，浏览器不接触 Cookie、请求签名或上游地址。
- 只有歌单 ID 和曲目 ID 等元数据进入数据库，不保存 Cookie、完整音频、媒体缓存或破解参数。
- 同步失败保留最近一次完整快照；不能把失败响应当成空歌单覆盖旧数据。
- 返回 `availability = unknown/unavailable` 时，前端可以展示曲目，但不能宣称已实现纯音频播放。
- 官方播放器入口与元数据同步独立；即使同步成功，也继续保留 `embedUrl` 和原始歌单链接。

本文件不实现音频解析、下载、转码、绕过登录或 DRM。纯音频属于 [music-audio-streaming-design.md](music-audio-streaming-design.md) 的后续里程碑。

## 2. 在现有后端中的位置

沿用 [backend-development-guide.md](backend-development-guide.md) 的单体 Go + Gin + PostgreSQL + GORM 架构：

```text
backend/
  internal/music/
    provider/provider.go             # 通用来源适配器接口
    provider/netease/client.go       # 网易云 HTTP 客户端
    provider/netease/parser.go       # 响应解析与字段校验
    provider/netease/types.go        # 上游最小 DTO，不向外暴露
    repository.go                    # music_sources/items/source_items
    service.go                       # 同步事务、状态和快照规则
    handler.go                       # 公开查询和管理同步接口
  migrations/
    000x_music_sources.sql
    000y_netease_sync_runs.sql
```

请求链路：

```text
管理员触发同步或定时任务
  -> music service 加载 netease source
  -> netease client 请求上游（超时、取消、限流）
  -> parser 校验完整性和字段上限
  -> repository 在一个事务中写入新快照
  -> 公开 API 返回最近一次成功快照
  -> Vue 前端将 author/sourceUrl/embedUrl 映射为当前字段
```

只为网易云替换来源逻辑；`music service`、推荐和公开 DTO 不复制一套网易云专用业务流程。

## 3. 来源配置

### 3.1 `music_sources` 记录

歌单是数据库中的来源配置，而不是硬编码在 handler 中：

```sql
CREATE TABLE music_sources (
  id              text PRIMARY KEY,
  provider        text NOT NULL CHECK (provider IN ('bilibili', 'netease')),
  external_id     text NOT NULL,
  title           varchar(160) NOT NULL,
  source_url      text NOT NULL,
  embed_url       text,
  enabled         boolean NOT NULL DEFAULT true,
  sync_status     text NOT NULL DEFAULT 'pending'
                  CHECK (sync_status IN ('pending', 'running', 'ready', 'failed')),
  synced_at       timestamptz,
  last_error_code text,
  last_error_at   timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, external_id)
);
```

初始化网易云来源：

```sql
INSERT INTO music_sources
  (id, provider, external_id, title, source_url, embed_url)
VALUES
  ('netease:595975585', 'netease', '595975585', '我喜欢的音乐',
   'https://music.163.com/m/playlist?id=595975585&creatorId=417762656',
   'https://music.163.com/outchain/player?type=0&id=595975585&auto=0&height=430')
ON CONFLICT (provider, external_id) DO UPDATE
SET title = EXCLUDED.title,
    source_url = EXCLUDED.source_url,
    embed_url = EXCLUDED.embed_url,
    updated_at = now();
```

服务端再次校验 URL：只允许 `https`、固定 host `music.163.com`，歌单 URL 的 path 为 `/playlist` 或 `/m/playlist`；embed URL 只能是 `/outchain/player`。数据库中的 URL 也不能因此跳过校验。

### 3.2 曲目表

沿用现有设计的 `music_items` 与 `source_items`，并用 `part_key = ''` 表示无分 P：

```sql
CREATE TABLE music_items (
  id                text PRIMARY KEY,
  provider          text NOT NULL CHECK (provider IN ('bilibili', 'netease')),
  external_id       text NOT NULL,
  part_key          text NOT NULL DEFAULT '',
  title             varchar(300) NOT NULL,
  author            varchar(160),
  source_url        text,
  duration_seconds  integer CHECK (duration_seconds IS NULL OR duration_seconds >= 0),
  availability      text NOT NULL DEFAULT 'unknown'
                    CHECK (availability IN ('available', 'unavailable', 'unknown')),
  embed_url         text,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, external_id, part_key)
);

CREATE TABLE source_items (
  source_id   text NOT NULL REFERENCES music_sources(id),
  item_id     text NOT NULL REFERENCES music_items(id),
  position    integer NOT NULL CHECK (position >= 0),
  active      boolean NOT NULL DEFAULT true,
  first_seen  timestamptz NOT NULL DEFAULT now(),
  last_seen   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (source_id, item_id)
);

CREATE INDEX source_items_order_idx
  ON source_items (source_id, active, position, item_id);
```

网易云曲目 ID 必须保存为字符串，避免 JSON 数字精度和未来 ID 格式变化。建议 `music_items.id` 使用稳定的 `netease:<songId>`，例如 `netease:123456`。

## 4. 上游适配器契约

通用接口只返回已经校验的内部候选，不把网易云响应结构泄漏给 service：

```go
package provider

import "context"

type PlaylistRef struct {
    Provider   string
    ExternalID string
    Title      string
    SourceURL  string
    EmbedURL   *string
}

type TrackCandidate struct {
    ExternalID      string
    PartKey         string
    Title           string
    Author          *string
    SourceURL       *string
    DurationSeconds *int
    Availability    string // available, unavailable, unknown
    EmbedURL        *string
}

type Adapter interface {
    Provider() string
    FetchPlaylist(ctx context.Context, ref PlaylistRef) ([]TrackCandidate, error)
}
```

网易云实现只负责：

1. 校验 `externalId` 为十进制歌单 ID。
2. 使用 `http.Client` 请求允许的上游地址。
3. 检查 HTTP 状态、Content-Type、响应大小和 JSON 结构。
4. 将歌曲 ID、名称、歌手、时长和官方歌曲链接转换成 `TrackCandidate`。
5. 发现登录、风控、字段缺失或结构不兼容时返回可分类错误。

### 4.1 Go HTTP 客户端骨架

```go
package netease

type Client struct {
    httpClient *http.Client
    baseURL    *url.URL // 由配置注入，生产默认指向受控上游地址
    userAgent  string
    maxBytes   int64
}

func (c *Client) FetchPlaylist(ctx context.Context, ref provider.PlaylistRef) ([]provider.TrackCandidate, error) {
    if ref.Provider != "netease" || !isDecimalID(ref.ExternalID) {
        return nil, ErrInvalidSource
    }

    endpoint := c.playlistEndpoint(ref.ExternalID)
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
    if err != nil { return nil, fmt.Errorf("build request: %w", err) }
    req.Header.Set("Accept", "application/json")
    req.Header.Set("User-Agent", c.userAgent)

    resp, err := c.httpClient.Do(req)
    if err != nil { return nil, classifyTransportError(err) }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
        return nil, ErrUpstreamUnauthorized
    }
    if resp.StatusCode == http.StatusTooManyRequests {
        return nil, ErrUpstreamRateLimited
    }
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("%w: status=%d", ErrUpstreamHTTP, resp.StatusCode)
    }

    limited := io.LimitReader(resp.Body, c.maxBytes+1)
    body, err := io.ReadAll(limited)
    if err != nil { return nil, fmt.Errorf("read response: %w", err) }
    if int64(len(body)) > c.maxBytes { return nil, ErrResponseTooLarge }

    payload, err := parsePlaylist(body)
    if err != nil { return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err) }
    return mapTracks(ref, payload)
}
```

生产实现不得把任意用户提供的 URL 直接交给 HTTP client，避免 SSRF。上游 host、超时、最大响应体、并发数和重试次数都来自服务端配置；重试仅用于连接失败和 502/503，不能重试 401/403/429。

### 4.2 解析规则

- `trackId` 缺失或不是十进制 ID：整次同步失败。
- 标题为空：整次同步失败，防止生成不可用快照。
- 歌手数组为空：保留作者为 `null`，不拼接客户端猜测值。
- 时长从毫秒转换为秒并向下取整；异常值设为 `null`。
- 歌曲官方 URL 使用 `https://music.163.com/#/song?id=<id>`，生成后仍经过 allowlist 校验。
- `embedUrl` 使用整张歌单的官方播放器 URL；单曲没有可靠 iframe 时返回 `null`。
- 同一曲目重复出现时保留第一次位置，后续重复项记日志并跳过。
- 单次最多接受 2,000 首曲目、标题 300 字符、作者 160 字符；超限属于 `invalid_payload`。

不要从 HTML 正则提取曲目，也不要依赖前端传入的 `creatorId` 作为权限证明。`creatorId` 仅用于保留原始来源链接。

## 5. 同步服务与快照事务

### 5.1 同步状态机

```text
pending -> running -> ready
                   \-> failed
ready   -> running -> ready
                   \-> failed（继续提供旧快照）
```

`failed` 不等于空歌单。公开接口仍返回最近一次 `ready` 的曲目，并在歌单 DTO 中提供 `syncStatus: failed` 和 `syncedAt`。首次同步失败且没有旧快照时，曲目列表为空，前端显示“待同步/暂不可用”。

### 5.2 并发与幂等

- 使用 PostgreSQL 行锁 `SELECT ... FOR UPDATE` 锁住 `music_sources`，同一来源只允许一个同步任务。
- 管理接口重复触发时返回 `409 SYNC_IN_PROGRESS`，不要启动第二个上游请求。
- 同步运行记录写入 `netease_sync_runs`，保存开始/结束时间、候选数、错误 code 和 requestId，不保存上游响应原文。
- 通过候选列表 hash 判断是否变化；未变化也更新成功时间，但不重复写全部曲目。

### 5.3 事务伪代码

```go
func (s *Service) Sync(ctx context.Context, sourceID string) error {
    source, err := s.repo.LockSourceForSync(ctx, sourceID)
    if err != nil { return err }

    candidates, err := s.adapter.FetchPlaylist(ctx, toRef(source))
    if err != nil {
        _ = s.repo.MarkSyncFailed(ctx, source.ID, classifySyncError(err))
        return err // 旧 source_items 不动
    }
    if err := validateSnapshot(candidates); err != nil {
        _ = s.repo.MarkSyncFailed(ctx, source.ID, "invalid_snapshot")
        return err
    }

    return s.repo.ReplaceSnapshot(ctx, source.ID, candidates)
}
```

`ReplaceSnapshot` 必须在单一数据库事务中完成：upsert `music_items`、写入新的 `source_items.position`、将不在新集合中的旧关联设为 `active = false`、更新 `sync_status = ready` 和 `synced_at`。任何一步失败都回滚，旧快照继续可读。

## 6. API 契约

### 6.1 公开查询

沿用现有前缀和分页约定：

```http
GET /api/v1/music/playlists
GET /api/v1/music/playlists/netease:595975585/tracks?page=1&pageSize=20
GET /api/v1/music/recommendations/today
```

歌单响应：

```json
{
  "data": [{
    "id": "netease:595975585",
    "provider": "netease",
    "title": "我喜欢的音乐",
    "sourceUrl": "https://music.163.com/m/playlist?id=595975585&creatorId=417762656",
    "embedUrl": "https://music.163.com/outchain/player?type=0&id=595975585&auto=0&height=430",
    "syncStatus": "ready",
    "syncedAt": "2026-09-21T12:00:00Z",
    "trackCount": 42
  }]
}
```

曲目响应：

```json
{
  "data": [{
    "id": "netease:123456",
    "provider": "netease",
    "externalId": "123456",
    "partId": null,
    "title": "示例歌曲",
    "author": "示例歌手",
    "sourceUrl": "https://music.163.com/#/song?id=123456",
    "durationSeconds": 238,
    "availability": "unknown",
    "embedUrl": null
  }],
  "pagination": { "page": 1, "pageSize": 20, "total": 42 }
}
```

公开 DTO 不返回上游原始 JSON、请求头、Cookie、同步错误详情或内部堆栈。`sourceUrl` 是用户可访问的原平台页面，`embedUrl` 只能是服务端 allowlist 通过的官方播放器地址。

### 6.2 管理同步接口

```http
POST /api/v1/admin/music/playlists/netease:595975585/sync
```

要求现有站长会话、Origin 校验和 CSRF token。响应：

- `202`：同步任务已接受，返回 `{ "data": { "runId": "...", "status": "running" } }`。
- `409`：已有任务运行，返回 `SYNC_IN_PROGRESS`。
- `404`：来源不存在或已禁用。
- `429`：管理员同步限流。

可选状态查询：

```http
GET /api/v1/admin/music/sync-runs/{runId}
```

仅返回状态、计数、时间和分类错误码，不返回上游内容。定时同步也调用同一个 service，不复制逻辑；建议每日最多一次，失败采用指数退避并保留旧快照。

## 7. 错误码和前端行为

| 内部错误码 | HTTP | 含义 | 前端行为 |
| --- | ---: | --- | --- |
| `INVALID_SOURCE` | 400 | ID 或来源配置非法 | 管理日志修正配置 |
| `UPSTREAM_UNAUTHORIZED` | 503 | 上游需要登录或拒绝访问 | 保留旧曲目，显示待同步 |
| `UPSTREAM_RATE_LIMITED` | 503 | 上游限流 | 延后重试，不立即重复请求 |
| `UPSTREAM_TIMEOUT` | 503 | 上游超时 | 保留旧曲目，显示重试 |
| `INVALID_PAYLOAD` | 502 | 响应结构不兼容 | 告警并等待适配器更新 |
| `SYNC_IN_PROGRESS` | 409 | 同来源已有同步 | 展示同步中 |
| `NO_SNAPSHOT` | 503 | 从未成功同步 | 显示空态和官方播放器入口 |

公开列表成功但最近同步失败时仍返回 200，因为旧快照可用；歌单 DTO 的 `syncStatus` 告知前端数据状态。只有没有任何快照时才返回 503 或 200 空列表，具体选择须在 OpenAPI 固定，不能按 handler 临时决定。

## 8. 安全、合规和运行约束

- 不从客户端接收任意网易云 URL 并代理；客户端只能传数据库中的 source ID。
- 上游请求固定 DNS/host allowlist，禁止跟随到内网地址；限制重定向次数并再次校验目标 host。
- 请求超时建议连接 5 秒、整体 15 秒；响应体上限 2 MiB；单来源并发 1，全局并发按配置限制。
- 日志只记录 provider、sourceId、状态码、耗时、候选数、错误码和 requestId；对 URL 查询参数做脱敏。
- 不提交 Cookie、账号密码、签名密钥或上游原始响应到 Git、数据库和日志。
- 仅同步公开元数据，并在页面保留网易云来源链接和作者信息；实际曲目使用条件仍需人工确认。
- 不实现绕过登录、验证码、风控或 DRM 的逻辑。上游拒绝时报告不可用状态。
- 数据库备份包含元数据和同步记录，不包含媒体文件。若未来启用缓存，必须单独设计容量、过期和清理规则，不能自动删除用户文件。

## 9. 测试与验收

### 单元测试

- 正常 payload：多歌手、无歌手、无时长、重复曲目。
- 非法 payload：缺 `trackId`、空标题、超长字段、超过 2,000 首。
- 状态分类：401/403、429、502、超时、响应体超限。
- URL 校验：合法官方 URL、非 HTTPS、错误 host、开放重定向。
- `partKey = ''` 的唯一性和重复同步幂等性。

### 集成测试

- 上游失败后 `source_items` 和 `synced_at` 保持旧值。
- 同步事务中途失败时不产生半套曲目。
- 两个并发同步只有一个获得锁，另一个收到 `SYNC_IN_PROGRESS`。
- 新快照移除的曲目变为 `active = false`，历史 `music_items` 不被物理删除。
- 公开接口只返回 active 关联、稳定 position 和统一 DTO。
- `recommendations/today` 使用 `Asia/Shanghai` 日期，并只从 ready 曲库抽取。

### 手工验收命令

```powershell
cd D:\blog-website\blog-website\backend
go test ./...
curl.exe http://127.0.0.1:8081/api/v1/health
curl.exe "http://127.0.0.1:8081/api/v1/music/playlists/netease:595975585/tracks?page=1&pageSize=20"
```

验收必须记录：同步前后的 `trackCount`、`syncStatus`、`syncedAt`、失败时旧数据是否仍可查询、日志是否包含敏感信息。不能只验证 HTTP 200；还要检查 JSON 字段、分页、顺序和错误码。

## 10. 实施顺序

1. 建立迁移和 `music_sources/music_items/source_items` repository。
2. 先写 parser fixture 和 URL/字段校验测试，不连接真实网易云。
3. 完成 `netease.Adapter`，使用可替换的 `httptest.Server` 验证状态分类。
4. 实现同步事务和 `sync_runs`，验证失败保留快照。
5. 暴露公开音乐 API，再由前端把本地 `netease-playlist.json` 的 pending 状态替换成 API 状态。
6. 加入需要会话的手动同步接口和每日任务。
7. 用真实公开歌单进行一次人工同步；若上游返回未授权，保留 pending/failed 状态，不伪造歌曲。
8. 只有元数据链路稳定后，另开任务评估纯音频播放，不把本文件的完成当作音频播放完成。

## 11. 完成标准

- `netease:595975585` 能在数据库中表示，URL 和 provider 校验通过。
- 成功同步后公开接口返回曲目、顺序、作者、时长和官方来源链接。
- 上游拒绝、超时、限流、结构变化均有稳定错误码，旧快照不丢失。
- 同步任务可重入、可取消、有限流，日志可通过 requestId 定位。
- 前端可以区分 `pending`、`ready`、`failed` 和无快照，不把空列表误报为“网易云歌单为空”。
- `go test ./...`、数据库集成测试和手工 API 验收通过。
- 没有下载音频、保存 Cookie、生成媒体缓存或修改现有播放器行为。

本次只新增技术文档，未创建 backend 代码、迁移、网络同步任务或测试残留，也没有删除任何文件。
