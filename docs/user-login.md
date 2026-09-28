# 用户登录与管理员权限

统一入口为 `/login`。未登录时导航显示「用户登录」，登录后显示「我的账号」；只有服务器返回 `role=admin` 才显示「文章管理」。直接访问 `/admin` 或 `/admin/local` 同样检查会话。普通用户访问管理 API 返回 `403 ADMIN_REQUIRED`，未登录返回 401。

部署前在 `backend/` 加载服务器环境配置，执行 `go run ./cmd/manage migrate`。迁移 000002 为既有账号保留 admin 身份，新账号默认 user；不删除账号、不改变密码和现有会话。历史表名 `admin_users`、会话外键 `admin_id` 保留以兼容已有数据。

普通用户通过管理命令创建（当前不提供公开注册）：

```bash
read -r -p '用户名: ' USER_USERNAME
read -r -s -p '密码（12–72 字节）: ' USER_PASSWORD
printf '\n'
export USER_USERNAME USER_PASSWORD
go run ./cmd/manage create-user
unset USER_PASSWORD
```

管理员继续使用 `ADMIN_USERNAME`、`ADMIN_PASSWORD` 和 `create-admin`。请先配置 `DATABASE_URL`，不要将密码提交到仓库。

会话接口为 `POST /api/v1/session`、`GET /api/v1/session`、`POST /api/v1/session/logout`。旧 `/admin/session` 路径保留兼容。会话返回 user 的 id、username、role；GET 同时返回 csrfToken。所有管理接口在后端重新读取角色，不接受客户端指定权限。退出仍需 Origin 和 CSRF 校验。

数据库验证使用独立 `TEST_DATABASE_URL` 和 `ALLOW_TEST_SCHEMA_CREATE=true` 执行 `go test ./... -count=1`。测试 schema 保留供检查，不自动删除。
