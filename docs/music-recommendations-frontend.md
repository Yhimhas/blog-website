# 每日推荐前端适配

`MusicView.vue` 使用 `useMusicRecommendations.ts` 管理状态，`musicRecommendations.ts` 负责同源 GET、响应校验和 DTO 映射。未配置时使用 API；开发环境只有显式设置 `VITE_MUSIC_RECOMMENDATIONS_SOURCE=local` 才使用原有 `dailySelection`。生产构建始终使用 API。

本地开发可在 frontend 的 `.env.local` 中设置以下内容，然后重启 Vite：

```dotenv
VITE_MUSIC_RECOMMENDATIONS_SOURCE=local
```

页面会标注“开发期本地 fallback · 非 API 推荐”。接入 API 时移除此设置或改为 `api`。同源 `/api/v1` 需由开发网关或生产反向代理路由到后端；本次不增加后端或 Vite proxy。没有后端时默认显示 error，绝不自动使用本地推荐。

响应沿用 backend-development-guide.md：

```json
{
  "data": {
    "date": "2026-09-25",
    "timezone": "Asia/Shanghai",
    "status": "ready",
    "items": [{
      "id": "netease:123456",
      "playlistId": "netease:595975585",
      "provider": "netease",
      "title": "曲目标题",
      "author": null,
      "sourceUrl": "https://music.163.com/#/song?id=123456",
      "embedUrl": null
    }]
  }
}
```

`author → artist`、`sourceUrl → url`、`provider → platform`；保留服务端曲目 ID、playlistId、日期、顺序和官方 embedUrl。author/sourceUrl/embedUrl 可为 null；无作者显示“未知作者”，无播放器不补造地址。其他 DTO 字段不影响现有页面。格式错误、重复 ID、未知 provider、非法来源 URL 均使整个快照进入 error，不能通过过滤坏条目变成 empty。来源地址要求 HTTPS 官方 host，embed 还校验官方播放器路径。

| 状态 | 行为 |
| --- | --- |
| loading | 首次加载、重试和上海日期变更时清空推荐区，显示加载提示 |
| error | HTTP 非成功、网络/JSON/契约错误或 10 秒超时；显示重试，不使用本地数据 |
| empty | 仅合法 ready 快照的 items 为 []；显示今日暂无推荐 |
| ready | 使用服务端 items，不在客户端重新抽样；标题日期来自响应 |

每 30 秒及 visibilitychange 检查上海日期；日期变更重新请求 today。取消旧请求并用版本号丢弃迟到结果；卸载时取消请求和定时器。API 日期未返回前不会用浏览器日期伪装服务端日期。已手动选择的曲目可以继续保留在播放器信息区，但不保留旧推荐列表冒充新请求结果。

接口返回的独立曲目可选择、搜索及在本次浏览中收藏；收藏仍只持久化 ID，没有本地元数据的 API 曲目在重新加载后需接口再次返回才能显示。未实现完整服务端音乐库或收藏同步。已有官方页面入口、官方 iframe 校验函数、歌单上下首逻辑和页面动效不变。当前 MusicView 实际没有渲染 iframe，本次也不新增播放器。

验证命令（frontend 目录）：

```powershell
node --experimental-strip-types --test scripts/music-daily.test.mjs scripts/music-recommendations.test.mjs
npm run type-check
npm run build
```

Vite 配置禁用清空 dist，以遵守不删除现有文件的要求；旧哈希产物可能保留。自动化测试覆盖 DTO、失败不 fallback、四态、重试、超时、跨日取消、迟到响应和卸载。真实后端联调需另行验证，不将 mock 测试当成真实推荐服务可用的证明。
