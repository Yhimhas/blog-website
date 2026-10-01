# 网易云歌单补齐与播放服务器复验

日期：2026-10-02（Asia/Shanghai）。本记录承接 [本地实现](netease-playlist-completion-implementation-20261002.md)。结论：重试直接 SSH 成功，当前代码已在隔离数据库中真实同步 267 首；数据库集成和公开样本转码/浏览器交互通过。没有部署或替换生产服务。

## SSH 与环境

用户指出 Remote SSH 可用后，使用 Windows OpenSSH 重试 `ssh -o BatchMode=yes -o ConnectTimeout=15 100.96.172.0` 成功，登录用户 lin、服务器 Linux。未修改 SSH、Tailscale、防火墙或凭据。此前超时仅代表当时连接失败，根因没有查明。

服务器默认 PATH 未包含 Go 和 FFmpeg，但已有 Go 安装及 `blog-web/music-playback-tools/ffmpeg-runtime/` 独立运行时。实测 FFmpeg 7.1.5、libmp3lame 可用；运行时需要既有动态库路径。

旧隔离服务 PID 13637，端口 18081/18082，连接本机 `blog_test.music_playback_validation_20260929`，其可执行文件及工作目录均已核实。本轮不停止或修改该服务。未检查或改动生产入口。

服务器只读官方接口返回：

| 接口 | 本次结果 |
| --- | --- |
| 旧 playlist detail，目标 `595975585` | HTTP 200 / code=20001，没有 ID 清单 |
| 新 v6 playlist detail，同一目标 | HTTP 200 / code=200，trackCount=267、trackIds=267、tracks=6 |
| v3 song detail，公开样本 `33894312` | HTTP 200 / code=200、songs=1 |

以上与本机接口验证一致，不将旧接口的业务拒绝解释为整台服务器被封禁。

## 隔离方式

创建新的服务器目录 `blog-web/music-validation-20261002-sync-7e43/`（相对于服务器用户主目录），仅上传本项目 backend/cmd、backend/internal、backend/migrations、go.mod、go.sum。源码包不含 .env、密钥、Git 元数据或凭据。

先从已核实的旧隔离进程在内存读取测试数据库连接，并通过 SQL `SELECT current_database()` 确认为 `blog_test`，之后才允许创建 schema。凭据没有打印或保存。测试不连接生产数据库。

运行服务器 Go 全套测试时实际执行 PostgreSQL 集成，保留两份新 schema：

- `backend_test_62e0bc9f7768db8a`：账号角色集成测试。
- `backend_test_9b14bab395f6541d`：事务、快照、同步失败保留及播放检查状态集成测试。

另外创建 `music_validation_20261002_sync_7e43`，执行既有迁移并启动本轮隔离 API。随机测试管理员密码仅在内存用于登录，不输出或写文件。API PID 52572，HTTP 仅监听 127.0.0.1:28081，媒体仅监听 127.0.0.1:28082。`MUSIC_AUTO_SYNC=false`，`MUSIC_FORCE_TRANSCODE=true`。

实际音乐页通过本机 Vite 5179、回环 SSH 转发 38081/38082 访问上述 API，未开放公网端口。

## 同步与数据库结果

| 检查 | 实际证据 |
| --- | --- |
| 服务器 `go test -buildvcs=false ./... -count=1 -v` | 全部通过；TestAccountRoles 和 TestPostgresContracts 实际 PASS，未 SKIP |
| 服务器 go vet / 构建 api、manage | 通过，构建使用 `-buildvcs=false` |
| 服务器真实 Adapter 只读测试 | 267 首，约 1.09 秒；2 次歌单详情、6 批单曲详情，结束复查一致 |
| 管理员真实同步 API | 返回 202，runId `c4f2daf0e410281a235228503b0bc6efd59f5b80cafbd5109d88741faf360c3f` |
| 同步任务终态 | ready，candidateCount=267 |
| 公开 Source DTO | ready、trackCount=267、syncErrorCode=null、syncedAt 非空 |
| 曲目读回 | 6 页、267 个不重复 ID，完整读回约 1.23 秒（包含触发同步与轮询） |
| 顺序 SHA-256 | `20dc270b8c1c57ca69f2c09db45be4e902723ba99ab8917e5ad3aefd6b8e5488`，与本机及服务器只读 Adapter 一致 |

本次同步成功是在隔离 schema 的新目标来源上实际完成，未借用人工导入记录。数据库集成验证中的失败保留使用可控拒绝响应；没有人为让网易云真实目标接口限流或中断。新分批路径的详情缺失/限流/复查变化保留行为另由本地真实 Adapter + SQL stub 回归覆盖，详见实现记录。

真实同步完成后，另行导入 `netease:1`，标题为“验收样本（人工导入，非目标歌单同步）”，包含公开音源 `33894312` 与无效 ID `1`。因此浏览器音乐库共显示 269 首，其中目标歌单同步 267 首、人工验收 2 首，两者分别保存。

## 实际媒体与浏览器

公开样本仅通过现有官方 outer 音源链路播放，不携带网易云账号 Cookie。媒体测试保存统计结果，音频仅在内存有界读取，不保存 MP3 或 PCM 文件。

| 检查 | 实际证据 |
| --- | --- |
| Go 播放会话 | 公开样本 ready，audio/mpeg |
| 强制转码 | 观察到 API 的 FFmpeg 子进程 PID 52646，输入 pipe:0、输出 pipe:1、编码器 libmp3lame |
| 媒体有效性 | HTTP 200，读取 65,536 字节；内存解码退出 0，1 秒 PCM 为 176,400 字节且非全零 |
| 释放 | 停止 HTTP 204；采样的 FFmpeg 子进程已释放 |
| 浏览器实播 | 实际 Vue 音乐页点击公开样本，显示“正在播放”，进度从 0:04 推进 |
| 鼠标暂停/恢复 | 点击暂停后 0:32 保持不变，显示“已暂停”；点击播放恢复，推进至 0:39 |
| 失败切歌 | 点击下一首切至无效样本，进度归零，显示“来源暂不可用，请重试或前往原站。”；有“重新播放”及原站入口，不自动跳过 |
| 切回有效曲目 | 点击上一首后重新播放，进度推进至 0:28 |
| 停止及导航 | 点击停止后 0:00、“已停止”；随后通过页面链接进入 /blog |
| 旧会话停止 | 本轮 API 日志记录 4 次 stop 请求返回 204；服务停止前没有 FFmpeg 子进程 |

首轮媒体探测仅读取 Go 主线程的 children，未捕获 FFmpeg；后续改为读取该 API 所有 task 的 children 后实际捕获成功。首轮没有保存媒体产物，探测失败不作为转码通过依据。

本轮暂停、恢复和切歌使用实际页面鼠标 click，补上此前仅键盘验证的限制。物理扬声器出声仍未人工确认；没有验证 267 首均可播，也未执行移动端、超长播放、高并发或生产入口验收。离开路由前已经点击停止，本轮不将该导航单独当作“播放中离开路由自动释放”的证据；该行为的回归测试仍保留。

## 收尾与保留产物

已核对本轮 API 可执行路径后发送 SIGTERM，28081/28082 均不再监听，无 FFmpeg 子进程。原隔离 PID 13637 仍存在，18081 的 ready 为 HTTP 200。本轮 SSH 转发与 Vite 已停止，本机 38081/38082/5179 均已释放。

没有删除任何文件或 schema。以下验证产物保留，便于审查，也可由用户决定后续是否清理：

- 本机 `../.validation-cache/netease-server-validation-20261002-source.tar`：本轮非敏感源码传输包。
- 本机 `../.validation-cache/netease-browser-playback-20261002.jpg`、`netease-browser-failure-20261002.jpg`：实播及失败提示截图。
- 服务器 `blog-web/music-validation-20261002-sync-7e43/`：backend 源码副本、source.tar、api、manage、api.pid、api.log、tests.log、vet.log、build-api.log、build-manage.log、live-tests.log、sample.json、sync-result.json、pipeline-result.json、shutdown-result.json 和 go-cache。api.pid 仅为已停止进程的历史记录。
- `blog_test` 内上述 3 个新测试 schema：角色/事务测试数据、267 首目标快照、2 个独立人工样本和测试管理员。没有移除历史测试 schema。

本轮只新增验收文档和验证产物，没有进一步修改业务代码、部署配置或生产服务。
