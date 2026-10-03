# 项目协作约定

- 使用中文回复，英文专用词不强行翻译。
- 不删除任何文件；即使用户要求删除，也须再次确认。生成的无用残留或试验产物须列出。
- 项目文档使用相对路径。

## 包管理器：统一使用 pnpm

- 本项目唯一包管理器是 pnpm，前端锁文件为 `frontend/pnpm-lock.yaml`。
- 安装、开发、测试、类型检查、构建和预览一律使用 pnpm；不要因为系统安装了 npm 或历史文档使用 npm 而切换工具。
- 在 `frontend/` 执行 `pnpm install`、`pnpm run dev`、`pnpm run test:blog`、`pnpm run type-check`、`pnpm run build`；调用本地工具使用 `pnpm exec`。
- 不运行 `npm install`、`npm run`、`npx` 或 yarn，不新增其他包管理器的锁文件，也不自动删除已有文件。
- pnpm 不可用时，先核对 PATH 和现有安装，报告阻塞；不能擅自改用 npm。
- 问题结论必须基于 pnpm 的复现结果。npm 下失败不能直接当作本项目构建失败。
- 历史验收记录中的 npm 命令保留为事实记录，不作为后续操作指引。新增文档和脚本使用 pnpm。
- pnpm 版本遵循 `frontend/package.json` 的 `engines.pnpm`：`>=11.19.0`，不固定具体版本，也不新增精确版本的 `packageManager` 声明。构建脚本及 SEO Worker 同样使用 pnpm，不回退 npm。详情见 [包管理约定](docs/package-manager.md)。
