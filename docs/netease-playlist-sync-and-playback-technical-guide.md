# 网易云歌单同步与站内播放：技术设计与代码说明

> 后续操作统一使用 pnpm，见 [包管理约定](package-manager.md)。下文验收部分的 npm 命令保留为当时执行记录。

> 历史方案／设计资料，保留原文。当前实现统一见 [项目当前状态](project-status.md)（2026-10-02）；下文的技术选型、待实现事项和阶段状态只代表编写时点。

日期：2026-09-30。适用项目：本仓库 Go 后端与 Vue 前端。

本文说明现有实现、实际限制及下一阶段改造方案。标注“现有”的内容来自代码及隔离验收；标注“拟新增”“建议”“示意”的内容尚未实现，也不代表已确认网易云接口可用。本次仅编写文档，不修改业务代码或部署服务。

## 1. 目标与结论

目标是完整同步用户歌单的曲目元数据，并在官方音源可用时通过统一播放器播放；来源拒绝时显示明确原因和原站入口。

必须分别判断三个能力：

| 能力 | 判断对象 | 当前结论 |
| --- | --- | --- |
| 歌单同步 | 曲目 ID、标题、作者、顺序及数量 | 指定歌单尚未成功同步 |
| 官方歌单外链播放器 | 网易云提供的 iframe | 用户截图显示“由于版权保护，无法生成外链” |
| 单曲站内播放 | 每首歌的官方音源 | 一个公开样本通过真实链路；不代表目标歌单全部可播 |

FFmpeg 只转换已取得的音频，不能解决登录、版权或来源拒绝。配置了 embedUrl 不等于该外链可用；歌单 iframe 受限也不能直接推导每首歌均不可播放。

## 2. 问题证据与诊断边界

### 2.1 目标歌单

- ID：`595975585`，内部 ID：`netease:595975585`。
- [歌单页面](https://music.163.com/playlist?id=595975585)。用户截图标题为“林桃-喜欢的音乐”，显示 265 首，仅列出前 6 首，并提示下载客户端。
- [当前后端请求的详情接口](https://music.163.com/api/playlist/detail?id=595975585)。
- 同一服务器、相同请求方式下，目标歌单返回 HTTP 200、`{"msg":"","code":20001}`；公开热歌榜 `3778678` 返回 code=200 和 result。

因此，当前证据不支持“服务器整体被网易云封禁”。更可能是目标歌单的匿名访问条件或接口策略不同。由于本次 msg 为空，不能把 20001 精确解释成“歌单私密”“必须登录”或“版权拒绝”。

网页部分展示、外链版权提示、详情接口业务码是三个独立证据，不能混成一个根因。

### 2.2 已验证的播放能力

在服务器独立测试 schema 中，人工公开样本 `33894312` 完成：

- Go 会话 ready，媒体 HTTP 200 / audio/mpeg。
- 强制转码时实际观察到 Go 的 FFmpeg 子进程，stdin/stdout pipe，编码器 libmp3lame。
- 有界读取 65,536 字节媒体，内存解码退出码 0；一秒 PCM 176,400 字节且非全零。
- 浏览器进度推进，暂停保持 0:57，恢复至 1:02；切到无效样本显示失败提示，切回有效样本重新播放，停止后归零。

按钮验收使用键盘激活；鼠标自动化未稳定验证，物理扬声器出声也仍需人工听验。详细证据和残留清单见 [服务器验收记录](music-server-validation-20260930.md)。

## 3. 现有代码结构

以下路径均相对于本文所在的 docs 目录。

| 文件 | 职责 |
| --- | --- |
| [netease/client.go](../backend/internal/music/provider/netease/client.go) | 请求歌单详情；解析 result/playlist；校验完整性并映射 Track |
| [netease/client_test.go](../backend/internal/music/provider/netease/client_test.go) | 完整/不完整快照、字段和上游错误测试 |
| [netease/audio.go](../backend/internal/music/provider/netease/audio.go) | 官方单曲外链解析、重定向及音频格式检查 |
| [netease/audio_test.go](../backend/internal/music/provider/netease/audio_test.go) | 音源解析及拒绝路径测试 |
| [provider/provider.go](../backend/internal/music/provider/provider.go) | Ref、Track、Adapter、来源 URL 与字段校验 |
| [music/service.go](../backend/internal/music/service.go) | 异步同步、任务状态、事务快照和失败保留 |
| [music/playback_store.go](../backend/internal/music/playback_store.go) | 音乐库与播放检查结果的存储衔接 |
| [playback/service.go](../backend/internal/music/playback/service.go) | 播放会话、生命周期与资源限额 |
| [playback/stream.go](../backend/internal/music/playback/stream.go) | 安全媒体请求与流传输 |
| [playback/transcode.go](../backend/internal/music/playback/transcode.go) | FFmpeg pipe 转码 |
| [platform/music_playback_http.go](../backend/internal/platform/music_playback_http.go) | 播放 HTTP 接口 |
| [musicLibraryApi.ts](../frontend/src/musicLibraryApi.ts) | 获取歌单与完整分页曲目，映射前端数据 |
| [musicPlaybackApi.ts](../frontend/src/musicPlaybackApi.ts) | 创建、查询、停止会话及播放错误文案 |
| [useMusicPlayer.ts](../frontend/src/useMusicPlayer.ts) | Audio 实例、暂停恢复、队列、切歌与竞态隔离 |
| [MusicView.vue](../frontend/src/views/MusicView.vue) | 音乐页展示与交互 |

### 3.1 现有同步流程

```text
POST /api/v1/admin/music/playlists/:id/sync
  → 校验登录、Origin、CSRF 和任务容量
  → music.Service.StartSync，返回 202 + runId
  → netease.Client.FetchPlaylist
  → Parse + provider.Validate
  → ReplaceSnapshot 事务
  → 成功：ready；失败：记录错误并保留旧快照
```

管理员通过 `GET /api/v1/admin/music/sync-runs/:runId` 查询结果。公开端读取：

```text
GET /api/v1/music/playlists
GET /api/v1/music/playlists/:id/tracks?page=1&pageSize=50
```

当前客户端不带登录 Cookie，不使用环境代理，拒绝重定向；单次 HTTP 超时 15 秒，响应最多 2 MiB。同步 Service 的总上下文预算为 30 秒。

### 3.2 为什么显示 INVALID_PAYLOAD

现有 Parse 在 client.go 内，核心分支是：

```go
if p.Code == 401 || p.Code == 403 {
    return nil, provider.Failure("UPSTREAM_UNAUTHORIZED")
}
if p.Code == 429 {
    return nil, provider.Failure("UPSTREAM_RATE_LIMITED")
}
if p.Code != 200 {
    return nil, provider.InvalidPayload
}
```

20001 因而被统一归为 INVALID_PAYLOAD。这是项目错误分类过粗，不等于返回的 JSON 损坏。

现有完整性检查要求 `trackCount == len(tracks)`，并限制最多 2000 首。若上游只返回 6 首预览、总数却为 265，当前实现会拒绝；这个保护应保留，不能通过删掉检查让同步“变绿”。

## 4. 拟改造的元数据同步方案

### 4.1 先做只读可行性验证

在隔离环境核查可用歌单详情接口是否返回完整 trackIds。开源适配器文档描述了“tracks 不完整但 trackIds 完整，再按 ID 获取 song/detail”的路径；它不是网易云对本项目的稳定性承诺，也尚未对目标歌单验证成功。

验证至少记录：业务码、是否需要登录、trackCount、trackIds 数量、tracks 数量、分页字段与单曲详情可取性。保持请求次数有界，不批量轮询多个接口碰运气。

决策规则：

| 实际响应 | 处理 |
| --- | --- |
| code=200，完整 tracks | 沿用现有字段校验及快照流程 |
| code=200，完整 trackIds，部分 tracks | 补齐缺失详情，完整校验后再提交 |
| trackIds 也不完整或没有可靠总数 | 同步失败，保留旧快照 |
| code=20001 等明确非成功业务码 | 显示访问受限/来源拒绝，不以空列表覆盖 |
| JSON/字段结构异常 | INVALID_PAYLOAD |

当前公开接口若仍返回 20001，就没有 ID 清单可补齐。此时先报告阻塞；引入登录同步属于单独设计，不把用户 Cookie 加到浏览器或公共媒体服务中。也可支持用户提供的完整元数据文件人工导入，并标注为人工快照，不冒充在线同步成功。

### 4.2 Adapter 内分阶段处理

保留现有 `provider.Adapter` 接口，先在 netease 包内部扩展：

```go
// 设计示意：并非当前已存在的类型。
type PlaylistDetail struct {
    TrackCount int
    TrackIDs   []string
    Tracks     []provider.Track
}

// 分阶段职责：
// fetchDetail(ctx, ref)          → 获取详情和完整 ID 清单
// fetchSongDetails(ctx, ids)     → 通过已验证接口获取一批元数据
// assembleComplete(detail, ...) → 按歌单原始顺序组装并检查遗漏
// FetchPlaylist(ctx, ref)       → 全部成功才返回 []provider.Track
```

不在本文伪造已可用的新版 endpoint、签名算法或请求参数。具体协议应在真实接口验证后封装，且只访问固定受信域名。

补齐算法：

1. 先处理 HTTP 状态和业务码，再解析成功数据。
2. 校验总数、所有 ID 的格式、响应大小和最大曲目数。
3. 确认原始 ID 列表完整；检查后再按现有规则去重并保留首次出现的顺序。
4. 将已有完整 tracks 建立 ID 索引，仅请求缺失项。
5. 拟从每批 50 个 ID、并发 1 开始；这是项目建议上限，需按实测接口限制调整。
6. 每批验证返回 ID 属于请求集合；缺失、重复冲突、字段错误或非成功响应都不得静默跳过。
7. 按歌单 ID 顺序重建 Track 数组；所有条目通过 provider.Validate 后才交给 ReplaceSnapshot。
8. 任何一步失败，返回错误，不提交部分结果。

总同步超时必须覆盖所有批次，不能给每批独立的无限重试预算。当前 30 秒是否足够需在 265 首真实歌单上测量。可在结束前复查 ID 清单是否变化；发生变化则本次失败或有限重试，避免混合两个版本的歌单。

对于已下架而缺少详情的 ID，第一版保守判定不完整；若后续需要占位条目，应另外定义 schema、DTO 和 UI 语义，不能临时伪造标题与可用性。

### 4.3 错误分类建议

以下新增码需要后端、OpenAPI、前端和测试同步更新。

| 场景 | 建议项目错误码 | 面向用户文案 |
| --- | --- | --- |
| 20001，原因字段为空 | UPSTREAM_ACCESS_RESTRICTED（拟新增） | 网易云暂不允许读取此歌单，未完成同步。 |
| HTTP/业务码 401、403 | UPSTREAM_UNAUTHORIZED（已有） | 来源平台拒绝访问，请检查歌单访问条件。 |
| HTTP/业务码 429 | UPSTREAM_RATE_LIMITED（已有） | 网易云请求受限，请稍后再试。 |
| 未识别的非成功业务码 | UPSTREAM_REJECTED（拟新增） | 网易云未返回可用歌单。 |
| 总数、ID 或详情不完整 | INCOMPLETE_PLAYLIST（拟新增） | 只取得部分曲目，已保留原有歌单。 |
| 无效 JSON、字段类型不符 | INVALID_PAYLOAD（已有） | 网易云响应格式异常。 |

“已保留原有歌单”只在确实存在历史快照时使用；首次同步失败应显示“尚无完整同步记录”。

业务码映射示意：

```go
// 拟新增；调用者仍应先处理 HTTP 状态及 JSON 解码失败。
func classifyBusinessCode(code int) error {
    switch code {
    case 200:
        return nil
    case 401, 403:
        return provider.Failure("UPSTREAM_UNAUTHORIZED")
    case 429:
        return provider.Failure("UPSTREAM_RATE_LIMITED")
    case 20001:
        return provider.Failure("UPSTREAM_ACCESS_RESTRICTED")
    default:
        return provider.Failure("UPSTREAM_REJECTED")
    }
}
```

日志保留 requestId、sourceId、HTTP 状态、业务码、预期/实际数量；不记录 Cookie、凭据、完整上游响应或临时媒体 URL。不把上游任意 msg 原样显示给访客。

## 5. 数据库与 API 契约

现有核心表：music_sources、music_items、source_items、netease_sync_runs。播放相关迁移见 [000003_music_playback.up.sql](../backend/migrations/000003_music_playback.up.sql)。

完整 ID 补齐可以先在 Adapter 内实现，不必仅为分批请求增加表。继续遵循：

- 只有完整结果才能替换快照；所有关联更新处于一个事务。
- 失败更新同步状态和错误，不覆盖已有曲目关联。
- 同步只描述元数据，不把所有曲目标记 available。
- 已验证播放状态不被 metadata 的 unknown 覆盖。
- 不保存音频和有时效的 CDN 地址。

当前公开 Source DTO 不返回 last_error_code。若希望音乐页解释失败，建议新增安全的 `syncErrorCode` 可空字段，并贯通 Source 查询/DTO、OpenAPI、musicLibraryApi.ts 和 MusicView.vue。现有数据库已有 last_error_code，通常不需要为公开别名新增数据库列。

不要复用播放错误字段来表达歌单同步错误，也不要在未迁移枚举约束时直接写入新的 syncStatus。第一版可继续使用 pending/running/ready/failed，增加错误原因即可。

## 6. 现有播放链路及建议

```text
用户选歌
  → POST /api/v1/music/playback-sessions {trackId}
  → 从库内取曲目，解析官方单曲音源
  → GET /api/v1/music/playback-sessions/:id
  → ready 后浏览器加载服务端 streamUrl
  → Go 校验媒体地址、重定向及音频内容
  → 直接转发或 FFmpeg stdin → MP3 stdout
  → 浏览器 Audio 播放
```

现有媒体解析使用官方 `song/media/outer/url`，不带账号 Cookie；仅允许配置内的官方媒体域名，并对公网地址、重定向和实际连接做检查。FFmpeg 不直接访问远程 URL。

已有配置：

```dotenv
MUSIC_PLAYBACK_ENABLED=true
MUSIC_YTDLP=yt-dlp
MUSIC_FFMPEG=ffmpeg
MUSIC_FORCE_TRANSCODE=false
MUSIC_AUTO_SYNC=false
```

以上只是配置说明，程序不会自动加载 .env。服务器目前使用独立 FFmpeg 运行时及其动态库路径，默认 PATH 下未安装可直接调用的 ffmpeg，不能照抄配置就宣称可运行。

正常 MP3 可直接转发；强制转码主要用于兼容验证。当前转码输出为 MP3、128 kbps、44.1 kHz、双声道，转码并发上限 2。不要为了“提高成功率”移除 URL 安全检查、让 FFmpeg 自行联网或把失败 JSON 当音频输出。

建议逐首按用户播放请求检查可用性，不在每次同步后自动对 265 首全量请求音源。临时拒绝不等于永久失效，允许重试；原站链接保留。播放器错误与同步结果独立展示。

## 7. 前端调整建议

- 同步 running 显示同步中；failed 显示安全原因，并区分有无旧快照。
- 已同步的曲目继续允许搜索，不因同平台另一歌单失败而禁用整个网易云列表。
- 不把音乐库里的人工导入样本计入目标歌单同步成功。
- 官方 iframe 不可用时不渲染空白播放器；保留原站链接，不承诺一定唤起客户端。
- 曲目能否站内播放由实际音源检查决定；不因整个歌单外链被禁而一刀切禁用所有曲目。
- 明确显示暂停、缓冲、失败和重新播放；切歌时取消旧请求、停止旧会话，保留 useMusicPlayer 的 generation 隔离。

现有 musicLibraryApi.ts 已逐页拉取公开 API，并检查分页总数。这只能保证“读完后端已保存的数据”，不能证明后端快照等于网易云完整歌单。

## 8. 实施顺序与验收

1. 只读验证完整 ID 清单是否可获取。若失败，记录访问限制，不进入生产改造。
2. 拆分业务拒绝与格式错误，补充错误码及回归测试。
3. 只有上游可行后，实现详情分批补齐、顺序重建与完整性检查。
4. 更新公开同步状态 DTO、OpenAPI 和前端提示。
5. 独立测试数据库和回环服务复验；保留测试文件/schema，不自动删除。
6. 形成可审查结果后，再另行安排上线；本文不授权替换生产服务。

关键测试矩阵：

| 测试 | 预期 |
| --- | --- |
| code=20001 / 空 msg | 访问受限错误，不再归为 JSON 格式错误 |
| 265 个完整 ID、仅 6 个 tracks | 补齐成功才可提交 265 首；按原始顺序 |
| ID 缺失、批次超时、429、下架详情缺失 | 整次失败，旧快照保持不变 |
| 详情乱序、重复 ID、超大整数 ID | 明确去重规则、字符串保存、重建顺序，不发生精度损失 |
| 同步期间歌单变化 | 不提交混合版本快照 |
| 明确的完整空歌单 | 按业务规则处理；区别于权限拒绝和缺失字段 |
| 首次失败与已有快照失败 | 分别展示“无完整记录”和“保留旧列表” |
| 元数据成功但音源拒绝 | 曲目可展示，播放显示错误及原站入口 |
| 暂停/恢复、快速切歌、路由离开 | 状态正确，旧流取消，无迟到响应覆盖 |
| 真实转码及浏览器 | 有真实媒体和解码证据，人工听验、鼠标和移动端分别记录 |

常规代码验证命令（在对应目录执行）：

```sh
# backend
go test ./...
go vet ./...
go build ./...

# frontend
pnpm run test:music
pnpm run type-check
```

数据库集成需单独设置已核实的测试库连接；未配置导致 SKIP 不能算通过。不要把上述普通测试命令当作真实上游同步或浏览器播放证据。

## 9. 参考与交付范围

- [本次服务器端验收及产物](music-server-validation-20260930.md)
- [网易云音源与 FFmpeg 实现记录](music-netease-transcode-validation.md)
- [原始网易云后端设计](netease-music-backend.md)（包含历史阶段说明，应结合当前代码阅读）
- [项目 OpenAPI](../api/openapi.yaml)
- [NeteaseCloudMusicApiEnhanced 文档](https://neteasecloudmusicapienhanced.js.org/)：第三方实现自身的协议说明，作为候选方案参考，不代表网易云官方支持或目标歌单实测通过。

本次交付仅为本文。未修改源码、数据库、服务或凭据，未新增测试媒体，也未删除任何文件。
