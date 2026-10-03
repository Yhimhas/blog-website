# blog-website

> 包管理器统一使用 **pnpm**。安装、开发、测试与构建遵循 [包管理约定](docs/package-manager.md)；历史 npm 命令不作为后续操作指引。

个人博客与音乐网站，使用 Vue 3 + TypeScript + Vite、Go + Gin + GORM 和 PostgreSQL。

## 当前进度

统一入口：[项目当前状态与文档导航](docs/project-status.md)。已实现博客发布与修订、归档和分类标签管理、账号权限、歌单同步、每日推荐、跨页面音乐播放与进度定位，以及 SEO 静态快照。生产部署和剩余验收以状态文档为准。

## 开发与操作入口

- [前端 README](frontend/README.md)：开发启动、页面行为与构建。
- [后端 README](backend/README.md)：环境、迁移、账号、API 和测试。
- [OpenAPI](api/openapi.yaml)：接口字段、权限和错误契约。
- [用户登录与管理员权限](docs/user-login.md)：账号创建与角色管理。
- [SEO 与链接分享](docs/seo-and-sharing.md)：正式构建、静态发布和内容更新流程。
- [公开播放实施与验收](docs/music-public-playback-implementation-20261002.md)：播放、定位、策略、配置及验收限制。
- [网易云完整歌单实现](docs/netease-playlist-completion-implementation-20261002.md)与[服务器验证](docs/netease-playlist-server-validation-20261002.md)：同步实现与证据。

早期方案与分阶段记录保留在原位置。请先阅读当前状态，再按专题查阅历史资料；不再用早期规划中的“尚未实现”判断当前进度。
