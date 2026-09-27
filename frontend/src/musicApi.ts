export interface RecommendedTrack {
  id: string
  title: string
  artist: string
  url: string
  platform: 'netease' | 'bilibili'
  playlistId: string
}

export class RecommendationError extends Error {}

function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Invalid response')
  return value as Record<string, unknown>
}

export function parseRecommendations(payload: unknown): { date: string; items: RecommendedTrack[] } {
  const data = object(object(payload).data)
  if (typeof data.date !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(data.date) ||
    data.timezone !== 'Asia/Shanghai' || data.status !== 'ready' || !Array.isArray(data.items)) {
    throw new Error('Invalid recommendation response')
  }
  const seen = new Set<string>()
  const items = data.items.map((value): RecommendedTrack => {
    const track = object(value)
    if (typeof track.id !== 'string' || !track.id || seen.has(track.id) ||
      typeof track.title !== 'string' || !track.title.trim() ||
      (track.provider !== 'netease' && track.provider !== 'bilibili') ||
      (track.author != null && typeof track.author !== 'string') ||
      typeof track.sourceUrl !== 'string' || track.availability !== 'available' ||
      (track.playlistId != null && typeof track.playlistId !== 'string')) throw new Error('Invalid track')
    const url = new URL(track.sourceUrl)
    const host = track.provider === 'bilibili' ? 'www.bilibili.com' : 'music.163.com'
    if (url.protocol !== 'https:' || url.hostname !== host || url.username || url.password || url.port) {
      throw new Error('Invalid official URL')
    }
    seen.add(track.id)
    return {
      id: track.id, title: track.title, artist: (track.author as string | null) || '未知作者',
      url: url.href, platform: track.provider,
      playlistId: (track.playlistId as string | undefined) || '',
    }
  })
  return { date: data.date, items }
}

export async function fetchTodayRecommendations(signal?: AbortSignal) {
  const response = await fetch('/api/v1/music/recommendations/today', {
    signal, headers: { Accept: 'application/json' }, cache: 'no-store',
  })
  if (!response.ok) {
    const body = await response.json().catch(() => null)
    if (response.status === 503 && body?.error?.code === 'RECOMMENDATION_NOT_READY') {
      throw new RecommendationError('今日推荐尚未生成，请稍后重试。')
    }
    throw new RecommendationError('推荐暂时无法加载，请稍后重试。')
  }
  return parseRecommendations(await response.json())
}
