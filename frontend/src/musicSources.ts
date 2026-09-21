import bilibiliPlaylist from "./data/bilibili-playlist.json"
import neteasePlaylist from "./data/netease-playlist.json"

/** 站点歌单配置。收到真实歌单链接并验证曲目后填写，不使用示例歌曲冒充平台数据。 */
export interface PlatformTrack {
  id: string
  title: string
  artist: string
  url: string
  /** 经验证的平台官方 iframe 地址，不能使用音频解析或下载地址。 */
  embedUrl: string
}
export interface PlatformPlaylist {
  id: string
  platform: 'netease' | 'bilibili'
  title: string
  url: string
  tracks: PlatformTrack[]
  syncedAt?: string
  sourceCount?: number
  /** 已登记但未能读取曲目时，不将空数组当作空歌单。 */
  syncStatus?: 'pending' | 'ready'
  /** 平台提供整张歌单播放器时使用；与逐曲播放器互斥显示。 */
  embedUrl?: string
}
export const platformPlaylists: PlatformPlaylist[] = [
  { ...bilibiliPlaylist, platform: "bilibili", syncStatus: 'ready' },
  { ...neteasePlaylist, platform: 'netease', syncStatus: 'pending' },
]

export function officialEmbed(url?: string): string | undefined {
  if (!url) return undefined
  try {
    const parsed = new URL(url)
    if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.port) return undefined
    if (parsed.hostname === 'music.163.com' && parsed.pathname === '/outchain/player') return parsed.href
    if (parsed.hostname === 'player.bilibili.com' && parsed.pathname === '/player.html') return parsed.href
  } catch { /* 没有合法来源时不加载 iframe。 */ }
  return undefined
}
