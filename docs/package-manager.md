# 包管理约定：统一使用 pnpm

生效日期：2026-10-03。适用于开发者、AI 助手、测试、构建及后续部署脚本。

本项目使用 pnpm，唯一前端依赖锁文件为 `frontend/pnpm-lock.yaml`。`frontend/package.json` 的 `packageManager` 固定 pnpm 11.19.0。系统存在 npm 不代表项目使用 npm。不得因旧文档或命令习惯切换包管理器，也不要新增 `package-lock.json` 或 `yarn.lock`；已有文件如需清理，必须先向用户确认。

## 常用命令

以下命令均在 `frontend/` 执行：

```sh
pnpm --version
pnpm install
pnpm run dev --host localhost --port 5173 --strictPort
pnpm run test:content
pnpm run test:blog
pnpm run test:music
pnpm run test:seo
pnpm run type-check
```

CI 或复现锁定依赖时使用 `pnpm install --frozen-lockfile`。需要完整构建依赖时不要省略 devDependencies。调用已安装工具用 `pnpm exec`，不用 `npx`。

构建参数直接跟在脚本名后，不照搬 npm 的额外 `--` 分隔形式：

```sh
# 本地验证：不用于正式发布，使用独立输出目录。
pnpm run build --mode development --outDir dist/pnpm-preview-001

# 正式构建：先配置所需环境和可访问的新版 API，每次使用新空目录。
pnpm run build --outDir dist/releases/release-001
```

正式构建条件见 [SEO 与链接分享](seo-and-sharing.md)。目录编号每次更换，不自动删除旧产物。这里只规定命令用法，不代表本次已经执行构建。

## 排查与历史记录

- pnpm 找不到时先检查安装及 PATH，不擅自安装其他包管理器或退回 npm。
- 检查实际 pnpm 版本、Node.js 版本及锁文件，再复现问题。非 pnpm 环境的失败只能作为该环境的观察结果。
- 先前在 npm 11.19.0 下观察到的构建参数转发失败，不构成 pnpm 构建失败的证据。
- 历史验收记录中的 `npm run ...` 是当时实际执行的命令，不改写历史测试证据；后续操作按本文使用 pnpm。

## 构建与部署调用

[package.json](../frontend/package.json) 的 `build-only` 通过 pnpm 执行内容检查，[SEO Worker](../backend/cmd/seo-refresh/main.go) 通过 `pnpm run build-only` 启动正式构建，参数直接传入，不使用 npm 的额外 `--`。`npm-run-all2` 是支持 pnpm 的脚本调度工具，包名不代表会切换到 npm。

服务的非交互 PATH 必须包含指定版本的 pnpm 和受支持的 Node.js；升级 Worker 后需重新构建其二进制并按既有流程部署。本次源码修改不代表运行中的服务器已升级。

`pnpm run test:seo:build` 使用 pnpm 的实际入口和 package scripts 验证正式构建，同时覆盖带空格的输出路径；不再绕过脚本直接调用 Vite。历史验收结果仍按原执行范围解读。

pnpm 11 可能在执行脚本前自动检查或安装依赖。仅复验现有依赖、明确不执行安装时，可设置进程环境 `pnpm_config_verify_deps_before_run=false`；这是当前进程的验证选项，不修改项目配置，也不能替代正式部署前的冻结锁文件安装。

## 本次统一后的验证（2026-10-03）

- pnpm 11.19.0 执行 `build --mode development --outDir ...`：类型检查、内容检查与打包通过，内部内容检查也使用 pnpm。
- `pnpm run test:seo:build` 通过真实 pnpm → build-only → 内容检查 → Vite production 调用链，覆盖发布、更新、归档、过期构建拒绝和带空格的输出路径；测试 API 已停止。
- Go SEO 文件系统测试及 Worker vet 通过，Linux `go build ./cmd/...` 交叉编译通过。数据库与 Linux 专属运行测试在本机跳过；未声称服务器 pnpm/systemd 已验收或部署。
- 初次 Go 文件系统测试受沙箱重命名权限限制；在沙箱外使用明确保留目录复验通过。pnpm 验证临时关闭自动安装，没有安装依赖或变更锁文件。
- 保留产物（相对于仓库根目录）：`frontend/dist/pnpm-unified-validation-20261003/`、`frontend/dist/seo-refresh-pnpm validation-9xPgvL/`、`../.validation-cache/pnpm-migration-seo-tests/`，以及初次沙箱测试临时目录中的 `seo-test-*` 与既有编译缓存。没有删除文件。

后续服务器实际使用隔离 pnpm 11.19.0 完成 API → Worker/timer → pnpm production 构建 → Nginx 闭环；没有依赖 npm 环境结论。PATH、版本加载、复制依赖的范围、失败注入及保留产物见 [服务器续验记录](seo-pnpm-server-validation-20261003.md)。正式服务没有在本次启用。
