import assert from 'node:assert/strict'
import { test } from 'node:test'
import { blogApi, ApiError } from '../src/blogApi.ts'
import { renderMarkdown } from '../src/markdown.ts'

test('login → CSRF session → create → edit → publish → public slug uses API contract', async (t) => {
  const input = { slug: 'hello-go', title: '你好 Go', summary: '摘要', contentMarkdown: '# 初稿', categoryId: null, tagIds: [] }
  const draft = { ...input, id: 'database-id', version: 1, status: 'draft' }
  const edited = { ...draft, version: 2, contentMarkdown: '# 发布内容' }
  const published = { ...edited, version: 3, status: 'published' }
  const calls = [
    ['/admin/session', 'POST', { username: 'owner', password: 'test-only-password' }, { user: { username: 'owner' } }],
    ['/admin/session', 'GET', undefined, { user: { username: 'owner' }, csrfToken: 'csrf-test' }],
    ['/admin/posts', 'POST', input, draft],
    ['/admin/posts/database-id', 'PATCH', { title: input.title, summary: input.summary, contentMarkdown: edited.contentMarkdown, categoryId: null, tagIds: [], version: 1 }, edited],
    ['/admin/posts/database-id/publish', 'POST', { version: 2 }, published],
    ['/posts/hello-go', 'GET', undefined, { slug: input.slug, title: input.title, contentMarkdown: edited.contentMarkdown }],
    ['/admin/session/logout', 'POST', {}, undefined],
  ]
  let index = 0
  t.mock.method(globalThis, 'fetch', async (url, options) => {
    const [path, method, body, data] = calls[index]
    assert.equal(url, `/api/v1${path}`)
    assert.equal(options.method, method)
    assert.equal(options.credentials, 'same-origin')
    assert.equal(options.cache, 'no-store')
    assert.deepEqual(options.body === undefined ? undefined : JSON.parse(options.body), body)
    if (index >= 2 && method !== 'GET') assert.equal(options.headers['X-CSRF-Token'], 'csrf-test')
    if (index === 0) assert.equal(options.headers['X-CSRF-Token'], undefined)
    index++
    return data === undefined ? new Response(null, { status: 204 }) : Response.json({ data })
  })
  await blogApi.login('owner', 'test-only-password')
  const created = await blogApi.create(input)
  const { slug, ...patch } = { ...input, contentMarkdown: edited.contentMarkdown }
  const updated = await blogApi.update(created.id, created.version, patch)
  const result = await blogApi.publish(updated.id, updated.version)
  assert.equal(result.status, 'published')
  assert.equal((await blogApi.post(result.slug)).contentMarkdown, edited.contentMarkdown)
  await blogApi.logout()
  assert.equal(index, calls.length)
})

test('public filters, pagination and URL encoding preserve server semantics', async (t) => {
  const query = new URLSearchParams({ q: '中文 & Go', category: 'dev', tag: 'go', page: '2', pageSize: '20' })
  const mock = t.mock.method(globalThis, 'fetch', async (url) => {
    assert.equal(url, `/api/v1/posts?${query}`)
    return Response.json({ data: [], pagination: { page: 2, pageSize: 20, total: 21 } })
  })
  assert.equal((await blogApi.posts(query)).pagination.total, 21)
  mock.mock.mockImplementation(async url => {
    assert.equal(url, '/api/v1/posts/a%2Fb%3Fc')
    return Response.json({ error: { code: 'POST_NOT_FOUND', message: '资源不存在' } }, { status: 404 })
  })
  await assert.rejects(blogApi.post('a/b?c'), e => e instanceof ApiError && e.status === 404)
})

test('conflict, expired session, gateway and invalid JSON remain errors without fallback', async (t) => {
  const mock = t.mock.method(globalThis, 'fetch', async () => Response.json({ error: { code: 'CONFLICT', message: '版本冲突' } }, { status: 409 }))
  await assert.rejects(blogApi.publish('id', 1), e => e.status === 409 && e.code === 'CONFLICT')
  mock.mock.mockImplementation(async () => Response.json({ error: { code: 'UNAUTHORIZED' } }, { status: 401 }))
  await assert.rejects(blogApi.session(), e => e.status === 401)
  mock.mock.mockImplementation(async () => new Response('bad gateway', { status: 502 }))
  await assert.rejects(blogApi.post('slug'), e => e.status === 502)
  for (const invalid of [null, {}, 'text']) {
    mock.mock.mockImplementation(async () => Response.json(invalid))
    await assert.rejects(blogApi.post('slug'), e => e.code === 'INVALID_RESPONSE')
  }
  mock.mock.mockImplementation(async () => { throw new TypeError('offline') })
  await assert.rejects(blogApi.post('slug'), e => e.code === 'NETWORK_ERROR')
})

test('navigation cancellation aborts stale requests', async (t) => {
  const controller = new AbortController()
  controller.abort()
  t.mock.method(globalThis, 'fetch', async (_, options) => {
    assert.equal(options.signal.aborted, true)
    throw new DOMException('Aborted', 'AbortError')
  })
  await assert.rejects(blogApi.post('slug', controller.signal), e => e.name === 'AbortError')
})

test('public rendering and preview escape HTML and reject executable links', () => {
  const html = renderMarkdown('# 标题\n\n<script>alert(1)</script>\n\n[x](javascript:alert(1))\n\n![x](data:text/html;base64,WA==)')
  assert.match(html, /<h1>标题<\/h1>/)
  assert.doesNotMatch(html, /<script|href="javascript:|src="data:text\/html/)
  assert.match(html, /&lt;script&gt;/)
})
