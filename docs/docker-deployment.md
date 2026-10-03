# Docker 部署与回退手册

更新日期：2026-10-03。按用户选择使用 Docker 自启，静态刷新由独立容器每 15 秒调用既有 SEO Worker，替代 user systemd timer，不依赖 linger。实际部署结果见 [部署验收](docker-deployment-validation-20261003.md)。

## 当前入口和目录

当前没有域名，入口为 `https://100.96.172.0:8443`，仅绑定服务器的 Tailscale 地址。`http://100.96.172.0:8088` 跳转到 HTTPS。访问设备需能连接该 Tailscale 网络。

使用 IP SAN 自签证书，浏览器尚未信任时会显示证书警告。本机公开证书为 `../.validation-cache/production-deployment-20261003/server-certificate.crt`，私钥只在服务器。证书信任由用户自行处理，不修改客户端信任库。取得域名后需更换入口、可信证书以及 API/SEO 的 origin，再重建页面；当前不具备公网访问或公开搜索收录条件。

服务器目录以下均相对于登录用户主目录：

- `blog-web/production/releases/release-20261003-b55f43a-b5731c/`：源码、二进制、部署配置与日志。
- `blog-web/production/private/release-20261003-b55f43a-b5731c/`：API、Worker、Compose 的私密环境文件，权限 0600。
- `blog-web/production/private/tls/`：证书与私钥，私钥权限 0600。
- `blog-web/production/site/`：Worker 与 Nginx 共享的 releases、current 和 assets；旧目录保留，不合并 HTML。
- `blog-web/production/backups/`：迁移前后数据库备份、恢复演练结果和元数据。
- `blog-web/production/deployment-state.json`：版本、镜像和配置位置，不包含数据库密码。

运行项目名为 `blog-production-live`。API 和媒体仅监听回环 8081/8082，Nginx 使用 Tailscale 8088/8443，未占用已有 8080、18081/18082 或其他应用端口。PostgreSQL 继续使用已有服务器实例。

## 常用运维命令

从服务器用户主目录进入生产目录后执行，环境文件只传路径，不将数据库 URL 放入命令参数：

```sh
cd blog-web/production
docker compose --env-file private/release-20261003-b55f43a-b5731c/compose.env \
  -f releases/release-20261003-b55f43a-b5731c/deploy/compose.yaml -p blog-production-live ps

docker compose --env-file private/release-20261003-b55f43a-b5731c/compose.env \
  -f releases/release-20261003-b55f43a-b5731c/deploy/compose.yaml -p blog-production-live logs --tail 50 api seo-refresh nginx

curl --cacert private/tls/site.crt https://100.96.172.0:8443/api/v1/ready
```

三个容器采用 `restart: unless-stopped`，Docker 和 PostgreSQL 已设置开机启动。已实测容器重启恢复，没有为验收重启整台服务器。手动 stop 后容器不会自行启动；需要时用 `up -d --no-recreate --wait` 恢复。

后台文章发布、修订发布、归档通过数据库持久版本触发 Worker。停止后 pending 保留；恢复后继续处理。进入后台核对静态状态，不将 API 成功直接当作 HTML 已更新。故障检查 `api` 健康、Worker 日志、目录权限和数据库迁移，不能直接改 appliedRevision。

按用户确认，本站 Docker 日志采用主机默认轮转：json-file，每个服务最多 3 个日志文件、每个 10 MB；达到上限时最旧的容器日志按轮转策略处理，不无限保留。Compose 不覆写轮转参数，实际配置可通过 docker inspect 查看。此授权仅适用于正常日志轮转；旧发布目录、备份和其他文件继续保留。当前剩余磁盘约 9.6 GB，应定期检查占用；Docker prune、Compose down、卷删除和其他旧文件删除不属于默认运维步骤。

## 准备后续版本

使用新的 release 目录，不覆盖当前源码、私密配置或旧发布产物。以下命令在新的源码根目录执行：

```sh
mkdir -p runtime/bin
cd backend
CGO_ENABLED=0 go build -buildvcs=false -trimpath -o ../runtime/bin/api ./cmd/api
CGO_ENABLED=0 go build -buildvcs=false -trimpath -o ../runtime/bin/manage ./cmd/manage
CGO_ENABLED=0 go build -buildvcs=false -trimpath -o ../runtime/bin/seo-refresh ./cmd/seo-refresh
cd ..
node deploy/prepare-tooling.mjs 12.4.1 ../../../music-playback-tools/yt-dlp
```

使用实际可访问的 yt-dlp 相对路径。这里的 12.4.1 是本次已验收运行时，项目要求仍是 `>=11.19.0`；换版本需重新验证。pnpm archive 校验 registry SHA-512；工具文件已存在时拒绝覆盖。

[Dockerfile](../deploy/Dockerfile.runtime) 使用新的容器文件系统执行 `pnpm install --frozen-lockfile`、类型检查、博客/音乐/SEO 测试。使用可配置 HTTPS Alpine 源，保留 TLS 与 APK 签名验证。pnpm launcher 提供 Go 可以直接执行的 shebang，避免 pnpm 12 registry 原始入口的 exec format error。应用凭据、日志和本机 node_modules 不进入构建上下文。

新的 Compose 私密配置需给出 BLOG_IMAGE、BLOG_API_ENV、BLOG_SEO_ENV、BLOG_SITE_DIR、BLOG_TLS_DIR 和 BLOG_NGINX_CONF，值对应新的版本及既有共享站点。私密文件只存在服务器，权限 0600。先构建并验证新镜像、备份数据库及演练迁移，再执行实际升级。

为了保留旧容器，新版本切换使用新的 Compose 项目名；先停止旧项目的 API/Worker/Nginx，并将退役容器 restart policy 改为 no，再启动新项目，避免争抢同一端口或开机同时运行。仅操作确认归属本站的容器，不停止 AstrBot、natfrp 或历史隔离 API。

## 备份和恢复

[备份脚本](../deploy/backup-database.py) 在服务器执行，凭据通过私密环境文件读入进程，数据库密码不出现在命令参数中：

```sh
cd blog-web/production
python3 releases/release-20261003-b55f43a-b5731c/deploy/backup-database.py \
  --env-file private/release-20261003-b55f43a-b5731c/api.env \
  --output backups/manual
```

每次备份创建新的文件和 SHA-256 元数据，文件权限 0600。失败的 partial 文件保留，不覆盖或删除旧备份。建议每次升级前后以及内容维护后执行；本次没有配置每日自动备份任务。

迁移前备份已恢复到独立 PostgreSQL 17 容器，并在恢复库实测 000001→000006。恢复演练没有连接应用库；原账号保留为 admin。部署后另有一份数据库备份，经用户明确授权复制到本机，哈希核对后限制 ACL。备份包含密码哈希和站点数据，应按私密数据保存。

实际恢复时先停止本站写入和 Worker，再将选定备份恢复到**新的数据库或独立 PostgreSQL 容器**，核对数据/迁移及账号。不要直接对当前库使用 clean、DROP 或重置命令。验证后通过新的私密环境文件把 API 和 Worker 一起指向恢复库；SEO sourceId 与恢复库匹配后重新生成当前页面。原库和旧静态目录继续保留。

## 回退流程

1. 保存本次已验证镜像引用、环境文件、源码和备份。本次修正后镜像为 `blog-production:release-20261003-b55f43a-b5731c-launcher`，作为后续回退基线；初次失败候选镜像不能用于回退。
2. 应用升级失败时，停止新项目，保留新容器及日志；核对旧应用是否兼容当前数据库后，启动已验证旧镜像与配置。不要把镜像回退等同于数据库回退。
3. 前端回退使用相应源码/运行时对当前公开数据重新构建，可在对应 Worker 容器执行现有 CLI 的 `--force`，再检查静态状态和 404。不能直接切回旧 HTML 目录，避免恢复已归档内容。
4. 数据库不兼容或数据损坏时，按上一节恢复到新库并验证后再切换，原库保留。迁移仅向前；不强行改 dirty/version。

这是首次 Docker 正式运行，没有此前已验证的旧正式 Docker 版本。若撤回本次上线，停止 live 项目并保留数据即可；历史开发/隔离服务仍在，但不能当作已验收的生产回退版本。
