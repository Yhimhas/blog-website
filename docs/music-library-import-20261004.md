# 正式曲库导入（2026-10-04）

本次根据用户指定来源，将实时公开元数据写入正式应用库 `blog_dev`。操作前已备份；不使用平台账号 Cookie，不下载或保存音频。

| 来源 | ID | 导入与读回 |
| --- | --- | --- |
| Bilibili 收藏夹「哈基米」 | `bilibili:3549762089`，所有者 `292715089` | 实时 51 条，3 页上游读取；本站 2 页完整读回 |
| 网易云「我喜欢的音乐」 | `netease:595975585` | 267 首，通过现有 Adapter 补齐详情及复查；本站 6 页完整读回 |

来源：[Bilibili 收藏夹](https://space.bilibili.com/292715089/favlist?fid=3549762089&ftype=create)、[网易云歌单](https://music.163.com/m/playlist?id=595975585&creatorId=417762656)。

Bilibili 实时收藏数为 51，旧本地快照的 53 不是本次使用的数据。读取时验证了收藏夹 ID、所有者、各页总数、唯一 BVID 和结束页，并复查第一页，51 条均为有效视频元数据。默认导入视频第 1 P，本站 ID 带 `:1`。

网易云复用服务器现有源码中的 `netease.Client.FetchPlaylist`，不手写另一套补齐规则。两份快照都通过现有 `manage import-music` 的 DTO 校验和快照事务写入，来源均为 ready，合计 318 条。完整分页读回的 ID 顺序与导入快照一致，无重复或遗漏。本站 HTTPS `/ready` 返回 200。

## 使用与边界

刷新音乐页后选择「曲库」即可查看，搜索覆盖两个来源的全部曲目。元数据导入后 availability 均为 unknown；此状态允许按既有播放器流程尝试音源，不代表全部歌曲完整可播。

2026-10-04 的每日推荐此前已保存为空，本次导入不会改写同日历史，所以「每日推荐」页仍可能为空。推荐候选还要求实际公共播放可用性检查，不将 unknown 直接改为 available。本次没有全曲播放探测、会员账号接入或人工听验。

当前导入属于管理命令导入快照，任务记录为 local-import。后续收藏夹变化需要重新读取并导入；本次没有启用每日自动同步或新增 Bilibili 后端同步 Adapter。

## 备份与正式快照

以下服务器路径相对于登录用户主目录：

- `blog-web/production/backups/music-import-20261004/20261003T170207Z-e18fcf.dump`：导入前应用库备份；文件名使用 UTC，上海日期为 2026-10-04。
- `blog-web/production/music-snapshots/bilibili-3549762089-20261004.json`：实际导入的收藏夹元数据。
- `blog-web/production/music-snapshots/netease-595975585-20261004.json`：实际导入的网易云元数据。
- `blog-web/production/music-snapshots/import-20261004-result.json`：读回数量、顺序哈希及当日推荐状态。

这些是正式数据快照及操作备份，权限限制为 0600。未将数据库凭据或备份传回本机。一次性 Go helper、专用编译缓存和容器临时输入 JSON 已清理；没有新增无用试验文件。
