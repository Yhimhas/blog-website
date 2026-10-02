# 音乐页公开播放：代码现状、缺口与改造设计

日期：2026-10-02（Asia/Shanghai）。分析基线：Git `203e90d`。

后续实施情况见 [公开播放改造实施与验收](music-public-playback-implementation-20261002.md)。本文保留审计时的代码现状；其中“尚未实现”不代表后续工作区版本。真实账号 Cookie 尚未配置，会员音源和生产启用仍需完成对应实测与配置。

本文针对“音乐页不区分访客和管理员，希望访客也能收听”的需求，分析当前本地代码。正文代码路径均相对于项目根目录。本文是技术审查和设计建议，不代表已实现账号音源接入，也不确认任何歌曲的对外播放授权。

## 1. 结论

目前已经实现匿名访客通过本站服务端代理播放公开音源，网易云使用官方 outer 接口，不使用网易云登录 Cookie。现有代码不是“已经接入账号但没有给访客开放”，而是根本没有账号凭据和账号音源解析链路。

最关键的缺口有五项：

1. 没有完整音源、试听、未知完整性的数据模型；有效音频头就可以进入正常播放流程。
2. 没有账号权限不足、账号凭据失效的独立诊断和处理。
3. 没有区分“账号可以获取”与“允许提供给本站访客”的服务端播放策略。
4. Nginx 反代场景下，按 RemoteAddr 限流可能把所有访客归入同一个 IP。
5. 公共接口的限流和并发限制已经存在，但缺少账号级熔断、流量预算和多实例共享控制。

保持统一音乐页完全可行。需要新增的是服务端音源策略和清晰的播放状态，不必因此给访客增加登录步骤或设计两套音乐页。

## 2. 范围与证据边界

- 阅读了 Vue 音乐页、播放 composable、API DTO、Go 网易云 Adapter、音频 Resolver、播放会话、媒体转发、配置、相关迁移和 Nginx 配置片段。
- 本轮仅新增本文，不修改业务代码，不连接服务器，不读取真实账号 Cookie，不修改生产配置。
- 既有 [服务器复验记录](netease-playlist-server-validation-20261002.md) 描述了隔离环境的 267 首元数据同步及单个公开样本播放。它不能证明 267 首都有完整音源，也不是本轮生产状态检查。
- 本文没有进行全 Git 历史秘密扫描；“当前没有 Cookie 接入代码”不等于证明所有历史提交、服务器日志和备份都没有敏感信息。

## 3. 当前调用链

```text
MusicView.vue
  -> useMusicPlayer.play()
  -> POST /api/v1/music/playback-sessions { trackId }
  -> 创建或复用本站 music_playback Cookie
  -> playback.Service.Create()
  -> music.Service.Find()：查已启用来源中的活动曲目
  -> netease.AudioResolver.Resolve()：公开 outer 接口
  -> GET /api/v1/music/playback-sessions/:id：等待 ready
  -> HTMLAudioElement 请求 /api/v1/music/streams/:id
  -> Go 重新请求并验证上游音频
  -> 必要时 FFmpeg 管道转码
  -> 返回本站音频流
```

音频流和普通 API 使用不同监听端口；`deploy/music-streams.nginx.conf` 将媒体路由反代到回环地址的 8082 端口。

### 3.1 核心代码索引

| 代码 | 当前职责 | 对后续改造的意义 |
| --- | --- | --- |
| `frontend/src/views/MusicView.vue` | 音乐库与统一播放器 UI | 增加试听、完整性未知和不可播放提示 |
| `frontend/src/useMusicPlayer.ts` | 创建、轮询、切歌、暂停、结束与释放 | 保存播放类型，避免试听结束被当作完整播放结束 |
| `frontend/src/musicPlaybackApi.ts` | PlaybackSession DTO、错误文案、同源流地址验证 | 增加权益和试听字段，保留机器可读错误码 |
| `backend/internal/platform/music_playback_http.go` | 匿名会话 Cookie、Origin、输入限制、媒体入口 | 保留匿名模式，修正可信客户端 IP 提取 |
| `backend/internal/music/playback/service.go` | Resolver、Audio、View、会话、限流与并发 | 放置统一策略裁决及新的状态字段 |
| `backend/internal/music/provider/netease/audio.go` | 公开 outer 音源解析 | 保留公开解析器；账号解析器应独立 |
| `backend/internal/music/provider/netease/client.go` | 公开歌单和单曲详情补齐 | 元数据同步与播放权限分开处理 |
| `backend/internal/music/playback/stream.go` | DNS/IP/URL 防护、字节验证、媒体流输出 | 保留请求头白名单和 CDN 凭据隔离 |
| `backend/internal/music/playback/transcode.go` | FFmpeg 管道转码 | 不能通过转码证明音源完整性 |
| `backend/internal/music/playback_store.go` | 回写播放可用性 | 当前 available 语义不足，不能承载权限判断 |
| `backend/internal/platform/config.go` | 环境配置与生产 HTTPS 检查 | 当前没有网易云账号配置 |
| `backend/internal/recommendation/recommendation.go` | 基于 available 曲目生成推荐 | 未来需要避免私人权限污染公开推荐 |

### 3.2 两种 Cookie 必须区分

当前的 `music_playback` 是本站生成的匿名播放会话归属标识，采用 HttpOnly、SameSite=Strict，生产模式要求 Secure；有效期 7 小时。它不是网易云 Cookie，也不能代表网易云会员权益。

`blog_session` 是既有站内账号会话。公开播放路由没有挂 `requireSession` 或 `requireAdmin`；现有后台鉴权不妨碍访客播放。

未来如果接入网易云凭据，它只能在服务器内部账号客户端使用，不能替换或复用上述两种本站 Cookie。

## 4. 已有安全与稳定性能力

| 能力 | 代码证据与边界 |
| --- | --- |
| 不接受访客任意上游 URL | 创建接口只接收 trackId，数据库要求曲目属于启用且活动的来源；这不是曲目授权白名单 |
| 上游地址不进入前端 | View 返回本站 streamUrl，内部 Audio 保存上游 URL 和 Headers |
| 会话归属校验 | Get、Stop、acquire 校验 sessionId 对应 owner；单独复制流 URL 不够 |
| 来源校验 | 写操作要求准确 Origin；流入口拒绝显式异源 Origin 与 cross-site 请求；脚本客户端仍能伪造这些头 |
| SSRF 防护 | 只允许 HTTPS 和受限端口，DNS 结果检查并连接已验证 IP，阻止私网、回环与部分特殊地址，无环境代理 |
| 网易云重定向限制 | 限定官方主机，限制跳转次数；部分 HTTP CDN Location 在连接前升级为 HTTPS |
| CDN 请求头隔离 | Stream 只从内部 Headers 复制 User-Agent、Referer，不复制 Cookie、Authorization 或访客请求头 |
| 有界解析与并发 | 全局解析 2、流 4、转码 2；每 owner 至多一个活动会话；owner/IP 每分钟 10 次创建计数 |
| 生命周期 | preparing 25 秒、ready 1 分钟、streaming 最长 6 小时；终态保留 5 分钟；会话容量上限 128（含解析计数） |
| 输出验证 | 校验上游响应和媒体签名；中途失败中止连接，避免追加 JSON 污染音频 |
| 日志与缓存 | API 日志归一化播放会话路径；媒体 no-store；仓库 Nginx 片段关闭媒体缓存、缓冲和 access_log |
| 资源释放 | 停止会话、客户端断开、切歌、组件销毁均有取消或释放链路 |

以上仅说明仓库实现。Nginx 主配置、上层 CDN、代理 error_log 和生产监听地址是否符合设计，本轮未核验。

## 5. 缺口与优先级

### P0：在开放账号音源之前完成

#### 5.1 音频有效不等于完整歌曲

`netease.AudioResolver.Resolve()` 只读取音频前缀、判断格式，返回的 Duration 来自曲目元数据。`Stream()` 在首次成功写出音频后调用 Checked，并把曲目标记为 available。

当前 Audio、View、PlaybackSession 都没有 full/preview/unknown 字段。因此即便上游提供的是合法格式的试听片段，也会进入“正在播放”；浏览器 ended 事件将显示“播放结束”并播放下一首。

这是模型无法区分试听的确定缺口，不是本轮已证实某一首歌曲返回试听。不能仅凭 HTTP 200、MP3 头、解码成功、元数据时长或转码成功判断完整性。实测时长差异只能用于异常提示，不能可靠推断授权或试听类型。

#### 5.2 账号客户端和凭据生命周期尚不存在

当前 Config 没有网易云凭据字段，NewAudioResolver 明确禁用 Cookie Jar，元数据客户端也没有登录凭据。直接增加一个环境变量并不会实现会员音源解析。

如后续接入，应新增服务器私密文件读取、非空/大小校验、轮换与失效处理；只把文件路径放入运行配置，凭据文件置于仓库和 Web 根目录之外。服务账号只读，限制文件权限，不通过前端、命令行参数或 API 回显传入凭据。

根 `.gitignore` 已忽略 `.env`、`.env.local`、`.env.*.local`，但不能据此认为任意私密配置文件都被覆盖；例如 `.env.production` 不匹配这几条。新增凭据路径必须检查忽略规则及是否已经被 Git 跟踪。忽略规则也不能清除历史记录。

#### 5.3 缺少公开播放策略

“来源启用、曲目存在”只是当前播放条件的一部分，不等于允许公开提供该音源。需要对音源做服务端策略裁决，并且在生成 ready 会话之前执行。

建议统一匿名页面采用 public_only 默认策略：公开访问链路和允许网站播放的范围分别核验。对于需要账号获取的音源，只有显式配置的公开播放授权依据才允许提供给访客。账号能获取但没有对应依据时，保留原站入口。

这项设计不要求把管理员和访客展示成两种页面。它根据音源许可范围裁决，而不是把所有访客隐式视为账号所有人。技术实现本身不能补足授权依据。

#### 5.4 没有可靠的错误分类

播放 Failure 目前有上游不可用、限流、超时、无音源、格式/转码错误等，但没有权限不足和凭据失效。网易云公开音频分支对若干拒绝响应统一返回 UPSTREAM_UNAVAILABLE。

账号链路应区分单曲权益不足与账号登录失效；HTTP 403 或风控拒绝不能一概认定 Cookie 失效。只有明确账号状态证据才能关闭凭据并提示维护。访客只需看到“此音源暂不可用”，不必看到账号标识或凭据细节。

### P1：公开访问稳定性与安全加固

#### 5.5 反代后的 IP 限流不准确

`bindPlayback()` 用 `net.SplitHostPort(c.Request.RemoteAddr)` 获取限流 IP，而 API Router 设置 `SetTrustedProxies(nil)`。如果 API 经本机 Nginx 转发，通常看到的是代理回环 IP；结果可能是不同访客共享每分钟 10 次创建额度。

修正应同时约束部署和代码：明确可信代理地址、由入口覆盖客户端地址头、只在连接来自可信代理时解析该头，并把可信 IP 传入 Service.Create。不能直接相信用户提交的 X-Forwarded-For，也不能只改 Gin 设置而继续读 RemoteAddr。需要测试直接伪造头、真实代理链、IPv4/IPv6 与同一 NAT 的场景。

#### 5.6 现有防滥用不是账号保护策略

匿名 Cookie 可被新建，Origin 可被非浏览器伪造，现有控制主要约束单进程容量。缺少账号级请求预算、连续拒绝后的熔断、全站输出字节预算、可配置限制和异常指标。

当前媒体最长允许 6 小时，未设置总输出字节上限；请求来自库中曲目降低了风险，但不等于上游长度可被无限信任。可结合曲目预计长度设置宽松上限，同时保留未知时长处理。

发生账号失效时，应暂停该账号解析并尝试符合策略的公开来源，避免每个访客继续触发登录失败；公开来源也失败时保持失败状态，不无限重试。

#### 5.7 全局 available 无法表示不同权限

`music_items` 保存曲目级 availability 与最近播放错误；没有按来源模式或权益范围分开。推荐服务又以 available 为候选条件。

如果未来私人账号播放成功也回写同一个 available，会污染公开推荐判断。应将公开可用性与账号解析结果分开；至少新增可用性作用域，长期可用独立音源能力表。不能把某账号的成功结果直接当成全部访客都可以播放。

#### 5.8 日志与部署需要端到端验证

应用日志已有一定裁剪，媒体 Nginx 片段关闭了 access_log，但普通 API 代理日志可能仍记录带 sessionId 的 URL；生产 CDN、error_log 和调试日志也未检查。

账号接入应测试所有跳转目标不收到 Cookie/Authorization，日志不含凭据、上游签名 URL、账号标识和原始响应体。上游账号客户端与 CDN 媒体客户端使用独立实例；账号客户端默认拒绝重定向，不共用 Cookie Jar。

### P2：体验和规模能力

- 当前明确 `seekMode: none`，Range 被忽略并返回 200，不支持拖动；若未来支持必须实现真实 Range/206 语义，不能只启用前端进度条。
- 会话、限流与并发计数都在进程内；重启丢失、多实例不共享。扩容前需要明确粘性路由或共享状态与全局配额方案。
- ready 状态到实际流请求存在二次访问；上游短时 URL 可能失效。可以增加一次受控重新解析，但仍须重新检查策略，禁止无限重试。
- 应记录准备耗时、首次音频延迟、错误码聚合、拒绝率、活动流、转码占用与输出字节，不记录敏感请求数据。

## 6. 建议的数据契约（尚未实现）

生命周期、音源完整性和错误原因应分开，避免把试听当作失败，也避免把可解码误称为完整。

```ts
type MediaKind = 'full' | 'preview' | 'unknown';
type FailureReason =
  | 'ENTITLEMENT_REQUIRED'
  | 'SOURCE_AUTH_EXPIRED'
  | 'PUBLIC_PLAYBACK_NOT_ALLOWED'
  | 'UPSTREAM_ACCESS_RESTRICTED';

interface PlaybackCapability {
  mediaKind: MediaKind;
  trackDurationSeconds: number | null;
  streamDurationSeconds: number | null;
  previewStartSeconds: number | null;
  previewEndSeconds: number | null;
}
```

保留现有 preparing/ready/streaming/ended/failed/stopped/expired，会话增加 capability。上例 FailureReason 是新增项，不替换已有超时、限流等错误。SOURCE_AUTH_EXPIRED 建议作为内部诊断，公开响应映射为稳定的音源暂不可用文案；只在维护渠道报告凭据失效。

后端内部另存 `sourceMode`（public/account）、`audience`（public/private/blocked）、判断依据和凭据版本；不把账号标识、Cookie、上游 URL 序列化到公开 View。

| 条件 | 后端行为 | 前端展示 |
| --- | --- | --- |
| 完整性有明确证据，且通过公开策略 | ready + full | 正常播放，可标明完整音源 |
| 明确试听且允许提供 | ready + preview + 有依据的区间 | 试听标识、试听进度和试听结束 |
| 已有公开链路允许播放，但完整性无法证明 | ready + unknown | 完整性未确认，不显示完整标识 |
| 单曲权限不足 | failed + 权限分类 | 暂无可用音源，前往原站 |
| 账号凭据失效 | 账号熔断；可尝试允许的公开源 | 使用公开源，或显示暂不可用 |
| 没有满足公开策略的来源 | failed + 策略拒绝 | 原站收听入口 |

unknown 只描述完整性未知，不能绕过 audience 检查。若产品要求站内只播完整音源，则 unknown 也应退回原站。试听的自动切歌策略需要显式定义；无论是否继续下一首，都不能记录成完整收听。

## 7. 建议实施顺序与代码改动

| 阶段 | 修改位置 | 交付与验收 |
| --- | --- | --- |
| A：先补状态模型 | playback/service.go、musicPlaybackApi.ts、useMusicPlayer.ts、MusicView.vue | full/preview/unknown 贯穿，旧公开解析器默认 unknown，不虚构完整性 |
| B：公开策略 | 独立 playback policy 模块、playback_store.go、新增迁移、推荐查询 | ready 前检查策略，账号成功不污染公开 availability；现有迁移不改写 |
| C：入口与配额 | music_playback_http.go、database_http.go、config.go、部署配置 | 可信代理 IP、可配置限流、指标及流量上限 |
| D：可选账号接入 | 独立账号客户端、config.go、cmd/api/main.go | 私密文件配置、独立 HTTP 客户端、明确状态判别、凭据轮换与熔断 |
| E：实环境验收 | 隔离环境及新验证记录 | 实际 Nginx、浏览器、CDN 头和日志；最后才评估生产启用 |

账号 Resolver 的接口选择和权益字段必须先通过文档或受控实测确认；当前公开 outer Adapter 无法提供足够依据，本文不虚构一个保证返回完整会员音源的接口。D 阶段不是让访客完整播放会员歌曲的无条件承诺。

## 8. 必要测试清单

1. 输入有效音频头但带试听标记，必须返回 preview，页面显示试听结束。
2. 有效音频但缺少完整性证据，必须是 unknown，不能根据曲目时长升级 full。
3. 权限不足、账号失效、风控、超时、限流分别映射；403 不自动变为失效。
4. 账号客户端拒绝未批准跳转；媒体 CDN 请求永远没有账号 Cookie、Authorization 和访客 Cookie。
5. 公开 DTO、API/代理日志及错误响应中不包含测试凭据和签名 URL。
6. 无公开策略许可的账号音源不能 ready，前端不能靠自填字段改变策略。
7. 私人解析成功不能改变公开推荐候选；账号失效不永久标记所有歌曲无音源。
8. 两个访客经可信反代分别计数；伪造转发头不能绕过限流；同 NAT 行为有明确预算。
9. 会话越权、URL 重放、单 owner 并发、容量耗尽、客户端断开、晚到错误及 FFmpeg 释放回归。
10. 账号连续失效触发熔断；轮换后旧会话和旧凭据缓存按既定策略处理。

## 9. 本轮验证结果与产物

| 检查 | 本轮结果 |
| --- | --- |
| `frontend` 中运行 `npm run test:music` | 60 个测试通过，0 失败，0 跳过 |
| `backend` 中运行 `go test ./internal/music/... ./internal/platform/... -count=1 -json` | 6 个包通过；本地真实 FFmpeg 测试未报告跳过 |
| 真实外部服务用例 | TestRealAudio、TestLivePlaylistCompletion 跳过；本轮没有重新验证真实平台音源 |
| 生产、数据库集成与浏览器实播 | 本轮未执行；上述定向 Go 测试不包含 storage 包的数据库集成 |

首次 Go 测试因默认构建缓存目录无访问权限而未运行成功，随后将 GOCACHE 指向已有的 `../.validation-cache/go` 并重试通过。测试通过仅说明现有行为的回归测试通过，不代表本文提出的缺失能力已经具备。

新增交付物只有本文。没有删除文件，没有新增实验脚本、音频文件或服务器资源；已有 `../.validation-cache/go` 在测试中被复用并可能新增构建缓存，这些属于可再生验证产物，按用户要求保留。
