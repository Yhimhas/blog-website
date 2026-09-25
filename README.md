# blog-website
a personnal website for blogging and music

## 开发入口

- [Go 后端开发与前后端协作手册](docs/backend-development-guide.md)：当前技术栈、API 契约、学习顺序与联调验收。
- [Go 后端](backend/README.md)：PostgreSQL 博客、站长登录、音乐元数据与每日推荐、显式迁移、启动及测试说明。
- [OpenAPI](api/openapi.yaml)：第一阶段接口、权限、分页和字段契约。
- 前端：`frontend/`，Vue 3 + TypeScript + Vite；助手负责前端，你负责 Go 后端实现。
- [纯音频流式设计](docs/music-audio-streaming-design.md)：后续音乐服务设计，尚未实现。
