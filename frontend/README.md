# vue-project

## 每日推荐 API

音乐页请求同源 `GET /api/v1/music/recommendations/today`，使用后端返回的上海日期及曲目快照，不再调用本地抽选函数。音乐库仍保留本地歌单，当前推荐也可被搜索、选择和收藏。

开发时先按 `../backend/README.md` 启动正式后端及 PostgreSQL，再运行 `pnpm dev`；Vite 将 `/api` 转发到 `http://127.0.0.1:8081`。生产环境需由反向代理将同域 `/api` 转发到 Go 服务（Vite 的开发代理不随构建发布）。

页面区分加载中、空推荐、尚未生成和请求失败，10 秒超时，可手动重试；页面可见时每 30 秒检查上海日期并重试失败请求，跨日重新读取推荐。`pnpm test:music` 验证接口解析与错误处理。后端未启动或尚无当天推荐时会显示真实状态。

## 文章维护

博客读取 `src/content/posts/*.md`。新增文章、发布日期、摘要、标签和 slug 的填写方式见 [本地文章内容库说明](src/content/README.md)。提交前运行 `pnpm check:content` 和 `pnpm test:content`；`pnpm build` 也会校验内容。

This template should help get you started developing with Vue 3 in Vite.

## Recommended IDE Setup

[VS Code](https://code.visualstudio.com/) + [Vue (Official)](https://marketplace.visualstudio.com/items?itemName=Vue.volar) (and disable Vetur).

## Recommended Browser Setup

- Chromium-based browsers (Chrome, Edge, Brave, etc.):
  - [Vue.js devtools](https://chromewebstore.google.com/detail/vuejs-devtools/nhdogjmejiglipccpnnnanhbledajbpd)
  - [Turn on Custom Object Formatter in Chrome DevTools](http://bit.ly/object-formatters)
- Firefox:
  - [Vue.js devtools](https://addons.mozilla.org/en-US/firefox/addon/vue-js-devtools/)
  - [Turn on Custom Object Formatter in Firefox DevTools](https://fxdx.dev/firefox-devtools-custom-object-formatters/)

## Type Support for `.vue` Imports in TS

TypeScript cannot handle type information for `.vue` imports by default, so we replace the `tsc` CLI with `vue-tsc` for type checking. In editors, we need [Volar](https://marketplace.visualstudio.com/items?itemName=Vue.volar) to make the TypeScript language service aware of `.vue` types.

## Customize configuration

See [Vite Configuration Reference](https://vite.dev/config/).

## Project Setup

```sh
pnpm install
```

### Compile and Hot-Reload for Development

```sh
pnpm dev
```

### Type-Check, Compile and Minify for Production

```sh
pnpm build
```
