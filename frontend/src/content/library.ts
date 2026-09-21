import MarkdownIt from 'markdown-it'
import { parseDocument } from 'yaml'

export interface Post {
  /** Compatibility with the existing /blog/:id route: always equal to slug. */
  id: string
  slug: string
  title: string
  category: string
  published: string
  date: string
  summary: string
  tags: string[]
  html: string
  searchText: string
}

// Local Markdown supports standard formatting, but never executes embedded HTML.
const markdown = new MarkdownIt({ html: false })

export function loadPosts(files: Record<string, string>): Post[] {
  const slugs = new Set<string>()
  return Object.entries(files).map(([file, source]) => {
    const fail = (message: string): never => { throw new Error(`[文章 ${file}] ${message}`) }
    const match = /^---\n([\s\S]*?)\n---(?:\n|$)([\s\S]*)$/.exec(source.replace(/^\uFEFF/, '').replace(/\r\n/g, '\n'))
    if (!match) return fail('需要 YAML frontmatter（以 --- 包围）。')
    const doc = parseDocument(match[1]!)
    if (doc.errors.length) return fail(`YAML 格式错误：${doc.errors[0]!.message}`)
    const data: unknown = doc.toJS({ maxAliasCount: 0 })
    if (!data || typeof data !== 'object' || Array.isArray(data)) return fail('元数据必须为键值对象。')
    const metadata = data as Record<string, unknown>
    const required = (key: string): string => {
      const value = metadata[key]
      return typeof value === 'string' && value.trim() ? value.trim() : fail(`${key} 必须是非空字符串。`)
    }
    const title = required('title')
    const slug = required('slug')
    const published = required('published')
    const category = required('category')
    if (category === 'all') return fail('category 不能使用保留值 all。')
    const summary = required('summary')
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug)) return fail('slug 仅支持小写英文字母、数字和单个连字符。')
    if (slugs.has(slug)) return fail(`slug 重复：${slug}`)
    slugs.add(slug)
    const timestamp = Date.parse(`${published}T00:00:00Z`)
    if (!/^\d{4}-\d{2}-\d{2}$/.test(published) || !Number.isFinite(timestamp) || new Date(timestamp).toISOString().slice(0, 10) !== published) {
      return fail('published 必须是有效的 YYYY-MM-DD 日期。')
    }
    const tags = metadata.tags
    if (!Array.isArray(tags) || !tags.length || tags.some(tag => typeof tag !== 'string' || !tag.trim())) {
      return fail('tags 必须是至少包含一个非空字符串的数组。')
    }
    const body = match[2]!.trim()
    if (!body) return fail('Markdown 正文不能为空。')
    const tokens = markdown.parse(body, {})
    const bodyText = tokens.map(token => token.children
      ? token.children.map(child => child.content).join('')
      : token.content).join(' ')
    return {
      id: slug, slug, title, category, published,
      date: published.replaceAll('-', '.'), summary,
      tags: [...new Set((tags as string[]).map(tag => tag.trim()))],
      html: markdown.renderer.render(tokens, markdown.options, {}),
      searchText: [title, summary, category, ...tags, bodyText].join(' ').toLowerCase(),
    }
  }).sort((a, b) => b.published.localeCompare(a.published) || a.slug.localeCompare(b.slug))
}
