# 网易云完整歌单补齐实现与验证

> 后续操作统一使用 pnpm，见 [包管理约定](package-manager.md)。下文验收部分的 npm 命令保留为当时执行记录。

日期：2026-10-02。依据 [技术设计](netease-playlist-sync-and-playback-technical-guide.md) 实现完整 ID 清单、缺失详情补齐、顺序重建和提交前复查。设计文档中“本次仅编写文档”的说明属于 2026-09-30 的历史交付范围；本记录说明当前源码变更。

2026-10-02 后续更新：用户指出 Remote SSH 可以连接后，重试直接 SSH 成功。当前代码已在服务器独立副本和 `blog_test` 的新 schema 中完成实际同步，目标歌单为 ready / 267 首；PostgreSQL 集成测试、真实 FFmpeg 转码与浏览器鼠标暂停/恢复、切歌及失败提示均通过。详情及保留产物见 [服务器复验记录](netease-playlist-server-validation-20261002.md)。下文“本次验证结果”和“验证边界”保留首次本地实施时的结果，服务器后续结论以上述记录为准。未替换生产服务。

## 实际接口验证

本机直接请求固定的 `music.163.com` 域名，关闭环境代理，不携带 Cookie，不自动跟随重定向。不对多个候选接口进行批量轮询。

| 请求 | HTTP / 业务码 | 实际结果 |
| --- | --- | --- |
| 旧 `/api/playlist/detail?id=595975585` | 200 / 20001 | 没有可用于补齐的 ID 清单 |
| `/api/v6/playlist/detail?id=595975585&n=2000&s=0` | 200 / 200 | trackCount=267、trackIds=267、tracks=6 |
| 同一新版接口，公开榜单 `3778678` | 200 / 200 | trackCount=200、trackIds=200、tracks=200 |
| `/api/v3/song/detail`，`c` 包含公开样本 `33894312` | 200 / 200 | songs=1 |

新版接口与单曲详情接口返回 JSON，但 Content-Type 为 `text/plain; charset=UTF-8`。客户端在固定的两个元数据 endpoint 上接受 `application/json` 和 `text/plain`，仍要求 HTTP 成功、业务成功、严格 JSON 解析及字段校验；HTML、无效 JSON 与超大响应均失败。这不修改媒体流的音频格式检查。

接口路径先参考适配器本身的源码：[playlist_detail.js](https://raw.githubusercontent.com/neteasecloudmusicapienhanced/api-enhanced/master/module/playlist_detail.js)、[song_detail.js](https://raw.githubusercontent.com/neteasecloudmusicapienhanced/api-enhanced/master/module/song_detail.js)。该项目采用自己的请求封装；本次直接 GET 协议是否可用以以上实际响应及下方 Go 实测为依据，不能从第三方源码单独推断。未加入第三方代理、签名服务或登录凭据。

## 当前实现

[netease/client.go](../backend/internal/music/provider/netease/client.go) 保持原有 Adapter 接口，内部流程为：

1. 校验平台、歌单 URL 和 externalId 一致性，获取新版详情。
2. 先判断 HTTP/业务状态，再解码成功字段。继续使用已有错误分类和最多 2000 首、每次最多 2 MiB 的限制。
3. 存在 trackIds 时，先要求原始清单数量等于 trackCount，校验所有 ID，再去重并保留首次出现的位置。ID 全程为十进制字符串，解析和请求序列化使用 `json.Number`，不经过 float64。
4. 已有 tracks 建立索引，只请求缺失项。每批最多 50 首，串行执行，不自动重试，所有请求共享同步任务的 30 秒上下文预算。完整 tracks 的历史 result/playlist 响应仍可解析；完整空歌单保留为空列表。
5. 每批要求返回的曲目属于请求集合、所有请求 ID 都有合法详情；冲突重复项、缺失项、下架后无详情、超时、限流和业务拒绝均返回错误，不返回部分候选结果。
6. 按原始 ID 顺序重建 Track。进行过补齐时，再获取一次详情，逐项核对 trackCount 和原始 ID 清单/顺序；发生变化返回 `INCOMPLETE_PLAYLIST`。
7. 完整结果通过 `provider.Validate` 后才交给既有 `ReplaceSnapshot` 事务。元数据 availability 保持 unknown；已有播放检查结果仍由原事务逻辑保留。

[诊断上下文](../backend/internal/music/provider/diagnostics.go) 和 [Service](../backend/internal/music/service.go) 使同步日志包含 requestId、sourceId、runId、HTTP 状态、业务码和可解析的预期/实际数量。日志不包含 Cookie、上游 msg、完整响应或临时音源 URL；失败状态落库失败也会记录。

数据库列、同步状态枚举和 Adapter 公共接口无需迁移。既有公开 `syncErrorCode`、首次失败/旧快照失败提示和播放器行为继续使用；[OpenAPI](../api/openapi.yaml) 补充 `INCOMPLETE_PLAYLIST` 包含 ID 清单、详情缺失和补齐期间歌单变更的语义。

[musicLibraryApi.ts](../frontend/src/musicLibraryApi.ts) 继续读取全部分页，并新增总数非负/不超过 2000、每页不超过 50、跨页总数稳定、ID 不重复、来源一致及最终数量恰好一致的检查。分页变化或不完整时拒绝本次结果，不将部分列表呈现为成功加载。该检查无法识别同数量、无重复 ID 的并发整版替换；它也不能单独证明后端快照等于网易云最新状态。

已有 [MusicView.vue](../frontend/src/views/MusicView.vue)、[useMusicPlayer.ts](../frontend/src/useMusicPlayer.ts) 及 Go 播放服务已经提供统一播放器、暂停/恢复、缓冲/失败、重新播放、原站入口及切歌 generation 隔离。本次没有修改播放协议或加入官方歌单 iframe，也没有同步后自动批量检测音源。元数据完整不代表 267 首都可播。

## 本次验证结果

普通测试不访问真实接口或数据库。真实上游检查通过单独 opt-in 测试执行，仅在内存读取元数据，不提交数据库快照。

| 检查 | 本次结果 |
| --- | --- |
| 新 Go Adapter 真实读取目标歌单 `595975585` | **通过**：267 首、2 次详情请求和 6 批单曲请求；复查 ID 清单与顺序一致；约 1.35 秒 |
| 按顺序连接 ID 的 SHA-256 | `20dc270b8c1c57ca69f2c09db45be4e902723ba99ab8917e5ad3aefd6b8e5488` |
| backend `go test ./... -count=1` | 通过；数据库测试与默认关闭的真实上游测试另见下文 |
| backend `go vet ./...`、`go build ./...` | 通过 |
| frontend `npm run test:music` | 60 项通过 |
| frontend `npm run type-check` | 通过 |
| `git diff --check` | 通过 |

文档历史截图中的 265 首与本次实测 267 首不同，程序以本次响应为准，不硬编码历史数量。

[补齐回归](../backend/internal/music/provider/netease/completion_test.go) 覆盖 265 个完整 ID/6 个预览、缺失项才请求、详情乱序、完整 tracks 顺序重建、重复 ID 去重、30 位大整数、缺失/非法 ID、HTTP/业务拒绝、缺失或冲突详情、整体超时、后续批次失败、复查数量/顺序/内容变化及完整空歌单。

[Service 回归](../backend/internal/music/service_test.go) 使用真实 GORM SQL 生成和内存 SQL stub 验证实际网易云 Adapter 的详情缺失、限流和复查变化都只更新失败状态，不执行快照写入，原时间/hash 和公开错误字段保持正确。该验证不等于 PostgreSQL 真实事务验收。[前端分页回归](../frontend/scripts/music-library.test.mjs) 覆盖完整 265 首、空歌单、跨页数量变化、重复、超量、缺页、非法总数和来源不符。

在 backend 目录可显式执行只读真实接口检查（PowerShell）：

```powershell
$env:NETEASE_LIVE_PLAYLIST_ID = '595975585'
go test ./internal/music/provider/netease -run '^TestLivePlaylistCompletion$' -count=1 -v
$env:NETEASE_LIVE_PLAYLIST_ID = ''
```

## 验证边界与上线状态

服务器只读 SSH 在本机受限执行返回 Permission denied；允许网络访问后的实际尝试为 TCP 22 Connection timed out，未进入远程 shell。本次未核实服务器的新接口可用性、FFmpeg 环境和独立测试数据库。

没有配置或访问 `TEST_DATABASE_URL`，PostgreSQL 集成测试 **SKIP**，不能算真实数据库验收通过。本次没有数据库快照落库、迁移、服务器文件上传、服务替换或部署。未执行新的浏览器实播、真实 FFmpeg 转码或人工听验；原有测试/历史验收不能替代这些检查。

当前交付是可审查的本地实现。上线前应恢复 SSH，核实隔离数据库和端口，在服务器执行真实同步与失败保留验证，再按 [服务器验收记录](music-server-validation-20260930.md) 复验媒体与浏览器。

## 保留产物

正式新增文件为诊断上下文、补齐回归、前端分页回归及本文；现有技术设计文档保留原文。

运行 Go 检查使用既有 `../.validation-cache/go/` 并更新构建缓存，TypeScript 检查可能更新既有 `frontend/node_modules/.tmp/tsconfig.app.tsbuildinfo` 和 `tsconfig.node.tsbuildinfo`。这些是可由用户决定保留的验证缓存；没有新增无用试验源码、下载媒体、打包文件、日志、测试 schema 或远程进程。没有删除任何文件。
