export interface SeoArticle {
  slug: string; title: string; summary: string; contentMarkdown: string
  publishedAt: string; updatedAt: string
}

export const siteName = 'Yhimhas / NOTES'
export const siteDescription = 'Yhimhas 的数字自留地，记录开发笔记、生活随记与音乐灵感。'
export const publicPages: Record<string, { title: string; description: string }> = {
  '/': { title: '首页', description: siteDescription },
  '/blog': { title: '博客', description: '阅读 Yhimhas 的开发笔记与生活随记，记录探索过程与日常灵感。' },
  '/music': { title: '音乐', description: 'Yhimhas 的音乐空间，收藏喜欢的音乐，发现每日推荐。' },
  '/about': { title: '关于', description: '关于 Yhimhas，以及这个记录、探索和保持好奇的个人网站。' },
}
export interface SeoPage {
  title: string; description: string; path: string; noindex?: boolean; post?: SeoArticle
}
export function routeSeo(path: string): SeoPage {
  const clean = path.split(/[?#]/)[0]!.replace(/\/+$/, '') || '/'
  const page = publicPages[clean]
  const privateTitles: Record<string, string> = { '/login': '用户登录', '/admin': '文章管理', '/admin/local': '本地草稿' }
  return { ...(page || { title: privateTitles[clean] || (clean.startsWith('/blog/') ? '文章' : '页面不存在'), description: siteDescription }),
    path: clean, noindex: !page && !/^\/blog\/[a-z0-9]+(-[a-z0-9]+)*$/.test(clean) }
}
export function articleSeo(post: SeoArticle): SeoPage {
  return { title: post.title, description: post.summary.trim() || siteDescription,
    path: `/blog/${encodeURIComponent(post.slug)}`, post }
}
export function siteOrigin(value: string): string {
  const url = new URL(value)
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.pathname !== '/' || url.search || url.hash) {
    throw new Error('VITE_SITE_URL 必须是完整的 http(s) 站点 origin，不能含子路径、凭据或查询参数')
  }
  return url.origin
}
export const escapeHtml = (value: string) => value.replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]!)

// One source for both the initial HTML and client-side navigation.
export function seoHead(page: SeoPage, origin: string): string {
  const title = `${page.title} · ${siteName}`
  const url = `${origin}${page.path}`
  const image = `${origin}/yhimhas-logo.jpg`
  const meta = (key: string, value: string, property = false) => `<meta data-seo ${property ? 'property' : 'name'}="${key}" content="${escapeHtml(value)}">`
  const tags = [
    `<title data-seo>${escapeHtml(title)}</title>`, meta('description', page.description),
    meta('robots', page.noindex ? 'noindex, follow' : 'index, follow, max-image-preview:large'),
    ...(!page.noindex ? [`<link data-seo rel="canonical" href="${escapeHtml(url)}">`] : []),
    meta('og:site_name', siteName, true), meta('og:locale', 'zh_CN', true),
    meta('og:type', page.post ? 'article' : 'website', true), meta('og:title', title, true),
    meta('og:description', page.description, true), meta('og:url', url, true),
    meta('og:image', image, true), meta('og:image:alt', 'Yhimhas 网站标志', true),
    meta('twitter:card', 'summary'), meta('twitter:title', title),
    meta('twitter:description', page.description), meta('twitter:image', image), meta('twitter:image:alt', 'Yhimhas 网站标志'),
  ]
  if (page.post) {
    tags.push(meta('article:published_time', page.post.publishedAt, true), meta('article:modified_time', page.post.updatedAt, true))
    const schema = { '@context': 'https://schema.org', '@type': 'BlogPosting', headline: page.post.title,
      description: page.description, url, mainEntityOfPage: url, image: [image],
      datePublished: page.post.publishedAt, dateModified: page.post.updatedAt,
      author: { '@type': 'Person', name: 'Yhimhas', url: `${origin}/about` } }
    tags.push(`<script data-seo type="application/ld+json">${JSON.stringify(schema).replace(/</g, '\\u003c')}</script>`)
  }
  return tags.join('\n    ')
}
