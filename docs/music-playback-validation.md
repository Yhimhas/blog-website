# music 播放实施与验收记录

日期：2026-09-28。状态：核心代码已实现，真实播放验收受服务器暂时断网阻塞；**尚未达到原计划的上线完成条件**。本文承接 [实施计划](music-playback-implementation-plan.md)。原计划保留。

## 已实现

- Bilibili 收藏夹 SourceURL 与视频 Track URL 分开校验；P1 canonical ID 与快照整体校验；`manage import-bilibili` 可重复运行，默认拒绝空列表，发现旧非 P1 数据先要求映射。
- 向前迁移 000003 保存播放检查时间与错误分类；metadata 同步的 unknown 不覆盖已验证结果，不存临时媒体 URL。
- 匿名 HttpOnly / SameSite=Strict Cookie 绑定会话；写请求 Origin、body、访客及 IP 限额；2 个解析任务、4 个媒体流、128 条有界会话；同访客仅一条活跃会话；ready 60 秒过期，终态 5 分钟回收。
- yt-dlp 固定参数 bestaudio，单独选择音轨；JSON/stderr 有界；Windows taskkill / Linux process group 取消子进程。当前支持实测格式 M4A/AAC；其它格式返回不支持，尚未提供 FFmpeg 转码路径。
- HTTPS 媒体 URL、全部 DNS 结果、实际拨号地址与重定向检查。默认 443，Bilibili 官方 bilivideo CDN 额外允许 8082、4483；拒绝私网、回环、保留地址及凭据 URL，不继承环境 HTTP proxy。
- 标准 net/http 媒体 handler；独立回环 8082，首字节/读取/写入/总时长限制；64 KiB 应用缓冲；停止取消连接；不保存媒体，不伪造 206 或长度；Range 忽略并完整返回 200。
- 页面真实播放、暂停、停止、手动上下首、结束顺序播放、音量、只读进度；明确 autoplay 拒绝和错误提示；页面内部 tab 切换继续，离开路由停止；队列快照与迟到响应隔离。
- 音乐库读取完整 API 分页，无本地列表回退；旧收藏保存为 v2，新旧键并存，未映射项保留为暂不可用。静态快照仅用于旧收藏迁移和 CLI 导入。
- OpenAPI、Vite 媒体代理、默认关闭的环境配置、Nginx 媒体 location 样例已补充。

## 已执行验证

- Windows：`go test ./...`、`go vet ./...`、`go build ./...` 通过。使用工作区 Go 缓存解决默认系统缓存写入权限问题。
- 前端：20 项 `npm run test:music` 通过，type-check、内容检查、生产 build 通过。覆盖迟到创建、autoplay 拒绝、错误不自动跳歌、全量分页、旧收藏、暂停后过期、路由销毁。
- 会话/HTTP 测试覆盖 Cookie 隔离、Origin、未知 JSON 字段、超大 body、并发解析上限、4 路流与第 5 路拒绝、重复媒体连接、幂等停止、过期、服务关闭、私网和官方媒体端口策略。
- 可控上游测试覆盖 403/429/HTML 拒绝、响应头前失败、有效媒体头、停止中断阻塞上游；未覆盖全部慢写、重定向和 DNS rebinding 情景。
- 服务器 Debian 13 / PostgreSQL 17：源码副本上的真实 DB 集成通过（数据库测试未跳过），unknown 不抹掉 verified availability；隔离 schema 导入 53 首 Bilibili 曲目成功。
- 服务器 yt-dlp 2026.08.19：首曲 BV1a4MS67Eey P1，79.296 秒，format 30280 / m4a / mp4a.40.2 / vcodec=none。上游返回 200 application/octet-stream，前 64 KiB 读取约 0.302 秒。此结果不是浏览器已发声的证明。
- 浏览器能显示 API 的 53 首曲目。第一次播放暴露 CDN 非 443 端口拒绝，已修复；修复后服务器断网，尚未重新完成实际播放。
- 本机同版本 Go resolver 已返回 ready / audio/mp4；本机 DNS 是 Fake-IP（198.18.x.x），安全媒体拨号按设计拒绝。没有放宽保留地址检查来绕过该环境问题。

断网前 Linux/DB 验证的是当时版本；后续 CDN 端口、额外边界测试及 UI 收藏细节修正仅完成 Windows 回归，恢复网络后应在 Linux 再跑一次。

## 恢复网络后的必要验收

1. 将当前源码更新到服务器隔离验证副本，重新 build；终止旧测试 PID 后启动新版本；运行 opt-in `MUSIC_TEST_YTDLP` 测试。
2. 目标服务 → 同域代理 → 浏览器，完成一首真实播放、暂停/恢复、停止、自动下一首、快速切歌 20 次（受每分钟 10 次创建限额约束，验证限流提示）。
3. 选择现有 1 小时曲目，连续播放超过 10 分钟；补失效样本、慢客户端、断网、并发、CPU/RSS 和 Worker 释放检查。
4. 完成 Chrome/Edge 和手机 Safari/Chrome 真机验证，以及博客、登录回归。
5. 查明实际公网入口与反向代理，再部署默认关闭版本；媒体 location 验证完成后启用。当前没有替换已有 API，没有更改应用数据库，没有开放公网测试端口。

网易云 P5 的音源可行性和 resolver 尚未实施。现有网易云元数据同步保留，页面只提供原站播放入口，不承诺网易云站内可播。

## 配置与操作

后端读取进程环境，不自动加载 .env。先按现有部署方式加载凭据，执行迁移、导入，再启动：

```sh
# 在 backend 下执行；DATABASE_URL 等沿用部署环境。
go run ./cmd/manage migrate
go run ./cmd/manage import-bilibili ../frontend/src/data/bilibili-playlist.json
MUSIC_PLAYBACK_ENABLED=true MUSIC_YTDLP=yt-dlp go run ./cmd/api
```

`MUSIC_MEDIA_ADDR` 默认 127.0.0.1:8082，必须回环 IP。`ALLOWED_ORIGIN` 必须与浏览器站点完全一致。生产需要 HTTPS 和 Secure Cookie。

[媒体 Nginx 配置](../deploy/music-streams.nginx.conf) 放入现有 server block；API 仍走 8081。当前 IP 限流直接使用 RemoteAddr，不信任 X-Forwarded-For，代理后的 IP 配额会合并。这是保守默认，需结合实际可信代理拓扑再调优。

关闭 `MUSIC_PLAYBACK_ENABLED` 并重启即可回退原站入口，保留元数据、收藏及所有文件。未实现的格式保持明确不支持，不启用无约束 FFmpeg 远程读取。

## 保留产物与进程

没有删除任何文件。可清理的验证产物列在下方，由用户决定是否保留。

- 工作区父目录：`.tools/music-playback/yt-dlp`、`yt-dlp.exe`、`backend-validation.tar`。前两项可继续作为开发工具；tar 是旧验证源码包，后续需重打包。
- 项目：`frontend/dist/` 本次构建及旧哈希资源；`frontend/node_modules/` 的构建缓存；工作区 `.tools/go-cache/` 的编译缓存。
- 服务器 `blog-web/`：`music-playback-tools/`（工具与传输包）、`music-playback-validation/`（源码、两个二进制、tests.log、api.log、api.pid）。没有媒体缓存。
- 独立测试数据库：schema `backend_test_eda035a055765824`、`backend_test_290ccfc5f57e5cef`、`music_playback_validation_20260928`。
- 断网前隔离 API PID 220232，回环监听 18081/18082。恢复网络后先核实 PID 对应的命令，再停止或替换；当前无法确认远程进程是否仍在运行。
- SSH 转发因断网退出。本地 Vite 5173 是本次预览服务；后端断线时显示明确加载失败，不会回退到静态曲库。
