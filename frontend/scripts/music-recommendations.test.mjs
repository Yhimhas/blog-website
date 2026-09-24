import assert from 'node:assert/strict'
import test from 'node:test'
import { effectScope, nextTick, ref } from 'vue'
import { parseRecommendations, fetchRecommendations } from '../src/musicRecommendations.ts'
import { useMusicRecommendations } from '../src/useMusicRecommendations.ts'

const item = {
  id: 'bilibili:api-only', playlistId: 'bilibili:new-playlist', provider: 'bilibili',
  title: 'API 曲目', author: '作者', sourceUrl: 'https://www.bilibili.com/video/BV123',
  embedUrl: 'https://player.bilibili.com/player.html?bvid=BV123',
}
const payload = (items = [item]) => ({ data: { date: '2026-09-24', timezone: 'Asia/Shanghai', status: 'ready', items } })
const snapshot = () => parseRecommendations(payload())
const settle = async () => { await Promise.resolve(); await nextTick() }
function mount(t, options = {}) {
  const scope = effectScope()
  const day = ref('2026-09-25')
  const result = scope.run(() => useMusicRecommendations({ mode: 'api', day, localTracks: snapshot().items, ...options }))
  t.after(() => scope.stop())
  return { ...result, day, scope }
}

test('DTO maps remote-only tracks and keeps server date/order; nullable links do not invent players', () => {
  const result = parseRecommendations(payload([item, { ...item, id: 'netease:42', provider: 'netease', author: null, sourceUrl: null, embedUrl: null }]))
  assert.equal(result.date, '2026-09-24')
  assert.deepEqual(result.items.map(track => track.id), [item.id, 'netease:42'])
  assert.equal(result.items[0].artist, '作者')
  assert.equal(result.items[0].url, item.sourceUrl)
  assert.equal(result.items[1].embedUrl, '')
  assert.equal(result.items[1].artist, '未知作者')
})

test('invalid snapshots fail as a whole, including unsafe URLs and duplicate IDs', () => {
  for (const data of [null, {}, { ...payload().data, status: 'pending' },
    { ...payload().data, date: '2026-02-30' }, { ...payload().data, timezone: 'UTC' },
    { ...payload().data, items: null }]) assert.throws(() => parseRecommendations({ data }))
  for (const patch of [{ provider: 'other' }, { title: '' }, { playlistId: null },
    { sourceUrl: 'javascript:alert(1)' }, { embedUrl: 'https://evil.example/player.html' }]) {
    assert.throws(() => parseRecommendations(payload([{ ...item, ...patch }])))
  }
  assert.throws(() => parseRecommendations(payload([item, item])))
  assert.deepEqual(parseRecommendations(payload([])).items, [])
})

test('HTTP errors, invalid JSON and bad envelopes never become successful empty results', async t => {
  const signal = new AbortController().signal
  let response = new Response('', { status: 503 })
  t.mock.method(globalThis, 'fetch', async (url, options) => {
    assert.equal(url, '/api/v1/music/recommendations/today')
    assert.equal(options.signal, signal)
    return response
  })
  await assert.rejects(fetchRecommendations(signal), /503/)
  response = new Response('<html>Vite fallback</html>')
  await assert.rejects(fetchRecommendations(signal))
  response = Response.json({ data: [] })
  await assert.rejects(fetchRecommendations(signal))
  response = Response.json(payload([]))
  assert.deepEqual((await fetchRecommendations(signal)).items, [])
})

test('loading → error clears data, retry → ready uses API, successful [] → empty', async t => {
  let finish
  let request = () => new Promise((resolve, reject) => { finish = { resolve, reject } })
  const model = mount(t, { request: signal => request(signal) })
  assert.equal(model.state.value.status, 'loading')
  assert.deepEqual(model.recommendations.value, [])
  finish.reject(new Error('offline'))
  await settle()
  assert.equal(model.state.value.status, 'error')
  assert.deepEqual(model.recommendations.value, [])
  request = async () => snapshot()
  await model.reload()
  assert.equal(model.state.value.status, 'ready')
  assert.equal(model.recommendationDate.value, '2026-09-24')
  request = async () => { throw new Error('offline again') }
  await model.reload()
  assert.equal(model.state.value.status, 'error')
  assert.deepEqual(model.recommendations.value, [])
  request = async () => parseRecommendations(payload([]))
  await model.reload()
  assert.equal(model.state.value.status, 'empty')
})

test('day changes cancel previous requests; stale completion cannot overwrite a new result', async t => {
  const calls = []
  const model = mount(t, { request: signal => new Promise(resolve => calls.push({ signal, resolve })) })
  model.day.value = '2026-09-26'
  await nextTick()
  assert.equal(calls[0].signal.aborted, true)
  calls[1].resolve(parseRecommendations(payload([])))
  await settle()
  calls[0].resolve(snapshot())
  await settle()
  assert.equal(model.state.value.status, 'empty')
  const running = model.reload()
  model.scope.stop()
  assert.equal(calls[2].signal.aborted, true)
  calls[2].resolve(snapshot())
  await running
  assert.equal(model.state.value.status, 'loading')
})

test('timeout becomes error and a late response is ignored', async t => {
  let finish
  let signal
  const model = mount(t, { timeoutMs: 5, request: active => { signal = active; return new Promise(resolve => { finish = resolve }) } })
  await new Promise(resolve => setTimeout(resolve, 20))
  assert.equal(model.state.value.status, 'error')
  assert.equal(signal.aborted, true)
  finish(snapshot())
  await settle()
  assert.equal(model.state.value.status, 'error')
})

test('explicit local fallback makes no API request', async t => {
  const model = mount(t, { mode: 'local', request: () => assert.fail('local fallback must not fetch') })
  assert.equal(model.localFallback, true)
  assert.equal(model.state.value.status, 'ready')
  assert.equal(model.recommendationDate.value, '2026-09-25')
  await model.reload()
  assert.equal(model.recommendations.value.length, 1)
})
