# Docker 内网部署验收（2026-10-03）

按用户选择使用 Docker 自启；用户暂无域名，因此本轮交付为 Tailscale 内网 HTTPS 运行环境，公网域名与可信 CA 证书仍待后续配置。操作、备份与回退步骤见 [部署手册](docker-deployment.md)。

## 实际部署

- 应用源码基线 `b55f43a`，加本轮新增部署模板；没有覆盖服务器旧 checkout 或修改全局 pnpm。
- 正式应用库 `blog_dev` 从 000001 升到 000006，dirty=false；原有 1 个账号保留为 admin，密码没有重置。
- 项目 `blog-production-live`：API、静态刷新、Nginx 三个容器，restart policy 为 unless-stopped；Docker/PostgreSQL 开机启动已启用。静态刷新容器每 15 秒执行既有 Worker，替代 systemd timer，不依赖 linger。
- 日志按用户明确选择采用主机默认轮转，3 个服务实际均为 json-file、max-size=10m、max-file=3；不保存过久的旧容器日志。无限保留草案没有应用，也没有重建或重启服务来变更日志。
- 入口 `https://100.96.172.0:8443`，仅监听 Tailscale 地址；8088 跳转到 HTTPS。API/媒体仅在回环 8081/8082，历史隔离 API 18081/18082 与其他应用保留。
- 自签证书包含 IP SAN，有效至 2027-10-03 21:45:12（Asia/Shanghai）。浏览器需要用户信任；没有修改客户端信任库，不能称为公网可信 HTTPS。
- 运行时：Node.js 24.21.0、pnpm 12.4.1、FFmpeg 8.1.2、yt-dlp 2026.08.19；项目 pnpm 要求仍为 >=11.19.0。

## 通过的验证

| 范围 | 实际结果 |
| --- | --- |
| 应用库备份与恢复 | 迁移前 custom dump 恢复到独立 PostgreSQL 17 容器，行数一致；恢复库 000001→000006 演练通过，账号保留 |
| Linux 回归 | PostgreSQL/race 的 SEO、storage、platform 共 80 个通过项（含子测试），零 SKIP；vet 和 3 个二进制构建通过 |
| 干净依赖安装 | 全新镜像内 pnpm 12.4.1 执行 frozen-lockfile 安装成功；没有复制旧 node_modules，仓库锁文件未改 |
| 前端 | 类型检查、36 项博客、70 项音乐、8 项 SEO 测试通过 |
| 实际 Worker | Go 直接执行 pnpm，通过内容检查和 Vite production 构建；应用库 revision/appliedRevision=1/1，lastError 为空 |
| HTTPS | 服务器和本机使用本次证书校验，首页 200；没有以忽略证书检查替代验证 |
| 页面与资源 | 首页、博客、音乐、关于、登录、后台入口 200；当前 JS 资源、sitemap、robots 可访问；未知页面/文章 404 |
| 权限与缓存 | 未登录的管理员会话/SEO 接口 401；私有页面 noindex；manifest 不公开；匿名播放 Cookie 包含 Secure/HttpOnly/SameSite=Strict |
| 容器重启 | 3 个新服务受控重启后恢复，静态状态和 HTTPS 再次通过；没有重启整台服务器 |
| 音频运行时 | 新镜像内存中 MP3 编码 16,763 字节，解码 176,400 字节非零 PCM；没有保存音频文件 |
| pnpm 12 构建回归 | 独立容器通过真实 pnpm 的发布、更新、归档和过期拒绝测试，支持 native 12 不再提供 npm_execpath 的情况 |

应用库当前没有文章或曲目，初次首页/博客和音乐为空是真实数据状态；没有将测试内容导入正式库。首次启动保存了当天空推荐，后续同日补曲不会自动重抽。登录使用已有管理员账号；本轮没有实际用户密码，因此未在正式库创建验收文章或验证管理员实际登录。

实际歌单播放、扬声器听验、手机/弱网/长时间并发，以及从正式后台发布文章后的持续运营仍需后续验证。已有隔离环境的博客发布/修订/归档闭环记录继续有效，但不能当作本轮在正式库执行这些操作。

## 构建中修正的问题

首次 Alpine CDN 连接失败，改用已探测的 HTTPS TUNA 源，保留 TLS 与 APK 签名验证。首次非 root Nginx 配置检查补齐临时目录，并改用实际 host 网络验证 IP 监听。

pnpm 12 registry 原始入口刻意没有 shebang，shell 调用通过，但 Go exec 返回 exec format error。补齐 [launcher](../deploy/pnpm-launcher.sh) 后，以 Go 探针在非 root、只读、无网络容器中验证成功，再启动 live 版本。前端构建回归也修正了只接受 pnpm JS 入口的旧假设，并用 native 12 实测通过。

失败候选容器没有用于正式入口，均保留；候选 API/Worker 已停止且 restart=no，避免开机争抢端口。其初次失败产生的静态尝试目录也保留。当前镜像为 `blog-production:release-20261003-b55f43a-b5731c-launcher`，image ID 为 `sha256:dc28f6796d8846690435b8f7b7f088039ad60605c0200edd78803cc431a61794`。

## 备份、空间与保留产物

服务器路径相对于用户主目录，本机路径相对于仓库根目录：

- 服务器 `blog-web/production/backups/release-20261003-b55f43a-b5731c/`：迁移前备份、原环境配置和恢复演练结果；均限制权限。
- 服务器 `blog-web/production/backups/post-deployment/`：升级后的 custom dump 和 SHA-256 元数据。
- 本机 `../.validation-cache/production-deployment-20261003/blog_dev-after-deployment.dump`：用户明确授权下载的服务器外副本，ACL 限当前用户和 SYSTEM；SHA-256 为 `859710e1a70aa9eafc2aabd6198cd69fc9b4db402961e486c5a622ec9f2db9ae`。私钥和环境凭据没有下载。
- 本机同目录：source.tar、deployment-files.tar、部署/恢复/验收脚本、Go exec 探针源码、公开证书和验收摘要。
- 服务器 `blog-web/production/releases/release-20261003-b55f43a-b5731c/`、incoming、go-cache：源码、构建二进制、工具归档、日志与文件系统测试产物。
- 服务器 `blog-web/production/site/`：当前发布、共享 assets，以及首次失败留下的尝试目录。
- 服务器 `blog-web/production/pnpm12-build-regression/`：独立测试 API 构建的发布/修改/归档/过期输出，测试已结束。
- 本机 `frontend/dist/seo-refresh-pnpm validation-mfnJ1X/`：修正测试后对 pnpm 11 JS 入口的构建回归，实际通过，测试 API 已结束。
- 停止的恢复、工具、Go exec、Nginx 检查、构建回归、失败候选容器，以及恢复卷、候选镜像和构建缓存均保留。
- `blog_test` 新增回归 schema 保留，名称在 release 的 database-race.log 中；原有所有历史验证产物保留。

部署后根分区约 91% 使用，剩余约 9.6 GB。本轮没有手动删除文件、容器、镜像、卷或 schema。用户已授权 Docker 正常日志轮转；除此之外，旧版本、备份和试验产物继续保留，清理需另行确认。备份脚本可按需执行，本轮未设置每日自动备份。
