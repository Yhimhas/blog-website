import type { PlatformTrack } from './musicSources'

export interface RecommendationTrack extends PlatformTrack {
  platform: 'bilibili' | 'netease'
  playlistId: string
}

export interface DailyRecommendations {
  date: string
  timezone: 'Asia/Shanghai'
  items: RecommendationTrack[]
}

function record(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('推荐数据格式不正确。')
  return value as Record<string, unknown>
}

function requiredString(value: unknown): string {
  if (typeof value !== 'string' || !value.trim()) throw new Error('推荐数据缺少必要字段。')
  return value
}

function officialUrl(value: unknown, provider: RecommendationTrack['platform'], embed = false): string {
  if (value === null) return ''
  const url = new URL(requiredString(value))
  const hosts = provider === 'bilibili' ? ['www.bilibili.com', 'bilibili.com'] : ['music.163.com']
  const validHost = embed
    ? provider === 'bilibili'
      ? url.hostname === 'player.bilibili.com' && url.pathname === '/player.html'
      : url.hostname === 'music.163.com' && url.pathname === '/outchain/player'
    : hosts.includes(url.hostname)
  if (url.protocol !== 'https:' || url.username || url.password || url.port || !validHost) {
    throw new Error('推荐数据包含非官方来源地址。')
  }
  return url.href
}

/** 校验整个快照；不丢弃坏条目后伪装成有效推荐或空结果。 */
export function parseRecommendations(payload: unknown): DailyRecommendations {
  const data = record(record(payload).data)
  const date = requiredString(data.date)
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || !Number.isFinite(Date.parse(date)) ||
      new Date(date).toISOString().slice(0, 10) !== date || data.timezone !== 'Asia/Shanghai' ||
      data.status !== 'ready' || !Array.isArray(data.items)) {
    throw new Error('推荐快照尚未就绪或格式不正确。')
  }
  const ids = new Set<string>()
  const items = data.items.map((value): RecommendationTrack => {
    const item = record(value)
    const id = requiredString(item.id)
    if (ids.has(id)) throw new Error('推荐数据包含重复曲目。')
    ids.add(id)
    if (item.provider !== 'bilibili' && item.provider !== 'netease') throw new Error('推荐来源不受支持。')
    if (item.author !== null && typeof item.author !== 'string') throw new Error('推荐作者格式不正确。')
    return {
      id,
      title: requiredString(item.title),
      artist: item.author?.trim() || '未知作者',
      platform: item.provider,
      playlistId: requiredString(item.playlistId),
      url: officialUrl(item.sourceUrl, item.provider),
      embedUrl: officialUrl(item.embedUrl, item.provider, true),
    }
  })
  return { date, timezone: 'Asia/Shanghai', items }
}

export async function fetchRecommendations(signal: AbortSignal): Promise<DailyRecommendations> {
  const response = await fetch('/api/v1/music/recommendations/today', {
    method: 'GET', headers: { Accept: 'application/json' }, cache: 'no-store', signal,
  })
  if (!response.ok) throw new Error(`推荐服务暂不可用（HTTP ${response.status}）。`)
  return parseRecommendations(await response.json())
}
