# 公开播放改造实施与验收

> 后续操作统一使用 pnpm，见 [包管理约定](package-manager.md)。下文验收部分的 npm 命令保留为当时执行记录。

日期：2026-10-02（Asia/Shanghai）。依据 [代码审计](music-public-playback-code-audit-20261002.md)，本轮完成 A–D 的代码和受控测试，以及 E 的独立 PostgreSQL、Nginx、音频与浏览器验收。用户确认尚未配置账号 Cookie，因此真实账号权益、会员音源和凭据在线轮换尚未实测。生产入口未启用本次版本。

## 已实现的行为

| 阶段 | 结果与代码 |
| --- | --- |
| A：完整性模型 | [policy.go](../backend/internal/music/playback/policy.go) 定义 full/preview/unknown、曲目时长、流时长及试听区间；[前端 DTO](../frontend/src/musicPlaybackApi.ts)、[播放器](../frontend/src/useMusicPlayer.ts)、[音乐页](../frontend/src/views/MusicView.vue) 全链路支持。试听结束停止并显示原站入口，不自动跳到下一首。未知完整性不因浏览器时长或有效 MP3 头升级为 full。 |
| B：公开策略和存储 | ready 前、开始流传输前和输出期间检查策略、授权有效期及凭据版本。账号来源仅在 licensed 模式且存在对应逐曲公开授权记录时允许 fallback。新增 [000004 迁移](../backend/migrations/000004_music_public_playback.up.sql)，按 sourceMode/audience/credentialVersion 保存播放检查；账号结果不改写公共 availability，试听不进入公开推荐。 |
| C：入口、配额和指标 | [可信代理配置](../backend/internal/platform/music_config.go) 只信任明确配置的代理，并从右向左处理转发链。新增每访客/IP 限额、会话/解析/流/转码容量、每流字节/时间上限和全站小时字节预算。生产通过 [PostgreSQL 控制](../backend/internal/music/playback_control.go) 共享速率、字节预算和资源租约。管理员指标端点返回聚合计数。 |
| D：账号接入 | [独立账号客户端](../backend/internal/music/provider/netease/account.go) 从私密文件读取 Cookie，限制大小、内容和权限；独立 HTTP Client，不使用 Cookie Jar、不跟随重定向、不继承环境代理。按凭据内容版本使旧账号会话失效，明确登录失效后停止重试该版本；连续拒绝触发冷却，账号请求预算可配置。 |
| E：隔离验收 | 当前源码在独立测试 schema 中通过数据库集成和 race；独立官方 Nginx 容器代理当前 API，真实公开音源经过 Go→FFmpeg→MP3；浏览器观察到音乐页播放和完整性提示。生产 CDN/反代和真实会员账号不在已通过范围内。 |

## 播放器体验边界

初次验收后的同日补做已实现跨页面持续播放与进度定位；当前边界如下：

| 项目 | 当前行为与验收结论 | 后续范围 |
| --- | --- | --- |
| 跨页面持续播放 | [App](../frontend/src/App.vue) 创建并提供应用级播放器；站内路由切换复用同一音频实例、队列、音量和播放状态。其他页面展示可暂停、停止和拖动的[迷你播放器](../frontend/src/components/MiniMusicPlayer.vue)，返回音乐页继续控制原播放器。 | 已补做并通过真实 RouterView 挂载/卸载回归；范围为同一标签页内的站内路由导航，不包括刷新、关闭标签页或跨标签页同步。 |
| 拖动进度 | [进度控件](../frontend/src/components/MusicSeek.vue) 支持拖动和键盘定位，松开后提交；后端接收 startSeconds 并使用 FFmpeg 从对应时间输出新音频流。seekMode=restart 表示支持此方式，仍不使用 HTTP Range。 | 已补做并通过真实 FFmpeg 音频定位测试。需配置 FFmpeg 且有可用时长；试听位置相对试听片段，只能在片段内定位。定位会重新缓冲，保留原暂停状态和音量。 |
| 歌单同步与完整播放 | ready / 267 首表示歌单同步结果，不代表全部歌曲均可播放或可完整播放。现有真实公开样本只证明观察时段内能播放，完整性仍为 unknown。 | 真实账号音源尚未实测；配置条件具备后进行受控抽样，分别记录可播性、试听信息与完整性证据，抽样结果不能外推至全部歌曲。 |

定位前停止旧会话，新会话重新执行原有来源、权限、完整性及配额检查；快速切歌会串行释放和创建，过时响应不会替换当前播放器。每次定位计入现有会话创建配额（默认每访客/IP 各 10 次/分钟）。FFmpeg 从受控输入流解码并跳过前缀，不落盘缓存音源；远距离定位可能较慢，沿用 15 秒首音频超时。未知完整性音源可用曲目时长作为定位参考，但可能在目标位置前已结束，定位失败不代表完整音源可用。

补做验证：前端音乐测试 70 项、type-check、内容检查及 development 模式构建通过；后端 `go test ./... -count=1`、`go vet ./...` 和 playback/platform 包的 `go test -race` 通过（本机未配置数据库，数据库集成不在此次验证范围）。新增回归覆盖站内路由切换、暂停后定位、向前/向后定位、音量保留、停止时的迟到响应、试听越界、无 FFmpeg/无时长拒绝，以及 HTTP→真实 FFmpeg→MP3 解码后的目标音频片段。正式 SEO 构建因本机未配置 VITE_SITE_URL、SEO_API_ORIGIN 未执行成功，已按项目要求使用 development 模式完成打包。未部署生产，未补做真实账号音源、生产反代或浏览器人工听验。

补做保留产物：`frontend/dist/` 中的 development 构建及既有旧资源；复用 `../.validation-cache/go/` 和 `../.validation-cache/account-private-file-fixtures/`（假凭据），以及既有前端类型检查/Vite 缓存。音频测试在内存生成并解码，无新增音频文件；未删除任何文件。

## 完整性和来源规则

公开 outer 解析器默认 unknown，曲目元数据时长只放在 trackDurationSeconds；没有实际流时长证据时 streamDurationSeconds 为 null。账号接口的 freeTrialInfo 起止值按毫秒转换为秒，明确标为 preview。缺失或 null 的试听字段也不直接证明 full。当前网易云解析器不会仅凭返回 URL 或 time 字段标记完整音源；full 模型留给有明确证据的可信解析器，并已完成模型和 UI 测试。

public_only 为默认模式，保留既有公开来源路径，不调用账号客户端。配置 MUSIC_PUBLIC_GRANTS_FILE 后，public 来源也受逐曲清单限制；空清单不放行任何曲目。licensed 模式必须提供该文件，账号来源还必须匹配 sourceMode=account、audience=public 和未过期记录。账号能取到音源不自动形成公开授权。程序记录操作员填写的依据，不能自行证明其法律效力。

前端只提交 trackId 和可选 startSeconds，不能自行填写 audience、mediaKind 或授权字段。公开 DTO 不包含 Cookie、账号标识、凭据版本、上游 URL 或内部判断依据。SOURCE_AUTH_EXPIRED 仅用于内部诊断和管理员聚合指标，公开映射为 UPSTREAM_UNAVAILABLE。HTTP 403/风控拒绝不会关闭凭据；音源接口的 401/301 也不单独作为账号失效证据，需账号状态接口提供明确结果。

短时音源访问拒绝时，输出音频前最多重新解析一次，并重新检查策略、完整性类型和凭据版本；重新解析也受解析并发及共享租约限制。已经开始输出后不拼接 JSON 或偷偷更换音源。补做后支持 seekMode=restart 的按秒重建流定位；不具备条件时仍为 none，不伪造 Range/206。

## 配置与迁移

程序读取进程环境，不自动加载 .env。新版本依赖 000004 迁移，ready 检查已覆盖新表和字段。已有迁移没有改写；上线前按既有管理流程执行 `manage migrate`，不要把测试库连接用于生产。

完整变量见 [backend/.env.example](../backend/.env.example)。新增主要配置：

| 配置 | 默认值/意义 |
| --- | --- |
| MUSIC_PLAYBACK_POLICY | public_only；账号公开 fallback 使用 licensed |
| MUSIC_REQUIRE_FULL | false；true 时 unknown 和 preview 都不 ready |
| MUSIC_PUBLIC_GRANTS_FILE | 空；操作员控制的 JSON 授权清单路径 |
| MUSIC_NETEASE_COOKIE_FILE | 空；服务器私密 Cookie 文件路径 |
| MUSIC_WEB_ROOT | 可配置实际 Web 根目录，防止私密文件位于其中 |
| MUSIC_TRUSTED_PROXIES | 空；不信任转发头。反代部署需填写准确代理 IP/CIDR，拒绝信任全部地址 |
| MUSIC_OWNER_REQUESTS_PER_MINUTE / MUSIC_IP_REQUESTS_PER_MINUTE | 各 10；同 NAT 共享 IP 预算，每访客另有独立预算 |
| MUSIC_MAX_SESSIONS / MUSIC_MAX_RESOLVERS / MUSIC_MAX_STREAMS / MUSIC_MAX_TRANSCODERS | 128 / 2 / 4 / 2 |
| MUSIC_MAX_STREAM_BYTES / MUSIC_HOURLY_BYTES | 256 MiB / 1 GiB |
| MUSIC_MAX_STREAM_DURATION | 6h |
| MUSIC_ACCOUNT_REQUESTS_PER_MINUTE / MUSIC_ACCOUNT_REJECT_THRESHOLD / MUSIC_ACCOUNT_COOLDOWN | 20 / 3 / 1m |

Cookie 和授权文件须放在仓库、Web 根目录之外。运行配置只包含路径；路径会解析符号链接并检查仓库范围，Web 根目录不在仓库内时应显式配置 MUSIC_WEB_ROOT。Cookie 文件必须为非空、最多 16 KiB 的普通文件，包含 MUSIC_U，不能包含嵌入换行或控制字符；Linux 权限需限制为服务账号可读，不能给 group/other 访问。Windows 部署还须由操作员配置文件 ACL。新增忽略规则覆盖 .env.production 等环境文件及专用凭据扩展名，不代表已清除任何历史秘密。

可在服务器使用用户配置目录，例如 `$HOME/.config/blog-music/`，用编辑器填写 Cookie，再设置相应文件权限。不要把明文 Cookie 放入 shell 命令参数、前端、日志、仓库或聊天。替换文件内容后，下次解析读取新版本；旧账号会话在开始或继续输出时被拒绝。授权清单在启动时加载，授权撤销/变更需重启对应实例；到期检查实时执行。

授权文件的结构示意（示意值不是已核实的授权）：

```json
[
  {
    "trackId": "netease:123",
    "sourceMode": "account",
    "audience": "public",
    "evidence": "由操作员填写已核实的公开播放授权记录标识",
    "expiresAt": "2026-12-31T00:00:00Z"
  }
]
```

`GET /api/v1/admin/music/playback-metrics` 需要现有管理员会话，返回实例级请求、拒绝、错误码计数、准备/首音频耗时累计、活动解析/流/转码、输出字节及重新解析次数。没有私人账号标识或临时 URL。

共享配额使用 PostgreSQL 事务锁及有界槽位；租约到期可以复用槽位，正常结束主动释放。会话内容和终态保留仍在实例内，扩容时 API 与媒体必须路由到同一实例，且各实例使用一致的配额配置。[Nginx 片段](../deploy/music-streams.nginx.conf) 说明按客户端地址稳定路由，避免新建会话时还没有 Cookie 导致后续换实例。账号冷却/失效状态为客户端实例内状态，账号请求预算跨实例共享；重启后重新检查账号状态。

## 验证结果

- 本机 `go test ./... -count=1`、go vet、go build 通过；本机未连接数据库。Cookie 文件测试显式配置 MUSIC_TEST_ARTIFACT_DIR，所有假凭据 fixture 保留。
- 前端 `npm run test:music` 67 项通过，type-check 通过；实际 SFC 回归验证完整音源、试听区间和未知完整性标识，composable 验证试听结束不自动切歌。
- 服务器 PostgreSQL 集成实际 PASS，包含共享并发租约、跨实例字节/速率预算、不同 IP 与同 NAT，以及私人账号结果不污染公开 availability、试听不进入推荐。没有以 SKIP 作为通过。
- 服务器 race 检查通过；文件权限、凭据轮换/失效、403 冷却、账号重定向拒绝、媒体 Cookie Jar/Authorization 隔离、字节上限、单次 URL 重新解析均有回归。
- 独立 Nginx→Go→FFmpeg 实测 HTTP 200 / audio/mpeg，有界读取 65,536 字节；观察到 FFmpeg 子进程 68145，1 秒解码为 176,400 字节非全零 PCM。音频仅在内存处理，没有保存媒体文件。
- 目标歌单再次同步为 ready / 267 首。浏览器观察到公开样本播放至 0:47 并可暂停，之后页面选择发生变化；该次暂停不计为完整的保持/恢复复验。随后实际页面观察到目标歌单《祂的指引》持续播放，以及《直到大地变成一颗酸橙》播放至 0:34，均显示“完整性未确认”。这些观察不能证明全部 267 首可播或完整。
- 最终修正后的独立 api-final 再次通过真实媒体检查，unknown + streamDurationSeconds=null，读取 65,536 字节、停止返回 204。最后两项流入口分类/刷新并发修正通过定向回归，不重新进行无关的全歌单音源探测。
- OpenAPI JSON 和 git diff --check 通过。应用及隔离 Nginx 日志未发现本轮检查的 Cookie/响应/签名 URL sentinel；真实账号 Cookie 未使用，不能把这个结果视为生产所有日志的秘密扫描。

本轮服务器原先未安装 Nginx，已有容器为其他应用。为完成反代验证，另拉取 Docker 官方 nginx:1.28-alpine 并启动仅回环入口的独立容器，使用仓库配置片段，只替换验证端口。没有更换原服务、原容器或生产反代。

## 尚需实际配置的条件

用户已明确 Cookie 未配置。真实账号登录状态、会员权益响应、会员试听语义和在线轮换需在提供私密文件路径后完成受控抽样，不能根据模拟测试宣称会员歌单已可公开播放。公开账号来源还需要操作员提供逐曲授权依据。现有未知完整性来源不会被强行标记 full。

生产 CDN、生产反代入口及其日志尚未核验，也未部署本次版本。物理扬声器人工听验、移动端和长时间压力测试未完成。可审查代码和隔离验收结果已经具备，以上条件应在生产启用评估时补齐。

## 收尾与保留产物

本轮临时 API、Nginx 容器、SSH 转发和 Vite 已停止，28081/28082/48081 及本机 38081/5179 已释放。原隔离服务 PID 13637 的 ready 仍为 200。没有删除任何文件、容器、镜像或 schema。

保留产物如下（服务器路径相对于服务器用户主目录，本机路径相对于项目根目录）：

- 本机 `../.validation-cache/music-public-playback-20261002-source.tar`、`account-private-file-fixtures/`（全是假凭据）、`music-public-capability-20261002.jpg`，以及复用的 Go/TypeScript 缓存。
- 服务器 `blog-web/music-public-validation-20261002-8c71/`：源码副本、source.tar、api/manage/api-final、历史 PID 文件、构建/测试/race/边界日志、api.log/api-final.log、nginx-image.log/nginx-runtime.log、Nginx 配置、人工样本 JSON、proxy-validation-result.json/final-validation-result.json、fake-credential-fixtures。后续的只读回归日志亦保留在此目录。
- 已停止的 Docker 容器 `music-public-validation-20261002-8c71` 和官方 nginx:1.28-alpine 镜像。
- `blog_test` 新增 schema：`backend_test_81e17325535bdeb6`、`backend_test_1233cc3e3ae285fb`、`backend_test_858b74da64b0a054`、`music_public_validation_20261002_8c71`。

原审计文档及此前同步/播放验收记录保留。源码、测试与本文作为同一次功能改造交付；生产启用另行评估。
