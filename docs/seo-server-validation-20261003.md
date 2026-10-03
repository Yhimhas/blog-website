# SEO 服务器隔离验证（2026-10-03）

> 后续操作统一使用 pnpm，见 [包管理约定](package-manager.md)。下文验收部分的 npm 命令保留为当时执行记录。

> 本文记录早期隔离测试及当时连接阻塞。剩余 CLI/systemd/Nginx 闭环已在后续 pnpm 续验完成，见 [最新服务器验收](seo-pnpm-server-validation-20261003.md)；下文「未执行」仅代表当时范围。

本记录补充 [自动刷新操作说明](seo-auto-refresh.md) 的服务器验证。验证源码来自当前未提交工作区的源码副本，不代表已部署生产版本。

## 连接与隔离范围

用户要求再次尝试 SSH 后，IPv4 入口仍出现 TCP/banner 超时。通过同一服务器的 Tailscale IPv6 地址连接成功，沿用原来的 SSH 用户、密钥和主机密钥校验；未修改 SSH、Tailscale、路由或防火墙配置。随后连接再次间歇超时，不能把一次成功视为稳定连通，也不能据此判定服务器故障。

服务器配置确认：应用库为 `blog_dev`，独立测试库为 `blog_test`；Go 1.27.1、PostgreSQL 17.11、已有 Node.js 24.21.0。凭据由服务器原配置在进程内加载，仅打印库名和端口，没有传回本机或写入文档。

新建服务器验证目录 `blog-web/seo-refresh-validation-20261003-4e19/`，上传仅含源码与部署模板的归档；没有上传本机 `.env`、Git 元数据或凭据。后端测试只使用 `blog_test`，每个测试创建独立 schema，全部保留。前端使用该目录内的源码，复制既有前端依赖到验证副本，没有修改原项目依赖或原应用库。

## 已通过

- Linux `go test -race -count=1 -v ./internal/seo ./internal/storage ./internal/platform`：28 项测试通过，零 SKIP。包括原有后台/公开接口与权限测试。
- 迁移 `000006` 的 SQL 在隔离 schema 中实际执行；发布、发布修订、归档持久事件，失败回滚及未发布修订隔离通过。
- 版本状态行锁实际阻止公开内容提交跨越切换区间；过期激活拒绝、切换失败不确认、session advisory lock 互斥和异常后释放通过。
- Linux 真实 symlink/rename 切换通过，归档文章退出当前目录，旧目录保留；路径别名重叠拒绝、不可变 assets 保留与碰撞拒绝通过。
- Worker 重试、连续事件合并、构建中更新拒绝及目录切换后丢失数据库确认的恢复通过。这组 Worker 测试使用构建 fixture，不等于实际 CLI 调用 npm 的整套服务验收。
- `go vet ./...` 与 `cmd/api`、`cmd/manage`、`cmd/seo-refresh` 构建通过。源码副本没有 Git 元数据，使用 `-buildvcs=false`。
- Linux 前端 `npm run type-check` 和 `npm run test:seo`：8 项通过，使用已有 Node.js 24.21.0 和复制的依赖。

真实 Vite production 构建中的发布、更新、归档与过期拒绝，已在本机临时 API 验证，见 [自动刷新记录](seo-auto-refresh.md)。服务器的数据库/目录测试与本机构建测试是不同的验收范围。

## 未执行及下一步

准备了“独立 Go API → 真实 Vite → user systemd timer → 回环 Nginx”验证脚本，包含构建期间再次发布、一次构建失败、停启 timer 恢复 pending 和归档 HTTP 404 检查。脚本传输时 SSH 再次持续超时，因此本次没有启动这些测试服务、timer 或 Nginx 容器。

仍需完成实际 CLI、systemd 调度与 Nginx 的整套隔离闭环，以及正式入口启用后的状态码、旧 assets 迁移、CDN 缓存与搜索/分享平台验收。没有迁移 `blog_dev`、启用正式 timer、替换现有 Web root 或重启当前服务。

## 保留产物

未删除文件、schema、容器或镜像。以下本机路径相对于仓库根目录，服务器路径相对于服务器用户主目录。

- 本机 `../.validation-cache/seo-refresh-server-20261003-4e19/`：`backend-source.tar`、`frontend-source.tar`、`run-backend-checks.py`、`run-e2e.py`。后者为准备好的未执行验收脚本。
- 服务器 `blog-web/seo-refresh-validation-20261003-4e19/`：两个源码归档、backend/deploy/frontend 源码副本、复制的 frontend/node_modules、`run-backend-checks.py`、`database-race.log`、`vet.log`、`build-api.log`、`build-manage.log`、`build-worker.log`、`api`、`manage`、`seo-refresh`，以及 `filesystem-tests/` 下的 6 个保留测试目录。没有用这些二进制替换原服务。
- `blog_test` 中保留 SEO schema：`seo_test_e60e98d305ace9c5693813dc`、`seo_test_43fc6a787acf3e2175b2942d`、`seo_test_c28e8ef9f6bfb445c667e544`、`seo_test_f34057fea8ba277066c5a980`。
- `blog_test` 中保留既有功能回归 schema：`backend_test_44b5c71f109ed319`、`backend_test_6756f88c0ccf9311`、`backend_test_5dc754d9d1dfb28c`、`backend_test_394b1561e3f73ad5`、`backend_test_adeb6b5f7e8c9c71`。

测试关闭了各自数据库连接；本次没有新启动需要收尾的常驻 API、timer、Nginx 或后台测试进程。
