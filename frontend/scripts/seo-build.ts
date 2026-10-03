import type { Plugin } from 'vite'
import { existsSync, readdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { articleSeo, escapeHtml, publicPages, routeSeo, seoHead, siteOrigin, type SeoPage, type SeoArticle } from '../src/seo.ts'
import { renderMarkdown } from '../src/markdown.ts'

export interface PublicRevision { revision: string; sourceId: string }

async function request<T>(origin: string, path: string, fetcher: typeof fetch): Promise<T> {
  const response = await fetcher(`${origin}/api/v1${path}`, { signal: AbortSignal.timeout(15000), redirect: 'error', cache: 'no-store' })
  if (!response.ok) throw new Error(`SEO 公开 API 请求失败 (${response.status}): ${path}`)
  return await response.json() as T
}

export async function loadPublicRevision(origin: string, fetcher: typeof fetch = fetch): Promise<PublicRevision> {
  const { data } = await request<{ data: PublicRevision }>(origin, '/seo/revision', fetcher)
  if (!data || typeof data.revision !== 'string' || !/^[1-9][0-9]*$/.test(data.revision) ||
    typeof data.sourceId !== 'string' || !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(data.sourceId)) throw new Error('SEO 公开版本格式无效')
  return data
}

function sameRevision(a: PublicRevision, b: PublicRevision) {
  if (a.revision !== b.revision || a.sourceId !== b.sourceId) throw new Error('构建期间公开内容变化，请重新构建')
}

export async function loadPublicSnapshot(origin: string, fetcher: typeof fetch = fetch): Promise<PublicRevision & { posts: SeoArticle[] }> {
  const revision = await loadPublicRevision(origin, fetcher)
  const posts: SeoArticle[] = []
  const seen = new Set<string>()
  let total: number | undefined
  for (let page = 1; ; page++) {
    const result = await request<{ data: { slug: string }[]; pagination: { total: number } }>(origin, `/posts?page=${page}&pageSize=50`, fetcher)
    if (!Array.isArray(result.data) || !Number.isSafeInteger(result.pagination?.total) || result.pagination.total < 0) throw new Error('SEO 文章列表格式无效')
    if (total !== undefined && total !== result.pagination.total) throw new Error('构建期间文章列表变化，请重新构建')
    total = result.pagination.total as number
    if (total > 49000) throw new Error('文章超过单个 sitemap 的容量，请先实现 sitemap index')
    for (const item of result.data) {
      if (typeof item.slug !== 'string' || !/^[a-z0-9]+(-[a-z0-9]+)*$/.test(item.slug) || seen.has(item.slug)) throw new Error('SEO 文章 slug 无效或重复')
      seen.add(item.slug)
      const { data } = await request<{ data: SeoArticle }>(origin, `/posts/${encodeURIComponent(item.slug)}`, fetcher)
      if (!data || data.slug !== item.slug || !(['title', 'summary', 'contentMarkdown'] as const).every(key => typeof data[key] === 'string') ||
        !(['publishedAt', 'updatedAt'] as const).every(key => typeof data[key] === 'string' && Number.isFinite(Date.parse(data[key])))) throw new Error('SEO 文章详情格式无效')
      posts.push(data)
    }
    if (posts.length === total) {
      sameRevision(revision, await loadPublicRevision(origin, fetcher))
      return { ...revision, posts }
    }
    if (!result.data.length || posts.length > total) throw new Error('SEO 文章分页不完整')
  }
}

export async function loadPublicPosts(origin: string, fetcher: typeof fetch = fetch): Promise<SeoArticle[]> {
  return (await loadPublicSnapshot(origin, fetcher)).posts
}

export function renderPage(template: string, page: SeoPage, origin: string, body: string) {
  if (!template.includes('<!-- seo:start -->') || !template.includes('<div id="app"></div>')) throw new Error('SEO HTML 模板缺少挂载点或标记')
  return template.replace(/<!-- seo:start -->[\s\S]*?<!-- seo:end -->/, () => `<!-- seo:start -->\n${seoHead(page, origin)}\n<!-- seo:end -->`)
    .replace('<div id="app"></div>', () => `<div id="app">${body}</div>`)
}

export function seoAssets(template: string, origin: string, posts: SeoArticle[], preview = false): Record<string, string> {
  const assets: Record<string, string> = {}
  const nav = `<nav>${Object.entries(publicPages).map(([path, page]) => `<a href="${path}">${page.title}</a>`).join(' · ')}</nav>`
  const links = `<ul>${posts.map(post => `<li><a href="/blog/${post.slug}">${escapeHtml(post.title)}</a><p>${escapeHtml(post.summary)}</p></li>`).join('')}</ul>`
  const urls: { loc: string; lastmod?: string }[] = []
  for (const path of Object.keys(publicPages)) {
    const page = { ...routeSeo(path), noindex: preview }
    assets[path === '/' ? 'index.html' : `${path.slice(1)}/index.html`] = renderPage(template, page, origin,
      `${nav}<main><h1>${escapeHtml(page.title)}</h1><p>${escapeHtml(page.description)}</p>${path === '/blog' || path === '/' ? links : ''}</main>`)
    urls.push({ loc: origin + path })
  }
  for (const post of posts) {
    const page = { ...articleSeo(post), noindex: preview }
    assets[`blog/${post.slug}/index.html`] = renderPage(template, page, origin,
      `${nav}<main><article><h1>${escapeHtml(post.title)}</h1><p>${escapeHtml(page.description)}</p><time datetime="${escapeHtml(post.publishedAt)}">${escapeHtml(post.publishedAt)}</time>${renderMarkdown(post.contentMarkdown)}</article></main>`)
    urls.push({ loc: origin + page.path, lastmod: post.updatedAt })
  }
  for (const path of ['/login', '/admin', '/admin/local', '/404']) {
    const page = routeSeo(path)
    assets[path === '/404' ? '404.html' : `${path.slice(1)}/index.html`] = renderPage(template, page, origin, `${nav}<main><h1>${escapeHtml(page.title)}</h1></main>`)
  }
  assets['sitemap.xml'] = `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${(preview ? [] : urls).map(url => `<url><loc>${escapeHtml(url.loc)}</loc>${url.lastmod ? `<lastmod>${escapeHtml(url.lastmod)}</lastmod>` : ''}</url>`).join('')}</urlset>`
  assets['robots.txt'] = preview ? 'User-agent: *\nDisallow: /\n' : `User-agent: *\nAllow: /\nDisallow: /api/\nSitemap: ${origin}/sitemap.xml\n`
  return assets
}

export function seoPlugin(env: Record<string, string>, mode: string): Plugin {
  let origin = ''
  let preview = false
  let snapshot: PublicRevision | undefined
  return {
    name: 'public-page-seo', enforce: 'post', apply: 'build',
    configResolved(config) {
      preview = mode !== 'production'
      if (!preview && (!env.VITE_SITE_URL || !env.SEO_API_ORIGIN)) throw new Error('正式 SEO 构建需要 VITE_SITE_URL 和 SEO_API_ORIGIN；本地预览使用 --mode development')
      origin = siteOrigin(env.VITE_SITE_URL || 'http://localhost:5173')
      // Reusing a release directory could retain HTML for an archived article.
      const output = resolve(config.root, config.build.outDir)
      if (!preview && existsSync(output) && readdirSync(output).length) throw new Error('SEO 发布必须使用新的空输出目录（--outDir）；保留旧目录，不自动删除文件')
    },
    async generateBundle(_, bundle) {
      const index = bundle['index.html']
      if (!index || index.type !== 'asset') throw new Error('缺少 Vite index.html 构建产物')
      const content = env.SEO_API_ORIGIN ? await loadPublicSnapshot(siteOrigin(env.SEO_API_ORIGIN)) : undefined
      snapshot = content
      const assets = seoAssets(String(index.source), origin, content?.posts || [], preview)
      const files = [...new Set([...Object.keys(bundle), ...Object.keys(assets), 'yhimhas-logo.jpg'])].sort()
      index.source = assets['index.html']!
      for (const [fileName, source] of Object.entries(assets)) {
        if (fileName !== 'index.html') this.emitFile({ type: 'asset', fileName, source })
      }
      this.emitFile({ type: 'asset', fileName: 'seo-release.json', source: JSON.stringify({
        revision: content?.revision || '0', sourceId: content?.sourceId || '', origin, preview, files,
      }) })
    },
    async writeBundle() {
      if (snapshot && env.SEO_API_ORIGIN) sameRevision(snapshot, await loadPublicRevision(siteOrigin(env.SEO_API_ORIGIN)))
    },
  }
}
