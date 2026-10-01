import test from 'node:test'
import assert from 'node:assert/strict'
import { fetchMusicLibrary } from '../src/musicLibraryApi.ts'

const playlist = { id: 'netease:123', provider: 'netease', title: '测试歌单', sourceUrl: 'https://music.163.com/playlist?id=123', syncStatus: 'ready', syncedAt: '2026-10-02T00:00:00Z' }
const rawTrack = id => ({ id: `netease:${id}`, title: `song ${id}`, provider: 'netease', sourceUrl: `https://music.163.com/song?id=${id}`, availability: 'unknown' })
function libraryMock(t, pages) {
  let requests = 0
  t.mock.method(globalThis, 'fetch', async url => {
    if (url === '/api/v1/music/playlists') return Response.json({ data: [playlist] })
    assert.ok(url.includes(encodeURIComponent(playlist.id)))
    const page = pages[requests++]
    assert.ok(page, 'no unexpected pagination request')
    return Response.json(page)
  })
  return () => requests
}
const load = () => fetchMusicLibrary(new AbortController().signal)

test('reads all 265 tracks in order, keeping metadata availability unknown', async t => {
  const all = Array.from({ length: 265 }, (_, i) => rawTrack(i + 1))
  const pages = Array.from({ length: 6 }, (_, i) => ({ data: all.slice(i * 50, (i + 1) * 50), pagination: { total: 265 } }))
  const requests = libraryMock(t, pages)
  const result = await load()
  assert.equal(requests(), 6)
  assert.deepEqual(result[0].tracks.map(track => track.id), all.map(track => track.id))
  assert.ok(result[0].tracks.every(track => track.availability === 'unknown' && track.playlistId === playlist.id))
})

test('accepts an explicit complete empty playlist', async t => {
  libraryMock(t, [{ data: [], pagination: { total: 0 } }])
  const result = await load()
  assert.deepEqual(result[0].tracks, [])
  assert.equal(result[0].syncedAt, playlist.syncedAt)
})

test('rejects changed totals, duplicate IDs and excess rows without a partial library', async t => {
  for (const [name, pages, message] of [
    ['changed count', [{ data: [rawTrack(1)], pagination: { total: 2 } }, { data: [rawTrack(2)], pagination: { total: 3 } }], /发生变化/],
    ['duplicate across pages', [{ data: [rawTrack(1)], pagination: { total: 2 } }, { data: [rawTrack(1)], pagination: { total: 2 } }], /重复/],
    ['excess rows', [{ data: [rawTrack(1), rawTrack(2)], pagination: { total: 1 } }], /数量不一致/],
    ['missing page', [{ data: [rawTrack(1)], pagination: { total: 2 } }, { data: [], pagination: { total: 2 } }], /未加载完整/],
    ['negative count', [{ data: [], pagination: { total: -1 } }], /分页无效/],
    ['excess count', [{ data: [], pagination: { total: 2001 } }], /分页无效/],
    ['wrong provider', [{ data: [{ ...rawTrack(1), provider: 'bilibili', sourceUrl: 'https://www.bilibili.com/video/BV1a4MS67Eey/' }], pagination: { total: 1 } }], /来源不符/],
  ]) {
    await t.test(name, async t => {
      libraryMock(t, pages)
      await assert.rejects(load(), message)
    })
  }
})
