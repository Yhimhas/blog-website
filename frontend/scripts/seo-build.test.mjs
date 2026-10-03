import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createServer } from 'node:http'
import { spawn } from 'node:child_process'
import { mkdir, mkdtemp, readFile, stat } from 'node:fs/promises'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontend = fileURLToPath(new URL('..', import.meta.url))
const origin = 'https://example.test'
const sourceId = '11111111-2222-3333-4444-555555555555'

test('production outputs follow publication, update and archive; a late mutation rejects the build', { timeout: 180000 }, async t => {
  let revision = 1
  let posts = [{ slug: 'refresh-test', title: 'Published title', summary: 'summary', contentMarkdown: '# Published body',
    publishedAt: '2026-10-03T01:00:00Z', updatedAt: '2026-10-03T01:00:00Z' }]
  let revisionReads = 0
  let mutateOnWrite = false
  const server = createServer((request, response) => {
    response.setHeader('Content-Type', 'application/json')
    response.setHeader('Cache-Control', 'no-store')
    if (request.url === '/api/v1/seo/revision') {
      if (mutateOnWrite && ++revisionReads === 3) revision++
      response.end(JSON.stringify({ data: { revision: String(revision), sourceId } }))
    } else if (request.url.startsWith('/api/v1/posts?')) {
      response.end(JSON.stringify({ data: posts.map(post => ({ slug: post.slug })), pagination: { total: posts.length } }))
    } else if (request.url === '/api/v1/posts/refresh-test' && posts.length) {
      response.end(JSON.stringify({ data: posts[0] }))
    } else {
      response.statusCode = 404
      response.end('{}')
    }
  })
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve) })
  t.after(() => new Promise(resolve => { server.closeAllConnections(); server.close(resolve) }))
  const children = new Set()
  t.after(() => { for (const child of children) child.kill() })
  await mkdir(join(frontend, 'dist'), { recursive: true })
  const root = await mkdtemp(join(frontend, 'dist/seo-refresh-validation-'))
  t.diagnostic(`Retained build validation outputs: ${root}`)
  const apiOrigin = `http://127.0.0.1:${server.address().port}`
  async function build(name, expected = 0) {
    const directory = join(root, name)
    const child = spawn(process.execPath, ['node_modules/vite/bin/vite.js', 'build', '--mode', 'production', '--configLoader', 'runner', '--outDir', directory], {
      cwd: frontend, env: { ...process.env, VITE_SITE_URL: origin, SEO_API_ORIGIN: apiOrigin }, stdio: ['ignore', 'pipe', 'pipe'],
    })
    children.add(child)
    let output = ''
    child.stdout.on('data', data => { output += data })
    child.stderr.on('data', data => { output += data })
    const code = await new Promise((resolve, reject) => { child.once('error', reject); child.once('close', code => { children.delete(child); resolve(code) }) })
    assert.equal(code, expected, output)
    return { directory, output }
  }
  const published = await build('published')
  assert.match(await readFile(join(published.directory, 'blog/refresh-test/index.html'), 'utf8'), /Published body/)
  revision++
  posts[0] = { ...posts[0], title: 'Updated title', contentMarkdown: '# Updated body', updatedAt: '2026-10-03T02:00:00Z' }
  const updated = await build('updated')
  const html = await readFile(join(updated.directory, 'blog/refresh-test/index.html'), 'utf8')
  assert.match(html, /Updated title/)
  assert.match(html, /Updated body/)
  const manifest = JSON.parse(await readFile(join(updated.directory, 'seo-release.json'), 'utf8'))
  assert.equal(manifest.revision, '2')
  assert.equal(manifest.sourceId, sourceId)
  for (const file of manifest.files) assert.ok((await stat(join(updated.directory, file))).size > 0, file)
  revision++
  posts = []
  const archived = await build('archived')
  await assert.rejects(stat(join(archived.directory, 'blog/refresh-test/index.html')), { code: 'ENOENT' })
  assert.doesNotMatch(await readFile(join(archived.directory, 'sitemap.xml'), 'utf8'), /refresh-test/)
  assert.match(await readFile(join(published.directory, 'blog/refresh-test/index.html'), 'utf8'), /Published body/, 'old release must be retained')
  mutateOnWrite = true
  revisionReads = 0
  const stale = await build('stale', 1)
  assert.match(stale.output, /公开内容变化/)
})
