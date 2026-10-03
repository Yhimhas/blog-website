# 音乐歌单同步错误分类与失败提示（2026-10-02）

> 后续操作统一使用 pnpm，见 [包管理约定](package-manager.md)。下文验收部分的 npm 命令保留为当时执行记录。

本次根据 [服务器实测记录](music-server-validation-20260930.md) 和 [网易云技术说明](netease-playlist-sync-and-playback-technical-guide.md) 修正本地实现。没有访问服务器、网易云真实接口或任何数据库，没有部署，没有删除文件。

## 分类与调用链

调用链为管理员同步 API → `Service.StartSync` → 网易云 `FetchPlaylist` / `Parse` → 完整字段校验 → `ReplaceSnapshot`。只有成功并通过校验的结果才进入快照事务；错误进入 `Service.fail`。

| 响应 | 项目错误码 | 音乐页提示 |
| --- | --- | --- |
| HTTP 200、JSON code=20001（含空 msg） | `UPSTREAM_ACCESS_RESTRICTED` | 网易云未提供此歌单的可用数据，本次未完成同步。 |
| 其他明确非成功业务码 | `UPSTREAM_REJECTED` | 网易云未提供此歌单的可用数据，本次未完成同步。 |
| HTTP / 业务码 401、403 | `UPSTREAM_UNAUTHORIZED` | 网易云拒绝了本次歌单读取请求。 |
| HTTP / 业务码 429 | `UPSTREAM_RATE_LIMITED` | 网易云请求受限，请稍后再试。 |
| 成功响应的声明数量与实际曲目数不一致 | `INCOMPLETE_PLAYLIST` | 未取得完整的歌单曲目，本次未完成同步。 |
| JSON 损坏、code 缺失/null/类型错误、成功数据字段异常 | `INVALID_PAYLOAD` | 网易云响应格式异常，本次未完成同步。 |

先解析业务状态，再解析成功响应的歌单字段，避免明确的业务拒绝被成功字段校验误归类。保留最多 2000 首、字段合法性和完整空歌单校验，不新增未经验证的详情补齐接口。

`UPSTREAM_ACCESS_RESTRICTED` 是项目分类名称，不能据此推断私密、必须登录或版权原因。页面仅使用固定文案，不展示上游 msg 或未知错误码原文。

## 数据与公开契约

- [Source / Service](../backend/internal/music/service.go) 将已有 `last_error_code` 列映射为可空的公开 `syncErrorCode`，无需数据库迁移；[OpenAPI](../api/openapi.yaml) 同步补充契约。
- 重试进入 running 时清除旧错误；失败事务只更新同步任务和来源的失败状态、错误及相关时间，不修改曲目、关联顺序/active、`snapshot_hash` 或 `synced_at`。成功提交清除错误。
- [musicLibraryApi.ts](../frontend/src/musicLibraryApi.ts) 传递 `syncErrorCode` 和 `syncedAt`，兼容旧 API 没有这些字段的情况。缺失或未知错误码使用无法确认具体原因的提示。
- [MusicView.vue](../frontend/src/views/MusicView.vue) 根据当前歌单的 `syncedAt` 区分“已保留原有歌单”和“尚无完整同步记录”；成功同步的空歌单仍有快照。另一歌单的人工样本不作为目标歌单的快照依据。
- 失败时已有曲目继续可搜索和选择，保留重新加载和原站入口。重新加载只读取状态，不会触发管理员同步。

历史运行中已保存的 `INVALID_PAYLOAD` 不会被自动改写；无法仅从该历史错误码还原上游 code。以后重新执行同步时才会使用新分类。本次未执行真实同步。

## 本地验证

后端在 backend 目录执行 `go test ./... -count=1`、`go vet ./...`、`go build ./...` 均通过。执行时清空进程内 `TEST_DATABASE_URL` 和 `ALLOW_TEST_SCHEMA_CREATE`，确保不连接数据库。

- [网易云 fixture 测试](../backend/internal/music/provider/netease/client_test.go)：HTTP 200 / code=20001、未知业务码、401/403、429、业务拒绝优先于成功字段校验、坏 JSON/code/字段和不完整曲目，以及原有完整/空歌单。
- [本地 SQL stub 测试](../backend/internal/music/service_test.go)：使用标准库内存连接和真实 GORM SQL 生成，覆盖 `StartSync` → worker → fail → 公开查询；断言分类实际写入、重试清除旧错误、没有额外快照写操作、公开字段正确序列化。该测试不建立网络或数据库连接。
- [PostgreSQL 集成测试](../backend/internal/storage/integration_test.go) 补充 code=20001 的失败保留、公开错误字段、时间/hash 和后续成功清除错误断言。本次因未配置测试数据库而 **SKIP**，不能算实际数据库事务验收通过。
- 前端 `npm run test:music`：50 项通过；`npm run type-check` 通过。[页面 SSR 回归](../frontend/scripts/music-view.test.mjs) 使用真实 SFC 和 API composable，覆盖有/无旧快照、空快照、人工样本隔离、错误提示、未知码安全降级和已有曲目搜索。
- `git diff --check` 通过；没有真实上游同步、浏览器播放或服务器验收结论。

## 保留产物

正式产物为本次代码、测试、OpenAPI 变更及本文。原有两份实测/技术文档保留。

首次 Go 测试遇到系统默认缓存访问受限，后改用仓库上级的 `.validation-cache/go/`（相对本项目根目录为 `../.validation-cache/go/`），该目录保留本地构建缓存，不需要提交。TypeScript 检查更新既有 `frontend/node_modules/.tmp/tsconfig.app.tsbuildinfo` 和 `tsconfig.node.tsbuildinfo`。没有新增媒体、打包文件、测试日志或部署二进制，没有清理这些缓存。
