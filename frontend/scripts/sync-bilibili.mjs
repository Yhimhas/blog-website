import { writeFile } from 'node:fs/promises'

// Public metadata only. This endpoint is used by Bilibili's web app and may change.
// Keep the last successful snapshot if any request or page validation fails.
const folderId = '3549762089'
const ownerId = '292715089'
const tracks = new Map()
let title = ''
let total = 0
let finished = false
for (let page = 1; page <= 100; page++) {
  const url = new URL('https://api.bilibili.com/x/v3/fav/resource/list')
  url.search = new URLSearchParams({ media_id: folderId, pn: String(page), ps: '20', platform: 'web' }).toString()
  const response = await fetch(url, { signal: AbortSignal.timeout(20000) })
  if (!response.ok) throw new Error(`Bilibili HTTP ${response.status}; previous snapshot retained`)
  const result = await response.json()
  if (result.code !== 0 || !result.data?.info || !Array.isArray(result.data.medias)) throw new Error(`Bilibili: ${result.code} ${result.message}; previous snapshot retained`)
  const { info, medias, has_more } = result.data
  if (String(info.id) !== folderId || String(info.upper?.mid) !== ownerId) throw new Error('Unexpected collection identity')
  title = String(info.title)
  total = Number(info.media_count)
  for (const video of medias) {
    const bvid = video.bvid || video.bv_id
    if (video.type !== 2 || !/^BV[0-9A-Za-z]{10}$/.test(bvid || '') || video.title === '已失效视频') continue
    tracks.set(bvid, {
      id: `bilibili:${bvid}`, title: String(video.title), artist: String(video.upper?.name || 'Bilibili'),
      url: `https://www.bilibili.com/video/${bvid}/`,
      embedUrl: `https://player.bilibili.com/player.html?bvid=${bvid}&p=1&autoplay=0&danmaku=0`,
    })
  }
  if (!has_more) { finished = true; break }
  if (!medias.length) throw new Error('Incomplete collection page; previous snapshot retained')
}
if (!finished) throw new Error('Collection exceeded page limit; previous snapshot retained')
const data = {
  id: `bilibili:${folderId}`, platform: 'bilibili', title,
  url: `https://space.bilibili.com/${ownerId}/favlist?fid=${folderId}&ftype=create`,
  syncedAt: new Date().toISOString(), sourceCount: total, tracks: [...tracks.values()],
}
await writeFile(new URL('../src/data/bilibili-playlist.json', import.meta.url), JSON.stringify(data, null, 2) + '\n')
console.log(`Synced ${title}: ${tracks.size}/${total} videos. No credentials or media files stored.`)
