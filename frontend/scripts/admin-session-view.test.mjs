import assert from 'node:assert/strict'
import { after, test } from 'node:test'
import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'
import { createServer } from 'vite'
import vue from '@vitejs/plugin-vue'

// Exercise the real SFC and session/CSRF flow without writing build artifacts.
const server = await createServer({ configFile: false, plugins: [vue()], optimizeDeps: { noDiscovery: true, include: [] }, server: { middlewareMode: true, ws: false }, appType: 'custom' })
after(() => server.close())
const { default: AdminView } = await server.ssrLoadModule('/src/views/AdminView.vue')
const { authSession } = await server.ssrLoadModule('/src/authSession.ts')
const receipt = { sourceId: '11111111-2222-3333-4444-555555555555', revision: '2', appliedRevision: '2', lastAttemptAt: null, publishedAt: null, lastError: '' }
const owner = { id: 'owner', username: 'owner', role: 'admin' }
const category = { id: 'cat', name: '开发', slug: 'dev' }
const tag = { id: 'tag', name: 'Vue', slug: 'vue' }
const post = { id: 'post', slug: 'saved-post', title: '原标题', summary: '原摘要', contentMarkdown: '# 原正文', categoryId: 'cat', tagIds: ['tag'], status: 'draft', version: 2, revision: null, publishedAt: null, createdAt: '2026-10-03T00:00:00Z', updatedAt: '2026-10-03T00:00:00Z' }
const failure = (status, code, message = code) => Response.json({ error: { code, message } }, { status })

async function renderScenario(t, scenario) {
  const previousWindow = globalThis.window
  globalThis.window = { confirm: () => false }
  authSession.user.value = owner
  t.after(() => { globalThis.window = previousWindow; authSession.user.value = null })
  let state
  const View = { ...AdminView, async setup(props, context) {
    state = AdminView.setup(props, context)
    state.checking.value = false
    state.authenticated.value = true
    state.categories.value = [category]
    state.tags.value = [tag]
    await scenario(state)
    return state
  } }
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/admin', component: View }, { path: '/blog', component: {} }, { path: '/blog/:slug', component: {} }, { path: '/admin/local', component: {} }, { path: '/login', component: {} }] })
  await router.push('/admin')
  try {
    const html = await renderToString(createSSRApp({ render: () => h(RouterView) }).use(router))
    assert.equal(router.currentRoute.value.path, '/admin', 'recovery must stay on the editor route')
    return { html, state }
  } finally { state?.staticPages.stop() }
}

function editBoth(state) {
  state.fill(structuredClone(post))
  Object.assign(state.form, { title: '未保存标题', summary: '未保存摘要', contentMarkdown: '# 未保存正文\n\n继续写作' })
  state.editTerm(category)
  Object.assign(state.termForm, { name: '未保存分类', slug: 'unsaved-category' })
}

function assertLeaveProtection(state) {
  assert.equal(state.allowedToLeave(), false)
  let prevented = false
  const event = { preventDefault() { prevented = true }, returnValue: undefined }
  state.beforeUnload(event)
  assert.equal(prevented, true)
  assert.equal(event.returnValue, '')
}

test('expiry keeps article and term inputs visible, editable and exportable while blocking server writes', async t => {
  for (const [status, code, editor] of [[401, 'UNAUTHORIZED', 'both'], [403, 'CSRF_FAILED', 'article'], [403, 'ADMIN_REQUIRED', 'term']]) {
    await t.test(`${code}, ${editor}`, async t => {
      const writes = []
      t.mock.method(globalThis, 'fetch', async (url, options) => {
        if (url === '/api/v1/admin/seo') return Response.json({ data: receipt })
        writes.push([url, options.method])
        return failure(status, code)
      })
      const { html, state } = await renderScenario(t, async state => {
        if (editor !== 'term') {
          state.fill(structuredClone(post))
          state.form.title = '未保存标题'
          state.form.summary = '未保存摘要'
          state.form.contentMarkdown = '# 未保存正文'
        }
        if (editor !== 'article') {
          state.editTerm(category)
          state.termForm.name = '未保存分类'
        }
        if (editor === 'term') await state.saveTerm()
        else await state.save()
        assert.equal(state.authenticated.value, false)
        assert.equal(authSession.user.value, null)
        assert.equal(state.username.value, owner.username)
        // Even a form submit bypassing the disabled button must not hit the API.
        await state.save()
        await state.saveTerm()
        assert.equal(writes.length, 1)
        if (editor !== 'term') state.form.contentMarkdown += '\n\n过期后继续编辑'
        assertLeaveProtection(state)
      })
      assert.match(html, /在本页重新登录/)
      assert.match(html, /导出当前编辑内容（JSON）/)
      assert.doesNotMatch(html, /前往用户登录|管理文章列表/)
      assert.doesNotMatch(html, /<fieldset[^>]*\bdisabled\b/)
      if (editor !== 'term') {
        assert.match(html, /value="未保存标题"/)
        assert.match(html, /未保存摘要/)
        assert.match(html, /过期后继续编辑/)
        assert.match(html, /<button type="submit" disabled[^>]*>保存文章/)
        assert.match(html, /<button type="button" disabled[^>]*>确认发布/)
        assert.equal(state.dirty.value, true)
      }
      if (editor !== 'article') {
        assert.match(html, /<details[^>]*class="term-manager"[^>]*open/)
        assert.match(html, /value="未保存分类"/)
        assert.match(html, /<button type="submit" disabled[^>]*>保存分类/)
        assert.equal(state.termDirty.value, true)
      }
    })
  }
})

test('failed credentials, network errors and non-admin login preserve both editors and clear the password', async t => {
  for (const reason of ['credentials', 'network', 'role']) {
    await t.test(reason, async t => {
      let adminReads = 0
      t.mock.method(globalThis, 'fetch', async (url, options) => {
        if (url === '/api/v1/admin/seo') return Response.json({ data: receipt })
        if (url === '/api/v1/session' && options.method === 'POST') {
          if (reason === 'network') throw new TypeError('offline')
          if (reason === 'credentials') return failure(401, 'INVALID_CREDENTIALS', '用户名或密码错误')
          return Response.json({ data: {} })
        }
        if (url === '/api/v1/session') return Response.json({ data: { user: { ...owner, role: 'user' }, csrfToken: 'user-csrf' } })
        adminReads++
        return failure(401, 'UNAUTHORIZED')
      })
      const { html } = await renderScenario(t, async state => {
        editBoth(state)
        await state.save()
        const original = [JSON.stringify(state.form), state.snapshot.value, JSON.stringify(state.termForm), state.termSnapshot.value]
        state.password.value = 'test-only-password'
        await state.login()
        assert.equal(state.password.value, '')
        assert.equal(state.authenticated.value, false)
        assert.equal(state.busy.value, false)
        assert.deepEqual([JSON.stringify(state.form), state.snapshot.value, JSON.stringify(state.termForm), state.termSnapshot.value], original)
        assertLeaveProtection(state)
      })
      assert.equal(adminReads, 1, 'non-admin/failed login must not load the workspace')
      assert.match(html, reason === 'credentials' ? /用户名或密码错误/ : reason === 'network' ? /无法连接博客服务/ : /当前账号没有文章管理权限/)
      assert.match(html, /value="未保存标题"/)
      assert.match(html, /value="未保存分类"/)
      assert.doesNotMatch(html, /test-only-password/)
    })
  }
})

test('inline login refreshes CSRF and lists without replacing drafts, resetting versions or auto-saving', async t => {
  let loggedIn = false
  const writes = []
  t.mock.method(globalThis, 'fetch', async (url, options) => {
    if (url === '/api/v1/admin/seo') return Response.json({ data: receipt })
    if (url === '/api/v1/session' && options.method === 'POST') {
      assert.deepEqual(JSON.parse(options.body), { username: 'owner', password: 'test-only-password' })
      assert.equal(options.headers['X-CSRF-Token'], undefined)
      loggedIn = true
      return Response.json({ data: {} })
    }
    if (url === '/api/v1/session') return Response.json({ data: { user: owner, csrfToken: 'fresh-csrf' } })
    if (url === '/api/v1/categories') return Response.json({ data: [{ ...category, name: '服务器上的分类' }] })
    if (url === '/api/v1/tags') return Response.json({ data: [tag] })
    if (url.startsWith('/api/v1/admin/posts?')) return Response.json({ data: [{ ...post, version: 99, title: '其他会话已更新' }], pagination: { page: 1, pageSize: 20, total: 1 } })
    assert.equal(url, '/api/v1/admin/posts/post')
    assert.equal(options.method, 'PATCH')
    writes.push(JSON.parse(options.body))
    if (!loggedIn) return failure(401, 'UNAUTHORIZED')
    assert.equal(options.headers['X-CSRF-Token'], 'fresh-csrf')
    return failure(409, 'CONFLICT')
  })
  const { html } = await renderScenario(t, async state => {
    editBoth(state)
    await state.save()
    const original = [JSON.stringify(state.form), state.snapshot.value, JSON.stringify(state.termForm), state.termSnapshot.value]
    state.password.value = 'test-only-password'
    await state.login()
    assert.equal(state.authenticated.value, true)
    assert.equal(state.password.value, '')
    assert.equal(writes.length, 1, 'login must not save or publish automatically')
    assert.equal(state.posts.value[0].version, 99, 'list actually refreshed')
    assert.equal(state.selected.value.version, 2, 'retain the edit base for optimistic concurrency')
    assert.deepEqual([JSON.stringify(state.form), state.snapshot.value, JSON.stringify(state.termForm), state.termSnapshot.value], original)
    assert.equal(state.dirty.value, true)
    assert.equal(state.termDirty.value, true)
    assert.match(state.notice.value, /已重新登录.*手动保存/)
    await state.save()
    assert.equal(writes[1].version, 2)
    assert.equal(writes[1].contentMarkdown, '# 未保存正文\n\n继续写作')
    assert.equal(state.conflict.value, true)
    assertLeaveProtection(state)
  })
  assert.match(html, /文章已被其他会话修改/)
  assert.match(html, /value="未保存标题"/)
  assert.match(html, /value="未保存分类"/)
  assert.doesNotMatch(html, /在本页重新登录/)
})

test('backup downloads all incomplete draft and term fields without credentials or changing unsaved guards', async t => {
  let blob
  let download
  let cleanup
  const revoked = []
  const previousDocument = globalThis.document
  globalThis.document = { createElement(tagName) {
    assert.equal(tagName, 'a')
    return { click() { download = { href: this.href, name: this.download } } }
  } }
  t.after(() => { globalThis.document = previousDocument })
  t.mock.method(URL, 'createObjectURL', data => { blob = data; return 'blob:backup' })
  t.mock.method(URL, 'revokeObjectURL', url => { revoked.push(url) })
  t.mock.method(globalThis, 'fetch', async () => Response.json({ data: receipt }))
  await renderScenario(t, async state => {
    state.fill()
    Object.assign(state.form, { slug: '', title: '', summary: '未完成摘要', contentMarkdown: '未完成正文', categoryId: 'cat', tagIds: ['tag'] })
    state.editTerm(undefined, 'tags')
    Object.assign(state.termForm, { name: '未完成标签', slug: '' })
    state.authenticated.value = false
    state.password.value = 'test-only-password'
    globalThis.window.setTimeout = (callback, delay) => { assert.equal(delay, 1000); cleanup = callback }
    state.exportBackup()
    const backup = JSON.parse(await blob.text())
    assert.deepEqual(backup.article, { id: null, version: null, ...state.form })
    assert.deepEqual(backup.term, { kind: 'tags', id: null, ...state.termForm })
    assert.deepEqual(backup.categories, [category])
    assert.deepEqual(backup.tags, [tag])
    assert.doesNotMatch(await blob.text(), /test-only-password|csrfToken/)
    assert.equal(blob.type, 'application/json;charset=utf-8')
    assert.equal(download.href, 'blob:backup')
    assert.match(download.name, /^admin-edit-backup-.*\.json$/)
    cleanup()
    assert.deepEqual(revoked, ['blob:backup'])
    assert.equal(state.dirty.value, true)
    assert.equal(state.termDirty.value, true)
    assertLeaveProtection(state)
    assert.match(state.notice.value, /请确认文件已保存/)
    t.mock.method(URL, 'createObjectURL', () => { throw new Error('download blocked') })
    state.exportBackup()
    assert.match(state.error.value, /无法导出备份.*复制/)
    assert.equal(state.form.contentMarkdown, '未完成正文')
    assert.equal(state.termForm.name, '未完成标签')
    assertLeaveProtection(state)
  })
})
