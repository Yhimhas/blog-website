# blog-website
a personnal website for blogging and music

## 开发入口

- [Go 后端开发与前后端协作手册](docs/backend-development-guide.md)：当前技术栈、API 契约、学习顺序与联调验收。
- [Go 后端](backend/README.md)：PostgreSQL 博客、站长登录、音乐元数据与每日推荐、显式迁移、启动及测试说明。
- [OpenAPI](api/openapi.yaml)：第一阶段接口、权限、分页和字段契约。
- [Vue 前端](frontend/README.md)：`main` 保留完整的 `frontend/`（Vue 3 + TypeScript + Vite）；前端功能在 `feat/frontend` 分支维护，可使用独立 worktree 与 Go 后端联调。
- [纯音频流式设计](docs/music-audio-streaming-design.md)：后续音乐服务设计，尚未实现。
