import assert from 'node:assert/strict'
import { test } from 'node:test'
import { fetchTodayRecommendations, parseRecommendations } from '../src/musicApi.ts'

const track = { id: 'netease:123', provider: 'netease', title: '测试曲目', author: null,
  sourceUrl: 'https://music.163.com/song?id=123', availability: 'available' }
const payload = (items = [track]) => ({ data: { date: '2026-09-27', timezone: 'Asia/Shanghai', status: 'ready', items } })

test('maps backend snapshots without requiring a local playlist and preserves order', () => {
  const result = parseRecommendations(payload([track, { ...track, id: 'netease:456' }]))
  assert.equal(result.date, '2026-09-27')
  assert.deepEqual(result.items.map(item => item.id), ['netease:123', 'netease:456'])
  assert.equal(result.items[0].artist, '未知作者')
  assert.equal(result.items[0].platform, 'netease')
  assert.equal(result.items[0].playlistId, '')
})

test('a completed empty result remains empty', () => {
  assert.deepEqual(parseRecommendations(payload([])).items, [])
})

test('rejects malformed data, duplicate IDs, unavailable tracks and unsafe links', () => {
  for (const invalid of [null, {}, { data: [] }, payload([track, track]),
    payload([{ ...track, availability: 'unknown' }]),
    payload([{ ...track, sourceUrl: 'javascript:alert(1)' }]),
    payload([{ ...track, sourceUrl: 'https://evil.example/song' }])]) {
    assert.throws(() => parseRecommendations(invalid))
  }
})

test('fetches same-origin daily API without cache and forwards cancellation', async (t) => {
  const controller = new AbortController()
  t.mock.method(globalThis, 'fetch', async (url, options) => {
    assert.equal(url, '/api/v1/music/recommendations/today')
    assert.equal(options.signal, controller.signal)
    assert.equal(options.cache, 'no-store')
    return Response.json(payload())
  })
  assert.equal((await fetchTodayRecommendations(controller.signal)).items.length, 1)
})

test('distinguishes recommendations not ready from other server failures', async (t) => {
  const mock = t.mock.method(globalThis, 'fetch', async () => Response.json(
    { error: { code: 'RECOMMENDATION_NOT_READY' } }, { status: 503 }))
  await assert.rejects(fetchTodayRecommendations(), /尚未生成/)
  mock.mock.mockImplementation(async () => new Response('<html>Bad gateway</html>', { status: 502 }))
  await assert.rejects(fetchTodayRecommendations(), /暂时无法加载/)
  mock.mock.mockImplementation(async () => { throw new TypeError('Network error') })
  await assert.rejects(fetchTodayRecommendations(), /Network error/)
})
