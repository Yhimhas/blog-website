import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { articleSeo, routeSeo, seoHead, siteOrigin } from '../src/seo.ts'
import { loadPublicPosts, loadPublicSnapshot, seoAssets, seoPlugin } from './seo-build.ts'

const template = readFileSync(new URL('../index.html', import.meta.url), 'utf8')
const post = { slug: 'hello', title: '标题 " & </script><script>alert(1)</script>', summary: '摘要 <img src=x onerror=alert(1)>',
  contentMarkdown: '# 正文\n\n<script>alert(1)</script>\n\n[bad](javascript:alert(1))',
  publishedAt: '2026-10-01T00:00:00Z', updatedAt: '2026-10-02T00:00:00Z' }
const revision = { revision: '9007199254740993', sourceId: '11111111-2222-3333-4444-555555555555' }
const withRevision = fetcher => (url, options) => url.endsWith('/seo/revision') ? Promise.resolve(Response.json({ data: revision })) : fetcher(url, options)

test('initial article HTML contains escaped metadata, readable body and valid JSON-LD', () => {
  const assets = seoAssets(template, 'https://example.test', [post])
  const html = assets['blog/hello/index.html']
  assert.match(html, /rel="canonical" href="https:\/\/example.test\/blog\/hello"/)
  assert.match(html, /property="og:type" content="article"/)
  assert.match(html, /name="twitter:card" content="summary"/)
  assert.match(html, /<h1>正文<\/h1>/)
  assert.doesNotMatch(html, /<script>alert|href="javascript:/)
  assert.equal((html.match(/<title\b/g) || []).length, 1)
  const schema = JSON.parse(html.match(/type="application\/ld\+json">([\s\S]*?)<\/script>/)[1])
  assert.equal(schema.headline, post.title)
  assert.equal(schema.dateModified, post.updatedAt)
  assert.match(assets['blog/index.html'], /href="\/blog\/hello"/)
})

test('canonical excludes filters; public pages reset article tags and private pages are noindex', () => {
  const head = seoHead(routeSeo('/blog/?q=search#top'), 'https://example.test')
  assert.match(head, /rel="canonical" href="https:\/\/example.test\/blog"/)
  assert.doesNotMatch(head, /article:|BlogPosting|search/)
  for (const path of ['/admin', '/admin/local', '/login', '/missing']) {
    const html = seoHead(routeSeo(path), 'https://example.test')
    assert.match(html, /noindex, follow/)
    assert.doesNotMatch(html, /rel="canonical"/)
  }
  assert.equal(articleSeo(post).path, '/blog/hello')
  assert.equal(routeSeo('/blog/hello').noindex, false, 'loading article must not temporarily tell crawlers to deindex it')
})

test('sitemap contains only public URLs and actual article lastmod; preview blocks indexing', () => {
  const assets = seoAssets(template, 'https://example.test', [post])
  assert.equal((assets['sitemap.xml'].match(/<url>/g) || []).length, 5)
  assert.match(assets['sitemap.xml'], /<lastmod>2026-10-02T00:00:00Z<\/lastmod>/)
  assert.doesNotMatch(assets['sitemap.xml'], /admin|login|404/)
  assert.match(assets['404.html'], /noindex/)
  assert.match(assets['robots.txt'], /Sitemap: https:\/\/example.test\/sitemap.xml/)
  const preview = seoAssets(template, 'http://localhost:5173', [], true)
  assert.match(preview['index.html'], /noindex/)
  assert.doesNotMatch(preview['sitemap.xml'], /<url>/)
  assert.match(preview['robots.txt'], /Disallow: \//)
})

test('public API pagination and details are read without credentials; failures stop the build', async () => {
  const calls = []
  const fetcher = withRevision(async (url, options) => {
    calls.push(url)
    assert.equal(options.headers, undefined)
    assert.equal(options.redirect, 'error')
    assert.equal(options.cache, 'no-store')
    if (url.includes('/posts?')) return Response.json({ data: [{ slug: calls.length === 1 ? 'hello' : 'second' }], pagination: { total: 2 } })
    return Response.json({ data: { ...post, slug: url.endsWith('/second') ? 'second' : 'hello' } })
  })
  assert.equal((await loadPublicPosts('https://api.test', fetcher)).length, 2)
  assert.match(calls[2], /page=2/)
  await assert.rejects(loadPublicPosts('https://api.test', async () => new Response('', { status: 503 })), /503/)
  await assert.rejects(loadPublicPosts('https://api.test', withRevision(async () => Response.json({ data: [], pagination: { total: 1 } }))), /分页不完整/)
  await assert.rejects(loadPublicPosts('https://api.test', withRevision(async () => Response.json({ data: [{ slug: '../admin' }], pagination: { total: 1 } }))), /slug/)
})

test('snapshot rejects equal-count edits, replacement and archive/publish during reading', async () => {
  for (const changed of [{ ...revision, revision: '9007199254740994' }, { ...revision, sourceId: 'aaaaaaaa-2222-3333-4444-555555555555' }]) {
    let reads = 0
    const fetcher = async url => {
      if (url.endsWith('/seo/revision')) return Response.json({ data: ++reads === 1 ? revision : changed })
      if (url.includes('/posts?')) return Response.json({ data: [{ slug: post.slug }], pagination: { total: 1 } })
      return Response.json({ data: post })
    }
    await assert.rejects(loadPublicSnapshot('https://api.test', fetcher), /公开内容变化/)
  }
})

test('empty public set still checks the revision; malformed revisions fail closed', async () => {
  const snapshot = await loadPublicSnapshot('https://api.test', withRevision(async () => Response.json({ data: [], pagination: { total: 0 } })))
  assert.deepEqual(snapshot, { ...revision, posts: [] })
  for (const value of [1, '0', '-1', '1.1', '', null]) {
    await assert.rejects(loadPublicSnapshot('https://api.test', async () => Response.json({ data: { ...revision, revision: value } })), /版本格式无效/)
  }
})

test('build emits a complete receipt and rechecks after writing, before success', async () => {
  const previousFetch = globalThis.fetch
  let current = revision
  globalThis.fetch = async url => url.endsWith('/seo/revision') ? Response.json({ data: current }) :
    url.includes('/posts?') ? Response.json({ data: [{ slug: post.slug }], pagination: { total: 1 } }) : Response.json({ data: post })
  try {
    const plugin = seoPlugin({ VITE_SITE_URL: 'https://example.test', SEO_API_ORIGIN: 'http://localhost:8081' }, 'production')
    plugin.configResolved({ root: fileURLToPath(new URL('..', import.meta.url)), build: { outDir: 'dist/not-created-receipt-test' } })
    const emitted = []
    await plugin.generateBundle.call({ emitFile: asset => emitted.push(asset) }, {}, { 'index.html': { type: 'asset', source: template }, 'assets/app.js': { type: 'chunk' } })
    const receipt = JSON.parse(emitted.find(asset => asset.fileName === 'seo-release.json').source)
    assert.equal(receipt.revision, revision.revision)
    assert.equal(receipt.sourceId, revision.sourceId)
    assert.equal(receipt.preview, false)
    assert.ok(receipt.files.includes('blog/hello/index.html'))
    assert.ok(receipt.files.includes('assets/app.js'))
    await plugin.writeBundle()
    current = { ...revision, revision: '9007199254740994' }
    await assert.rejects(plugin.writeBundle(), /公开内容变化/)
  } finally {
    globalThis.fetch = previousFetch
  }
})

test('production configuration cannot silently publish localhost or reuse an existing release', () => {
  for (const value of ['javascript:alert(1)', 'https://a.test/path', 'https://a.test/?q=1', 'https://user:secret@a.test']) assert.throws(() => siteOrigin(value))
  const missing = seoPlugin({}, 'production')
  assert.throws(() => missing.configResolved({}), /VITE_SITE_URL/)
  const existing = seoPlugin({ VITE_SITE_URL: 'https://example.test', SEO_API_ORIGIN: 'http://localhost:8081' }, 'production')
  assert.throws(() => existing.configResolved({ root: fileURLToPath(new URL('..', import.meta.url)), build: { outDir: '.' } }), /新的空输出目录/)
})
