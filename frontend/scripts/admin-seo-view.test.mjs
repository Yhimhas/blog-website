import assert from 'node:assert/strict'
import { after, test } from 'node:test'
import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'
import { createServer } from 'vite'
import vue from '@vitejs/plugin-vue'

const server = await createServer({ configFile: false, plugins: [vue()], optimizeDeps: { noDiscovery: true, include: [] }, server: { middlewareMode: true }, appType: 'custom' })
after(() => server.close())
const { default: Panel } = await server.ssrLoadModule('/src/components/SeoPublicationStatus.vue')
const { default: AdminView } = await server.ssrLoadModule('/src/views/AdminView.vue')
const receipt = { sourceId: '11111111-2222-3333-4444-555555555555', revision: '2', appliedRevision: '2', lastAttemptAt: null, publishedAt: '2026-10-03T00:00:00Z', lastError: '' }

test('actual status panel renders pending, failure, success and unconfirmed states', async t => {
  for (const [status, label] of [['pending', '静态页面尚未更新'], ['failed', '上次静态页面更新失败'], ['current', '静态页面已更新'], ['unknown', '无法确认静态页面是否更新'], ['checking', '正在检查静态页面状态']]) {
    await t.test(status, async () => {
      const html = await renderToString(createSSRApp(Panel, { state: { status, data: ['pending', 'failed', 'current'].includes(status) ? receipt : undefined }, loading: false }))
      assert.ok(html.includes(label), html)
      assert.match(html, /刷新静态状态/)
      if (status === 'pending' || status === 'failed') assert.match(html, /归档文章的旧页面仍可能公开返回/)
      if (status === 'pending') assert.match(html, /发布任务尚无执行记录/)
      if (status === 'failed') assert.match(html, /role="alert"/)
    })
  }
})

test('actual administrator publish/archive notices survive a failed status read and never claim immediate static access', async t => {
  for (const action of ['publish', 'archive']) {
    for (const statusReadFails of [false, true]) {
      await t.test(`${action}, status ${statusReadFails ? 'unavailable' : 'pending'}`, async t => {
        let changed = false
        let post = { id: 'post', slug: 'seo-loop', title: 'Article', summary: '', contentMarkdown: '# body', categoryId: null, tagIds: [], status: 'published', version: 2, publishedAt: '2026-10-03T00:00:00Z', createdAt: '2026-10-03T00:00:00Z', updatedAt: '2026-10-03T00:00:00Z', revision: null }
        const previousWindow = globalThis.window
        globalThis.window = { confirm: () => true }
        t.after(() => { globalThis.window = previousWindow })
        t.mock.method(globalThis, 'fetch', async (url, options) => {
          if (url === `/api/v1/admin/posts/post/${action}`) {
            assert.equal(options.method, 'POST')
            changed = true
            post = { ...post, version: 3, status: action === 'archive' ? 'archived' : 'published' }
            return Response.json({ data: post })
          }
          if (url === '/api/v1/admin/seo') {
            if (changed && statusReadFails) return Response.json({ error: { code: 'NOT_READY' } }, { status: 503 })
            return Response.json({ data: { ...receipt, revision: changed ? '3' : '2' } })
          }
          if (url.startsWith('/api/v1/admin/posts?')) return Response.json({ data: [post], pagination: { page: 1, pageSize: 20, total: 1 } })
          throw new Error(`unexpected request: ${url}`)
        })
        let state
        const View = { ...AdminView, async setup(props, context) {
          state = AdminView.setup(props, context)
          state.checking.value = false
          state.authenticated.value = true
          await state.staticPages.start()
          state.fill(post)
          if (action === 'publish') await state.save(true)
          else await state.archive()
          await state.staticPages.refresh()
          return state
        } }
        const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/admin', component: View }, { path: '/blog', component: {} }, { path: '/blog/:slug', component: {} }, { path: '/admin/local', component: {} }] })
        await router.push('/admin')
        try {
          const html = await renderToString(createSSRApp({ render: () => h(RouterView) }).use(router))
          assert.equal(changed, true)
          assert.match(html, statusReadFails ? /无法确认静态页面是否更新/ : /静态页面尚未更新/)
          assert.match(html, action === 'publish' ? /文章已发布到数据库/ : /文章已归档，公开 API 已隐藏/)
          assert.doesNotMatch(html, /发布成功，可以通过下方链接公开访问|文章已归档，已停止公开访问|归档后无法公开访问/)
          assert.equal(state.error.value, '', 'a failed status read must not turn a successful article operation into an error')
        } finally { state?.staticPages.stop() }
      })
    }
  }
})
