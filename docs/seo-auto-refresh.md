# SEO 自动刷新操作说明

> 后续操作统一使用 pnpm，见 [包管理约定](package-manager.md)。下文验收部分的 npm 命令保留为当时执行记录。

更新日期：2026-10-03。静态页面和元信息说明见 [SEO 与链接分享](seo-and-sharing.md)。本文描述当前实现与安装步骤；提供模板不表示已经启用生产服务。

## 触发与一致性

1. 迁移 `000006_seo_publication` 新建单行状态与 posts trigger，初始 revision=1、appliedRevision=0，已有文章也会触发首次构建。发布、发布修订、归档或公开 SEO 字段变化，在文章事务内增加 revision；失败时一起回滚。草稿、未发布修订、失败/冲突请求不触发。字符串版本避免 JS 整数精度损失。
2. 独立 `seo-refresh` 每次只尝试一个构建。systemd timer 启动时检查，任务结束约 15 秒后再次检查；连续变化合并为最新版本。PostgreSQL session advisory lock 阻止多个实例并行发布；丢失连接不能继续切换，因为最终事务仍使用持锁连接。失败与 pending 持久保存，重启后补偿。
3. 每次预留新的 `releases/revision-<revision>-<随机编号>/`。插件在读取前、读取后和文件写完后复核版本。Worker 再检查 manifest、文件清单、站点 origin、preview 标志及数据库 sourceId，避免混合快照、错误数据库或预览产物上线。
4. 完整构建的 hashed JS/CSS 复制到独立不可变 assets 库。同名不同内容拒绝覆盖，旧资源保持可访问。新目录不合并旧 HTML，归档文章不会从旧目录残留进入当前 Web root。
5. 最终短事务锁住版本状态行，复核 revision 后，通过同目录临时 symlink + Linux `rename` 原子替换 current，再确认 appliedRevision。文章 trigger 更新同一行，新公开提交只能在切换完成后继续，不会在整个构建期间锁住编辑。后续版本保持 pending，下一任务继续处理。
6. 构建/复核/资源复制失败时保留旧 current；切换失败不确认。rename 成功、数据库确认前中断时，下次任务从 current manifest 恢复确认。所有旧目录与失败产物保留，不删除普通文件；current 若为普通文件/目录则拒绝接管。

构建期间可以继续编辑和发布；频繁公开更新可能连续放弃构建，停止变动后收敛。API 成功与静态切换之间有构建耗时及 timer 延迟。归档涉及敏感内容时，须核验静态地址及 CDN 缓存。旧 releases 目录只用于私有运维，不应通过 Nginx 暴露整棵目录。

锁与切换依据：[PostgreSQL 显式锁](https://www.postgresql.org/docs/current/explicit-locking.html)、[Linux rename](https://man7.org/linux/man-pages/man2/rename.2.html)、[Nginx open_file_cache](https://nginx.org/en/docs/http/ngx_http_core_module.html#open_file_cache)。

## 安装（Linux）

示例约定仓库位于用户主目录下 `blog-web/blog-website/`，二进制位于 `blog-web/bin/`，静态目录位于 `blog-web/seo-site/`。可修改 service 中的 `%h` 路径及环境配置，适配已有部署。

1. 按既有部署约定备份数据库。用新源码执行迁移，再升级 API。迁移后的旧 API 仍可操作文章，数据库 trigger 同样记录变化；新 SEO 插件需要新版 `/api/v1/seo/revision`。从用户主目录执行：

   ```sh
   cd blog-web/blog-website/backend
   go run ./cmd/manage migrate
   mkdir -p ../../bin
   go build -o ../../bin/seo-refresh ./cmd/seo-refresh
   ```

2. 在 `frontend/` 使用 `pnpm install --frozen-lockfile` 按 lockfile 安装完整依赖，包括构建使用的 devDependencies，运行 `pnpm run type-check`、`pnpm run test:seo`。人工操作及 Worker 均使用 pnpm，其非交互 PATH 须包含 package.json 固定版本的 pnpm 和受支持的 Node.js（见 [包管理约定](package-manager.md)）；Go API 的工具 PATH 不代表 Worker 的 PATH。
3. 将 [环境示例](../deploy/seo-refresh.env.example) 复制到仓库外的 `blog-web/seo-refresh.env`，权限 0600，替换数据库凭据与正式域名。API 和 Worker 使用同一 DATABASE_URL/search_path。binary 读取进程环境，不自动读 `.env`；systemd 通过 EnvironmentFile 加载。
4. releases、current、assets 设置为三个互不包含的路径，Worker 用户可写、Nginx 用户可读。默认相对路径从 service 的 WorkingDirectory 解析。首次使用一个尚不存在的 current 路径或合法 symlink；不要把现有普通站点目录作为 current，Worker 不会移动或删除它。
5. 将 [service](../deploy/seo-refresh.service) 与 [timer](../deploy/seo-refresh.timer) 复制到用户 `.config/systemd/user/`，调整路径。需要退出登录后继续运行时，由管理员配置该用户的 linger。执行：

   ```sh
   systemctl --user daemon-reload
   systemctl --user start seo-refresh.service
   systemctl --user enable --now seo-refresh.timer
   systemctl --user list-timers seo-refresh.timer
   journalctl --user -u seo-refresh.service -n 100 --no-pager
   ```

6. 首次构建成功后，再按 [Nginx 片段](../deploy/seo.nginx.conf) 修改入口并执行现有配置检查/reload：server root 指向 current；`/assets/` 的 root 指向共享 assets 的父目录，按实际 Nginx prefix 修改片段中的 `seo-site`。切入口前，把原站点的 hashed assets 复制到共享库并校验同名内容，避免已有浏览器请求旧 chunk 时 404。HTML/404 必须关闭 `open_file_cache`，代理/CDN 不能缓存文章 API 和版本接口；Worker 建议直连回环 API，响应已含 no-store。后续内容刷新只换 symlink，无须逐次 reload。

同一 static root 由一个数据库和 Worker 管理，其他部署脚本不得合并旧 HTML、直接写入当前目录或另行切换指针。源码升级和内容构建应在部署流程中串行安排。

## 状态、故障与回退

- 无凭据 `GET /api/v1/seo/revision` 仅返回 sourceId/字符串 revision，不返回任务错误或路径。内存 Demo 不提供此接口，不能用于正式 SEO 构建。
- 管理员 `GET /api/v1/admin/seo` 返回 revision、appliedRevision、lastAttemptAt、publishedAt、lastError。目标版本大于确认版本表示待刷新；版本相等且无错误表示已确认。普通用户无权查看状态。
- 后台文章管理直接展示上述状态，每 5 秒检查一次，可点击「刷新静态状态」手动复查。发布/归档成功后立即清除旧确认并显示待更新，再读取最新状态；旧请求不能覆盖新的待更新提示。未执行过任务时提醒确认自动刷新服务启用，失败时提示检查发布任务；接口失败或旧后台缺少此接口时显示「无法确认」，不将数据库成功误报为静态访问已生效。退出登录或离开管理页会停止检查并取消请求。
- 在服务运行目录、加载同一进程环境后执行 `seo-refresh --status` 可只读查看数据库状态。故障先检查 service 日志、API `/ready`、Node/pnpm PATH、域名、权限及 manifest；不修改 appliedRevision 来伪造成功。失败目录保留，下次使用新目录。
- systemd 的 control-group 在任务中断时结束该任务的 pnpm/Vite 子进程，不结束其他应用。CLI 也在超时/退出时取消构建进程组。任务默认最长 5 分钟，SEO_BUILD_TIMEOUT 可设为 1 秒至 30 分钟。
- 仅前端源码变化不会增加文章版本。完成源码、依赖及检查后，在相同运行目录和配置下执行 `seo-refresh --force`；它仍遵守互斥和公开版本保护。强制发布失败须修复后重新执行 `--force`；普通 timer 只负责公开内容版本的收敛。
- 回退前端时保留历史目录，回退源码后执行 `--force`，重新生成当前公开版本。直接设旧目录为 root 可能恢复已归档文章，不能作为内容安全回退。所有失败构建、旧目录与 staging 文件均保留，由站长审查后处理。

## 验证与保留产物（2026-10-03）

- `npm run test:seo` 8 项、`npm run test:blog` 7 项、类型检查通过。新增用例覆盖同总数更新、来源变化、空公开集合、字符串大整数版本和文件写完后的复核。
- `npm run test:seo:build` 通过真实 Vite production 构建验证发布、更新、归档与预期失败的过期构建；核验正文、manifest 文件清单、归档后文章 HTML 缺席、sitemap 移除及旧目录保留。使用本机测试 API，测试后已停止。
- 后端 `go test ./...`、`go vet ./...` 通过；文件系统用例覆盖不完整/预览/越界产物、普通目录保护、旧资源保留与不可变资源碰撞拒绝。Linux Worker 和 Linux 测试二进制交叉编译通过，仅编译未执行。沙箱限制回环网络与 pnpm Junction 解析，完整测试和类型检查在获准的沙箱外运行通过。
- 本机未配置 TEST_DATABASE_URL 时 PostgreSQL 用例 SKIP，Windows 同样跳过 Linux 切换；后续服务器隔离验证实际运行 PostgreSQL/Linux race 测试，28 项通过、零 SKIP，包含事务、并发锁、互斥、失败重试、事件合并、切换和中断恢复。服务器 vet、三个后端命令构建、前端类型检查及 8 项 SEO 测试也通过，详见 [服务器记录](seo-server-validation-20261003.md)。
- 初次 SSH 超时后，经同一服务器的 IPv6 连接成功完成上述验证；后续连接又持续超时。未改动正式部署，实际 CLI/systemd/Nginx 整套闭环仍待验收，不把 fixture 构建的 Worker 测试记作真实 Vite 服务闭环。

在 Linux 的独立测试库设置 `TEST_DATABASE_URL`、`ALLOW_TEST_SCHEMA_CREATE=true`，执行 `go test -race ./internal/seo ./internal/storage ./internal/platform`。每次创建并保留独立 schema，不 DROP。可设置 `SEO_TEST_OUTPUT` 指定保留文件目录；真实构建测试执行 `pnpm run test:seo:build`，所有输出保留。

生产验收应完成发布 → 保存修订（静态保持旧版）→ 发布修订 → 归档，逐步等待 revision=appliedRevision，检查首屏 HTML、sitemap 和归档地址 404；构建中再次发布确认过期产物不切换，停止/重启 Worker 确认 pending 保留。验收还须覆盖原站点 assets 迁移、未知路由 HTTP 404 和 CDN 缓存。

未删除文件。本次保留：

- `frontend/dist/seo-refresh-validation-3nkecv/` 与 `frontend/dist/seo-refresh-validation-BWMtTw/`：每次各保留 published、updated、archived、stale 四个测试目录，均非正式发布。
- `../.validation-cache/seo-refresh-20261003/`（相对于仓库根目录）：文件系统验证目录为 `seo-test-1002892224`、`seo-test-1887407093`、`seo-test-2006606109`、`seo-test-3069254031`、`seo-test-3144014026`、`seo-test-481171159`、`seo-test-607096543`、`seo-test-636322464`、`seo-test-677948362`；另保留 `seo-refresh-linux` 与 `seo-linux.test` 交叉编译产物，未部署。
- `frontend/node_modules/.tmp/`：既有类型检查增量缓存已更新。
- 本次 SSH 续验的源码归档、服务器副本、二进制、日志和 9 个测试 schema，完整清单见 [服务器保留产物](seo-server-validation-20261003.md)。

历史验证产物继续保留，见 [旧 SEO 记录](seo-and-sharing.md)。

### 后台状态提示补充（2026-10-03）

`npm run test:blog` 扩展至 26 项并通过，类型检查通过。新增测试覆盖状态接口的无缓存/凭据/字符串版本约定、无效响应拒绝、状态轮询、过期响应、退出/离开取消，以及真实管理页和状态组件在发布/归档、待更新与状态不可用时的呈现。状态读取失败不会覆盖数据库操作成功提示。

本次仅修改前端与说明，未部署服务、未删除文件，没有新建构建目录或试验产物；更新了既有前端类型检查/Vite 缓存。此前的数据库和服务器验证范围仍以各自记录为准。
