# SEO pnpm／systemd／Nginx 服务器验收（2026-10-03）

本次完成此前剩余的「后台 API → pnpm production 构建 → SEO Worker → systemd timer → Nginx 静态页面」隔离闭环。源码基线为 `fd80739`，包含 pnpm 统一和后台静态状态／会话提示更新。未部署正式服务。

> 后续版本约定已调整为 `engines.pnpm >=11.19.0`，见 [包管理约定](package-manager.md)。本文中的固定 11.19.0 和隔离 launcher 描述当时实际验收环境，不再作为后续版本要求，也不代表 12.4.1 已通过相同闭环。

## 环境与边界

- 本次通过已配置的 IPv4 SSH 直接连接成功，未修改 SSH、Tailscale、路由或防火墙配置。
- 服务器：Linux、Go 1.27.1、Node.js 24.21.0、PostgreSQL 17.11；使用已存在的 `nginx:1.28-alpine` 镜像，没有拉取新镜像。
- 默认 PATH 未包含现有 pnpm 的 bin，找到的全局版本为 12.4.1。项目固定 11.19.0，使用新验证目录内的 Corepack 缓存加载该版本，再创建仅供本次进程使用的 launcher。没有切换到其他包管理器，也没有修改全局 pnpm。
- 当前前端源码使用复制的既有 node_modules，进程设置 `pnpm_config_verify_deps_before_run=false` 禁止脚本执行前自动安装。未执行依赖安装；这不能替代正式上线前的 `pnpm install --frozen-lockfile` 验证。
- 只使用原配置指定的 `blog_test`，新建独立 schema `seo_e2e_20261003_c6a1_8ecc3ec7`。凭据在服务器内加载，没有输出或传回本机。实际执行管理命令迁移至 `000006`，并创建仅用于该 schema 的测试管理员。
- API 仅监听回环 28085，Nginx 仅监听回环 48085。测试 unit、目录和容器均使用独立名称。timer 使用仓库模板，将验证间隔临时改为 2 秒；正式模板仍为 15 秒。

## 检查结果

`go vet ./...`、API/manage/seo-refresh 三个命令构建、`pnpm run type-check` 通过。`pnpm run test:seo` 8 项、`pnpm run test:blog` 36 项通过，零 SKIP。这里的 36 项包含当前后台会话回归；此前 28 项 PostgreSQL/Linux race 结果见 [上次记录](seo-server-validation-20261003.md)，本次没有重复执行该组未变更的核心数据库测试。

真实 Worker 的构建命令为 `pnpm run build-only --mode production --outDir <新目录> --configLoader runner`，内部内容检查也使用 pnpm。故障注入 wrapper 只在实际 pnpm 调用前暂停或失败一次；成功产物全部来自仓库的真实内容检查、Vite 构建及公开 Go API，没有用 fixture HTML 代替。

| 操作 | 数据库／任务结果 | Nginx 与静态结果 |
| --- | --- | --- |
| 草稿首次发布 | revision 2，timer 自动执行并确认 2/2 | `/blog/seo-loop` 返回 200，含正文和 BlogPosting；JS 资源可访问 |
| 保存已发布文章修订 | revision 保持 2 | HTML 仍为原正文 |
| 确认发布修订 | revision 3，自动确认 3/3 | HTML 更新为修订正文 |
| 构建中再次发布 | Worker 已开始 revision 4 构建时提交 revision 5；旧尝试被拒绝，随后确认 5/5 | 过期尝试期间仍为 revision 3 HTML，下一任务更新到最新正文；没有激活 revision 4 |
| 重复启动 CLI | 在一个 Worker 持锁时启动第二个，日志显示 already running 并正常退出 | 没有重复构建或切换 |
| 归档并让构建失败一次 | revision 6，appliedRevision 暂为 5，持久记录失败 | 旧静态目录保持可用，未伪造成功确认 |
| 停启 timer/Worker | pending 6/5 保留；重启后继续构建并确认 6/6 | 文章返回 404，404 HTML 含 noindex，sitemap 移除文章；旧发布目录仍保留 |

同时检查了未知路径 404、登录页 noindex、归档后旧 JS 资源继续返回 200，以及 Nginx 配置检查成功。Worker journal 显示只激活了 2、3、5、6；revision 4 被判定为过期，revision 6 的首次失败产物未激活。

最终状态：revision=`6`，appliedRevision=`6`，lastError 为空；最后确认时间为上海时间 2026-10-03 19:25:50。共保留 6 个发布尝试目录，包含过期构建和注入失败产生的空目录。

第一次验收脚本把数据库 URL 放入 PGDATABASE，psql 因此尝试默认 socket 上的 lin 角色并失败，发生在 schema 创建前。修正为显式 PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE 后重跑通过；第一次失败记录也保留。这是验证脚本参数错误，不是 pnpm 构建失败。

## 收尾与正式部署边界

- 本轮 API 已正常退出，28085/48085 均无监听；测试 timer 与 service 均为 inactive，容器为 exited。
- 测试 unit 仅 link、没有 enable；保留的链接状态为 linked，不会自动启动本轮测试。没有删除 unit 文件、链接或容器。
- 既有服务的回环 18081 ready 在验证前后均为 200；原有运行容器不变。服务器原项目工作区仍干净，提交仍为 `a08eb0d`。
- 没有迁移 `blog_dev`，没有启用正式 timer、替换正式 Web root、重启原服务或上传覆盖原项目。

现在已有实际 CLI/systemd/Nginx 的隔离验收证据；生产仍需按 [自动刷新操作说明](seo-auto-refresh.md) 配置满足 `engines.pnpm` 范围的 pnpm 与非交互 PATH、冻结锁文件安装、迁移应用库、接入真实入口，并核验原站点 assets 迁移、HTTPS、CDN 缓存及搜索／分享平台。隔离成功不表示生产已启用。

## 保留产物

未删除任何文件、schema、容器或镜像。本机路径相对于仓库根目录，服务器路径相对于服务器用户主目录。

- 本机 `../.validation-cache/seo-refresh-pnpm-server-20261003-c6a1/`：`source.tar`、`prepare-checks.py`、`run-e2e.py`、`e2e-result.json`、`worker-journal.log`、`nginx.log`、`nginx-check.log`、`unit-verify.log`。
- 服务器 `blog-web/seo-refresh-pnpm-validation-20261003-c6a1/`：源码归档和副本、复制的前端依赖、隔离 Corepack/pnpm 运行时、`runtime-bin/pnpm`、两个验证脚本、`api`／`manage`／`seo-refresh` 二进制，及版本、vet、构建、类型检查、SEO/博客测试日志。
- `attempt-1b74b967/`：首次脚本连接错误的 `schema-create.log` 和失败结果 JSON，没有创建 schema 或启动服务。
- `attempt-8ecc3ec7/`：成功结果 JSON、API/Worker/Nginx 与迁移/权限/重复任务日志、service/timer 文件、Nginx 配置、故障注入标记与 wrapper、`site/current`、6 个 `site/releases/` 目录及共享 assets。`worker.env` 只保留在服务器该目录，权限 0600，包含测试库配置，没有下载。
- 用户 `.config/systemd/user/` 中保留 `seo-pnpm-check-20261003-c6a1-8ecc3ec7.service` 与同名 timer 的链接，均已停止且未 enable。
- 已停止 Docker 容器 `seo-pnpm-check-20261003-c6a1-8ecc3ec7`；复用的 Nginx 镜像保留。
- `blog_test` 中保留 schema `seo_e2e_20261003_c6a1_8ecc3ec7`，含测试文章、修订／归档状态、测试账号和发布记录。

上述目录和 schema 属于隔离验收产物，不能作为正式部署目录。历史产物继续保留。
