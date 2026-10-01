import assert from 'node:assert/strict'
import { after, test } from 'node:test'
import { createSSRApp } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createServer } from 'vite'
import vue from '@vitejs/plugin-vue'

// Load the real SFC and API composable without emitting temporary build files.
const server = await createServer({ configFile: false, plugins: [vue()], optimizeDeps: { noDiscovery: true, include: [] }, server: { middlewareMode: true }, appType: 'custom' })
after(() => server.close())
const { default: MusicView } = await server.ssrLoadModule('/src/views/MusicView.vue')
const target = { id: 'netease:595975585', provider: 'netease', title: '指定歌单', sourceUrl: 'https://music.163.com/playlist?id=595975585' }
const manual = { id: 'netease:1', provider: 'netease', title: '人工导入', sourceUrl: 'https://music.163.com/playlist?id=1', syncStatus: 'ready' }
const track = { id: 'netease:33894312', provider: 'netease', title: '人工样本', author: '测试作者', sourceUrl: 'https://music.163.com/song?id=33894312', availability: 'unknown' }

async function render(t, { status = 'pending', manualTracks = [], targetTracks = [], query = '', platform = 'netease', tab = 'library', extraPlaylists = [], failLibrary = false } = {}) {
  const playlists = [{ ...target, syncStatus: status }, manual, ...extraPlaylists]
  t.mock.method(globalThis, 'fetch', async url => {
    if (url === '/api/v1/music/recommendations/today') return Response.json({ data: { date: '2026-09-30', timezone: 'Asia/Shanghai', status: 'ready', items: [] } })
    if (failLibrary) return new Response(null, { status: 503 })
    if (url === '/api/v1/music/playlists') return Response.json({ data: playlists })
    const tracks = url.includes(encodeURIComponent(target.id)) ? targetTracks : url.includes(encodeURIComponent(manual.id)) ? manualTracks : []
    return Response.json({ data: tracks, pagination: { total: tracks.length } })
  })
  const View = {
    ...MusicView,
    async setup(props, context) {
      const state = MusicView.setup(props, context)
      state.switchTab(tab)
      state.platform.value = platform
      state.query.value = query
      await state.loadLibrary()
      return state
    },
  }
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: View }, { path: '/blog', component: {} }] })
  await router.push('/')
  const html = await renderToString(createSSRApp(View).use(router))
  const input = html.match(/<input\b[^>]*type="search"[^>]*>/)?.[0]
  assert.ok(input, 'renders the actual search input')
  assert.doesNotMatch(input, /\bdisabled\b/)
  return html
}

test('pending target playlist does not block searching manually imported NetEase tracks', async t => {
  const html = await render(t, { manualTracks: [track], query: '人工样本' })
  assert.match(html, /01 TRACKS/)
  assert.match(html, /选择曲目：人工样本/)
  assert.match(html, /指定歌单 · 待同步/)
  assert.doesNotMatch(html, /人工导入 · 待同步/)
})

test('no search match shows search feedback alongside the scoped playlist status', async t => {
  const html = await render(t, { manualTracks: [track], query: '不存在的曲目' })
  assert.match(html, /00 TRACKS/)
  assert.match(html, /暂时没有找到这段旋律/)
  assert.match(html, /换一个歌名或作者试试/)
  assert.match(html, /指定歌单 · 待同步/)
})

test('pending, running, failed and ready match API status with or without retained tracks', async t => {
  for (const [status, label] of [['pending', '待同步'], ['running', '同步中'], ['failed', '同步失败'], ['ready', '']]) {
    for (const targetTracks of [[], [track]]) {
      await t.test(`${status}, ${targetTracks.length} retained tracks`, async t => {
        const html = await render(t, { status, targetTracks })
        assert.match(html, targetTracks.length ? /01 TRACKS/ : /00 TRACKS/)
        if (label) {
          assert.ok(html.includes(`指定歌单 · ${label}`))
          assert.ok(html.includes('选择此歌单'))
          if (targetTracks.length) assert.match(html, /已收录曲目仍可搜索/)
        } else {
          assert.doesNotMatch(html, /music-playlist-status/)
        }
        if (status === 'failed' || status === 'running') assert.doesNotMatch(html, /待同步/)
        if (targetTracks.length) assert.match(html, /选择曲目：人工样本/)
      })
    }
  }
})

test('unknown sync status is not silently classified as pending', async t => {
  const html = await render(t, { status: 'unexpected' })
  assert.match(html, /指定歌单 · 同步状态未知/)
  assert.doesNotMatch(html, /待同步/)
})

test('multiple playlists retain their own statuses and respect scene/provider filters', async t => {
  const extraPlaylists = [{ ...manual, id: 'netease:2', title: '另一个歌单', syncStatus: 'pending' }]
  const html = await render(t, { status: 'failed', extraPlaylists })
  assert.match(html, /指定歌单 · 同步失败/)
  assert.match(html, /另一个歌单 · 待同步/)
  for (const options of [{ platform: 'bilibili' }, { tab: 'favorites' }, { tab: 'discover' }]) {
    await t.test(JSON.stringify(options), async t => {
      assert.doesNotMatch(await render(t, { status: 'failed', ...options }), /music-playlist-status/)
    })
  }
  await t.test('all providers still show scoped status', async t => {
    assert.match(await render(t, { status: 'failed', platform: 'all' }), /指定歌单 · 同步失败/)
  })
})

test('library load failure is displayed as a load error rather than a sync state', async t => {
  const html = await render(t, { status: 'failed', failLibrary: true })
  assert.match(html, /音乐库暂时无法加载，请重试/)
  assert.doesNotMatch(html, /music-playlist-status/)
})
