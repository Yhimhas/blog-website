# 音乐站内播放实施计划

日期：2026-09-28。状态：基于当前代码核查的实施计划，尚未实现或验证播放。

本文承接 [纯音频原方案](music-audio-streaming-design.md)、[网易云元数据方案](netease-music-backend.md) 和 [后端协作手册](backend-development-guide.md)。目录均以仓库根目录为基准。旧文档保留；当前状态以代码及本文核查为准。

## 1. 实施结论与首版范围

继续使用 Vue 3 + Go/Gin + PostgreSQL，在现有后端中增加播放模块。先完成“数据库中一首 Bilibili 曲目 → 音轨解析 → 同域音频流 → 页面真实播放”，再扩展曲库和网易云。无需先搭 Redis、消息队列、独立微服务或媒体文件库。

首版交付：公开访客点击播放、暂停/恢复、停止、音量、手动上下首、真实进度、结束后顺序播放，以及明确的失败提示。保持当前人物背景、分层切换、标题及作者出处布局。首版仅 Bilibili P1；不承诺任意拖动、不播放视频画面、不提供本地上传、不持久化媒体。

网易云歌单已登记为 `netease:595975585`，不是等待用户提供链接。其元数据同步与音源获取分开验收，不能因歌曲出现在列表就承诺站内可播。

## 2. 代码核查结果

| 部分 | 当前证据 | 实施含义 |
| --- | --- | --- |
| 音乐后端基础 | `backend/internal/music/service.go`、`import.go`、`provider/netease/client.go` 已存在 | 复用查询、导入、快照事务和网易云同步，不重写整套后端 |
| HTTP 接口 | `backend/internal/platform/database_http.go` 已注册歌单、曲目、推荐和管理同步 | 新增播放接口；已有接口继续使用 |
| 播放接口 | 当前没有 playback-sessions / streams 路由 | 音源解析和媒体传输属于新增工作 |
| 当前页面 | `frontend/src/views/MusicView.vue` 没有 audio 或 iframe；底部仅原站播放入口 | 上下首只改变选中曲目，不存在真实播放状态 |
| 音乐库 | `frontend/src/musicSources.ts` 静态导入两个 JSON | 正式音乐库改为读取后端分页 API |
| 每日推荐 | 页面实际调用 `frontend/src/musicApi.ts`，经 `useMusicRecommendations.ts` 管理状态 | 不应只修改另一份 `musicRecommendations.ts` 就认为接入完成 |
| 开发代理 | `frontend/vite.config.ts` 已把 `/api` 转发到 8081 | 不重复添加普通代理；单独补媒体路由 |
| Bilibili 快照 | 本地 2026-09-20 快照记录 53 条；ID 为 `bilibili:<BVID>` | 不是当前线上数量，也不代表 53 条都可播 |
| 后端 Bilibili ID | `provider.Validate` 要求 `bilibili:<BVID>:<partId>` | 导入、前端选择、收藏 ID 必须统一 |
| 收藏夹 URL | 本地使用 `space.bilibili.com/.../favlist`；`provider.OfficialURL` 只接受 Bilibili 视频页 | 直接把现有快照转换后交给 Import 仍会失败，须先拆分 URL 校验 |
| HTTP 超时 | `backend/cmd/api/main.go` 的 WriteTimeout 为 10 秒 | 现有监听不能直接照搬作长音频传输 |
| 推荐候选 | `backend/internal/recommendation/recommendation.go` 只选 availability=available | 网易云同步生成 unknown，不会自动进入推荐；不能简单将所有曲目标成 available |

本次没有执行数据库集成或真实音源测试。尝试通过 SSH 对 `100.96.172.0` 做只读检查，连接返回 Permission denied；未取得服务器系统、代理配置、数据库及工具安装情况。该结果不能用于判断密钥失效或服务未部署。

## 3. 目标架构

```text
MusicView + useMusicPlayer
  ├─ 歌单/曲目/推荐 API → 现有 Gin → PostgreSQL 元数据
  ├─ 创建/查询/停止会话 → 新 playback.Service（进程内，有界存储）
  └─ <audio src="/api/v1/music/streams/...">
       → 同域代理 → 同一 Go 进程的媒体监听
       → 校验后的上游音轨 → 原样转发 / 必要时 FFmpeg → 浏览器
```

解析器和媒体会话在同一后端进程中运行。yt-dlp 作为受控子进程解析公开 Bilibili 内容，显式选独立音轨；FFmpeg 仅在格式需要转换时参与。工具版本在 P0 实测后固定，不预先声称某个版本必然兼容当前平台。

默认保留匿名访问音乐页；播放写请求校验同源 Origin、限制 body 和频率，采用短期匿名播放 Cookie 绑定会话。管理同步仍使用现有站长权限和 CSRF。会话随机 ID 与 Cookie 共同校验，不能仅凭曲目 ID 无限启动 Worker。

## 4. 先解决数据和契约问题

### 4.1 Bilibili 曲目与来源

新增独立的 SourceURL 与 TrackURL 校验：前者支持固定 host、收藏夹路径及合法 owner/fid；后者仍只接受官方视频页。不要全局放宽 OfficialURL，导致歌曲地址或 iframe 校验也接受收藏夹 URL。Import 和 Playlists 的来源校验都要改。

首版固定 `partId="1"` 表示 P1，ID 为 `bilibili:<BVID>:1`。解析所得 CID 是独立内部字段，不能与分 P 序号混用。实现前先核查数据库是否已有其他含义的 partId；有歧义的已有数据需逐项映射，不盲目重写。

新增可重复运行的本地快照导入命令/子命令，读取 `frontend/src/data/bilibili-playlist.json`，映射为现有 Source/Track 后走 Import 事务：

- platform → provider，artist → author，url → sourceUrl，externalId 从已验证 BVID 提取。
- 补齐 P1 和 canonical ID；未知时长返回 null，未验证播放的 availability 为 unknown。
- 先完成全量校验再导入；默认拒绝意外空列表，输出导入统计；失败保留原快照。
- 沿用事务软停用旧关联，不删除原 JSON、数据库曲目或任何文件。

收藏兼容：读取旧 `lin-music-favorites` 和 `lin-music-favorite-snapshots` 时，将已确认的旧 Bilibili ID 映射为 P1 新 ID；保存新版本数据，保留旧键供回退。没有映射的收藏保留并显示暂不可用，不能静默丢弃。

### 4.2 音源状态与推荐

复用三态 availability，但补充含义和更新规则：unknown 允许用户发起一次受限解析；available 代表最近一次验证成功，不保证永久可播；unavailable 表示已确认的内容失效或当前不支持。

新增向前迁移保存播放检查时间和分类结果（例如 playback_checked_at、playback_error_code）；不保存临时音源 URL。暂时超时、429、服务忙不能把歌曲永久标成 unavailable。

元数据同步的 unknown 不能无条件覆盖已验证的播放结果；修改 ReplaceSnapshot 的更新字段策略并测试。推荐仍沿用 available 筛选，不把 unknown 批量改成 available。首次只对小批样本做有界播放验证，再让后续每日推荐使用它们。

当天推荐已保存为空时，导入/验证成功不会自动改写它，这是现有快照设计；音乐库可以立即播放，推荐按下一次生成生效。此次不偷偷重抽当天历史记录。

## 5. 播放 API 和会话生命周期

以下均为拟新增接口，写入 `api/openapi.yaml` 后再实现；JSON 沿用项目 data/error/requestId 包装。

| 方法与路径（前缀 `/api/v1/music`） | 约定 |
| --- | --- |
| `POST /playback-sessions` | body 仅 `{ "trackId": "bilibili:BV1a4MS67Eey:1" }`；查启用来源和 active 关联；202 返回 preparing 会话 |
| `GET /playback-sessions/{id}` | 返回 preparing/ready/streaming/ended/failed/stopped/expired 和安全错误码 |
| `GET /streams/{id}` | 同源 Cookie 校验后输出音频；一个会话同一时刻最多一个媒体连接 |
| `POST /playback-sessions/{id}/stop` | 幂等释放解析、上游及 Worker；返回 204 |

就绪响应字段：sessionId、status、streamUrl、mimeType、durationSeconds、seekMode、expiresAt、attribution。第一版 `seekMode="none"`，未知时长为 null。ready 仅表示解析完毕，真正播放以 audio 的 playing 为准。

创建接口先校验并预留配额，快速返回 202，不在当前 10 秒 HTTP 写窗口内等待完整解析。异步任务使用“应用生命周期 + 会话取消 + 解析超时”的 context，不能直接使用已返回的创建请求 context。

建议初始配置（待服务器实测调整）：解析并发 2、活跃媒体流 4、FFmpeg 并发 2、单访客活跃流 1、创建请求每访客/IP 每分钟 10 次。会话总条目上限 128，未连接 ready 会话 60 秒过期，终态保留 5 分钟；到期移除内存条目，避免请求只建不播耗尽资源。限流 IP 仅采信已配置的反向代理，不能信任任意 X-Forwarded-For。

解析超时 20 秒、首字节 15 秒、上游读空闲及下游写空闲各 30 秒、最长流 6 小时。首批有效字节前校验上游状态和媒体类型；媒体响应开始后出现异常只中止流并更新会话，不能向音频追加 JSON。现有 Gin panic 中间件也需避免对已写响应再次发送 JSON。

前端以约 500–1000ms 轮询 preparing 状态，终态停止轮询。切歌先取消旧请求、释放旧 audio source，再停止旧会话；使用请求序号忽略迟到结果。如果旧 POST 已成功创建但响应未到，后端 TTL 负责兜底回收。后端重启导致会话丢失时提示重新播放。

暂停允许浏览器继续有限缓冲；如果连接已超时或会话已过期，恢复时明确提示重新开始，不能伪装成无缝续播。浏览器的重复探测、Range 请求、断连后重连作为实际联调项目：首版不支持 Range seek，不能伪造 206；并发重复媒体连接返回明确冲突。

错误码至少包括：TRACK_NOT_FOUND、PLAYBACK_UNSUPPORTED、UPSTREAM_UNAVAILABLE、UPSTREAM_RATE_LIMITED、PLAYBACK_TIMEOUT、PLAYBACK_BUSY、SESSION_EXPIRED。参数错误 400，不存在 404，过期 410，频率/容量限制 429，来源暂不可用 503；不支持的来源/格式 422。流已开始后的错误由状态 API 提供。

## 6. 音轨解析、传输和部署

### 6.1 解析策略

新增 AudioResolver 接口，与现有 FetchPlaylist 适配器分开。输入只接受库内曲目；输出仅供后端使用的 URL、必要 headers、codec/container、时长和过期信息。

Bilibili PoC 显式选择 bestaudio，不使用会悄悄退回视频的默认格式；yt-dlp 的 stdout 默认格式可能包含视频，必须在测试中核实返回的是独立音轨。[yt-dlp 官方说明](https://github.com/yt-dlp/yt-dlp#format-selection)

优先转发实测兼容的独立音轨；不兼容时使用 FFmpeg 输出 128 kbps MP3 作为兼容路径。以参数数组启动、持续消费有界 stderr、限制解析 JSON 大小。停止和断连终止整棵子进程树；分别实现/测试 Windows 开发与 Linux 部署的取消方式。

应用不能只校验用户输入：解析得到的媒体 URL、DNS 实际连接地址和每次重定向都校验，拒绝私网/本机地址。若 FFmpeg 自行读取远程 URL，会绕开 Go HTTP client 的检查；优先从受控 Go 请求管道输入。遇到输入必须 seek 的容器，则在 PoC 中验证受限出站代理方案；在该路径可控之前返回不支持，不启用无约束远程读取。

不写完整媒体、不写 HLS 分片；按需读取并施加背压，应用缓冲最多 1 MiB/流，不使用 ReadAll 读取媒体。日志隐藏签名 URL、请求头和会话凭据。公开来源拒绝访问时报告失败，不增加登录绕过逻辑。

### 6.2 解决 10 秒写超时

推荐同一 Go 进程使用两个回环监听：现有 8081 提供短请求，新增可配置的 8082 专供媒体。这不是新增微服务；共享同一个 playback.Service。媒体 server 不设置 10 秒总 WriteTimeout，改为每次写入推进 30 秒 deadline，并通过会话 context 控制总时长。保留请求头超时和连接限制。

使用标准 net/http 媒体 handler，便于直接操作 ResponseController；每次写入/flush 检查错误，不能只设置 timeout 名称而没有实际中断阻塞写。Go 对 WriteTimeout 和 SetWriteDeadline 的定义见 [net/http 文档](https://pkg.go.dev/net/http)。应用退出时同时关闭两个 listener 并取消会话，任何一个 listener 启动失败都报告失败。

开发环境在现有 `/api` 规则前增加更具体的 `/api/v1/music/streams` → 8082。生产反向代理同样分流，客户端只访问同域 URL，不暴露内部端口。

媒体 location 单独关闭 proxy_buffering、proxy_cache、gzip，设置 proxy_max_temp_file_size 为 0；read/send 空闲超时可先为 60 秒。还要核查公网链路是否包含额外 CDN 或隧道。Nginx 响应缓冲可能写临时文件，配置依据见 [Nginx 官方文档](https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_buffering)。

响应使用真实 Content-Type、Cache-Control: no-store；无法确定长度不伪造 Content-Length，不伪造 Accept-Ranges。上线保留总开关 MUSIC_PLAYBACK_ENABLED，默认关闭；只切换配置/版本回退，保留原站入口及已有文件。

## 7. 前端接入

新增 `frontend/src/musicPlaybackApi.ts`、`useMusicPlayer.ts`，把单个 audio 实例、请求取消和状态机从页面布局中分离。第一版 audio 随 MusicView 生命周期：音乐页内部 tab 切换继续播放，离开音乐路由停止。跨路由持续播放以后再移到全局播放器。

UI 由真实媒体事件驱动：loadedmetadata、playing、waiting、pause、timeupdate、ended、error。捕获 play() 拒绝；浏览器阻止自动播放时显示“点击继续播放”。首次用户点击后异步解析返回也可能失去激活状态，需真机测试，不能以 Promise 成功之外的状态宣称播放成功。

只在 ended 自动进入下一首；网络失败显示重试，不连续跳完整个歌单。切歌队列由本次选中列表固定，推荐跨日更新不突然改写正在播放的队列。进度条首版只读，音量与 audio.volume 同步。

新增音乐库 API/composable，读取全部所需分页后再提供当前的小曲库搜索，不能只拿第一页就宣称全库搜索。保留 loading/error/empty/ready 及来源 pending/failed 状态，失败不静默退回本地快照。音乐库与推荐使用统一 Track 映射，保留 externalId、partId、durationSeconds、availability。

## 8. 可执行任务拆分

每个阶段以明确产物和退出条件收口；下表工作量为单人集中开发估算，不包含外部平台拒绝或环境阻塞时间。

| 阶段 | 工作与改动位置 | 完成标准 | 估算 |
| --- | --- | --- | --- |
| P0 音源可行性 | 核验服务器系统/代理/数据库/工具；选择 1 首普通、1 首长音频、1 个失效或不可用样本；记录版本、音轨、格式、首字节和失败分类 | 至少一首目标曲目能经目标服务器获得独立音轨并在浏览器发声；否则记录阻塞，不批量搭建未经验证的解析路径 | 0.5–1 天 |
| P1 数据与契约 | 修复来源 URL 校验；统一 P1 ID；导入现有快照；补播放状态字段；扩展 OpenAPI | 真实数据库可查询导入曲目；收藏 ID 可映射；同步不覆盖播放检查结果；接口契约固定 | 1–1.5 天 |
| P2 后端播放 | 新增 playback.Service、Bilibili resolver、会话接口、媒体 handler、8082 listener、限流/取消 | 浏览器测试入口可播放一首，持续超过 10 秒；停止、失败和超时能释放资源；不落媒体文件 | 1.5–2.5 天 |
| P3 页面闭环 | 新增 player composable/API；MusicView 真正控制 audio；音乐库 API；收藏兼容 | 现有页面播放/暂停/切歌/音量正常，快速切歌仅最后一首发声，错误状态与实际一致 | 1–1.5 天 |
| P4 部署验收 | 代理分流；Linux Worker 生命周期；浏览器/真机；限流/慢客户端；开关与回退 | 真实站点访问成功且长音频不中断；原博客与登录回归通过 | 1–1.5 天 |
| P5 网易云 | 基于已有歌单单独验证合法可用音源，新增 resolver，不改已有元数据职责 | 可用曲目进入统一播放器；无法获得音源的曲目清楚显示仅原站播放 | 可行性 0.5–1 天，实现视结果再估 |

Bilibili 首版按约 5–8 个工作日排期。执行时先做 P0 小样本，再实施 P1–P4；网易云不阻塞 Bilibili 上线。Bilibili 全自动远端同步、任意 seek、HLS、跨路由播放、账号收藏同步均排后续。

建议新增模块位置：

```text
backend/internal/music/playback/          # 会话、资源上限、取消、错误分类
backend/internal/music/provider/bilibili/ # 音轨解析（不冒充已实现歌单同步）
backend/internal/platform/music_playback_http.go
backend/internal/platform/music_stream_http.go
frontend/src/musicPlaybackApi.ts
frontend/src/useMusicPlayer.ts
frontend/src/musicLibraryApi.ts
frontend/src/useMusicLibrary.ts
```

已有需要修改的文件：`backend/cmd/api/main.go`、`internal/platform/config.go`、`internal/platform/database_http.go`、`internal/music/provider/provider.go`、`internal/music/service.go`、`cmd/manage/main.go`（均在 backend 下），以及 `api/openapi.yaml`、`frontend/src/views/MusicView.vue`、`frontend/vite.config.ts`。数据库变更使用下一编号向前迁移，不修改已执行 SQL。

## 9. 验证与发布门槛

自动化测试聚焦真实边界：

- Source/Track/Embed URL 分别校验；P1 ID 和收藏迁移；unknown 不误判为不可播放或已可播。
- 会话重复创建、达到并发上限、创建后不连接、重复 stop、过期与服务退出。
- 使用本地假上游测试首字节前失败、传输中断、403/429、慢读、慢写、断连、跳转和 DNS 地址校验。
- PostgreSQL 测试验证失败不覆盖快照、同步不抹掉播放检查、推荐日期不被重写；SKIP 不算数据库验收成功。
- 前端测试覆盖迟到响应、快速切歌、play() 拒绝、暂停后过期、错误不自动跳歌。

基础回归命令（分别在对应目录运行）：

```powershell
# backend
go test ./...
go vet ./...
go build ./...

# frontend
npm run test:music
npm run type-check
npm run build
```

上线必须额外人工验证：一首完整播放、连续播放超过 10 分钟、快速切歌 20 次、4 路并发及第 5 路受限、网络断开/暂停恢复、桌面 Chrome/Edge 和真实手机 Safari/Chrome。首个可用音源的起播耗时先以 10 次样本记录，不将未知网络环境下的“秒开”写成承诺。

切歌/停止后目标 5 秒内释放旧连接和 Worker；观察应用及子进程 CPU/RSS，不应随切歌次数持续增长。对照运行前后文件变化检查应用、代理和工具是否产生媒体缓存。测试数据库 schema、构建旧哈希文件及诊断文件如有产生，列明供用户处理，不自动删除。

发布步骤：先在测试环境完成契约与数据库验收 → 目标服务器小样本验证 → 部署关闭播放开关的版本 → 核对代理和媒体就绪状态 → 开启 Bilibili → 观察错误率和资源 → 后续再开放网易云。出现问题关闭播放开关并回到原站入口，不反向删库或删除旧发布文件。

## 10. 本次产物

本次仅新增本计划，未修改业务代码、安装工具、同步真实曲库、下载媒体或更改服务器配置。未删除文件，未生成无用试验产物。代码测试和真实播放验收属于后续实施阶段，不将本计划视为播放功能完成证明。
