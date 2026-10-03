# 用户登录验证记录（2026-09-28）

> 后续操作统一使用 pnpm，见 [包管理约定](package-manager.md)。下文验收部分的 npm 命令保留为当时执行记录。

> 历史验收记录，结果仅适用于当时版本和环境。最新能力与后续验证入口见 [项目当前状态](project-status.md)；下文“未实现／未提交／连接失败”等表述不代表当前状态。

实现前的时间精度修复已提交为 `90f9489`。本次用户登录改动尚未提交、推送或部署。

## 验证结果

- 前端 `npm run test:blog`：5 项通过。
- 前端 `npm run build`：类型检查、内容检查和生产构建通过。Vite 配置保留旧构建文件。
- Linux Go 1.27.1、PostgreSQL 17：角色迁移在隔离库执行成功，版本 2，dirty=false。
- 使用服务器项目 `backend/.env` 中的独立测试库：`go test ./... -count=1 -v` 全部通过，数据库测试未跳过。
- Linux `go vet -buildvcs=false ./...`、`go build -buildvcs=false ./...` 通过。验证目录为源码副本，因此关闭 VCS stamping。
- 覆盖普通用户/管理员登录和会话角色、普通用户管理读取与写入拒绝、管理员降权即时生效、退出 CSRF 和会话失效，以及原有文章、音乐、推荐事务测试。
- Windows Go 检查遇到系统 Go 缓存访问限制，后端验证结果以上述 Linux 实际执行为准。

现有运行项目和应用数据库未更新。上线需要同步前后端代码，加载数据库配置，执行 `go run ./cmd/manage migrate`，再构建和重启服务。账号创建方法见 [用户登录说明](user-login.md)。

## 保留的验证产物

以下路径以各自注明的目录为基准，均未删除：

- 本地工作区父目录：`user-login-backend-validation.tar`，后端源码传输包。
- 本地项目：`frontend/dist/` 构建文件及 `frontend/node_modules/` 内 TypeScript/Vite 构建缓存。
- 服务器 `blog-web/`：`user-login-backend-validation.tar` 和 `user-login-validation-3bWPdqje/`，包含源码副本、`tests.log`、`configured-tests.log`。
- 原隔离实例 `blog-web/postgres-validation-9p3HHMOa/`：复用并保留，运行后已停止；本次新增 schema 为 `backend_test_6a44d65155e77df7`、`backend_test_6d66a319ae5434a9`。
- 配置指定的独立测试库：保留 schema `backend_test_9d9c9fccfe961287`、`backend_test_81a0a70a0098e8af`。

测试源码属于正式回归测试；以上传输包、验证副本和测试 schema 属于可清理的试验产物，当前按要求保留。
