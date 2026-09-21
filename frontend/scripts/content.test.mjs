import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readdirSync, readFileSync } from 'node:fs'
import { loadPosts } from '../src/content/library.ts'

const article = (metadata = '', body = '## 标题\n\n正文关键词 **加粗** 与 `code`。') => `---
title: 测试文章
slug: test-post
published: "2026-09-21"
category: 新分类
summary: 文章摘要
tags: [Vue, 写作]
${metadata}---

${body}`

test('reads the four migrated posts and preserves their URLs and dates', () => {
  const directory = new URL('../src/content/posts/', import.meta.url)
  const posts = loadPosts(Object.fromEntries(readdirSync(directory).filter(name => name.endsWith('.md'))
    .map(name => [name, readFileSync(new URL(name, directory), 'utf8')])))
  for (const [slug, date] of Object.entries({ building: '2026.09.16', components: '2026.09.14', 'slow-days': '2026.09.12', listening: '2026.09.10' })) {
    const post = posts.find(post => post.slug === slug)
    assert.equal(post?.id, slug)
    assert.equal(post?.date, date)
    assert.match(post.html, /<h2>/)
  }
  assert.ok(posts.find(post => post.slug === 'components').searchText.includes('update:visible'))
})

test('renders Markdown and indexes body, summary, category and tags', () => {
  const [post] = loadPosts({ 'test.md': article() })
  assert.match(post.html, /<strong>加粗<\/strong>/)
  for (const query of ['正文关键词', '文章摘要', '新分类', 'vue', 'code']) assert.ok(post.searchText.includes(query))
  assert.equal(post.published, '2026-09-21')
})

test('sorts by date descending, then slug for stable ties; accepts CRLF', () => {
  const posts = loadPosts({
    'old.md': article().replace('test-post', 'old').replace('2026-09-21', '2026-09-01'),
    'z.md': article().replace('test-post', 'z').replaceAll('\n', '\r\n'),
    'a.md': article().replace('test-post', 'a'),
  })
  assert.deepEqual(posts.map(post => post.slug), ['a', 'z', 'old'])
})

test('rejects invalid metadata with the source filename', () => {
  for (const source of [
    'missing frontmatter',
    article().replace('title: 测试文章\n', ''),
    article().replace('2026-09-21', '2026-02-30'),
    article().replace('test-post', 'bad/slug'),
    article().replace('[Vue, 写作]', '[]'),
    article().replace('[Vue, 写作]', 'Vue'),
    article('title: duplicate\n'),
    article('', ''),
  ]) assert.throws(() => loadPosts({ 'invalid.md': source }), /invalid\.md/)
  assert.throws(() => loadPosts({ 'one.md': article(), 'two.md': article() }), /slug 重复/)
})

test('does not render executable HTML or javascript links', () => {
  const [post] = loadPosts({ 'test.md': article('', '<script>alert(1)</script>\n\n[x](javascript:alert(1))\n\n[正常链接](https://example.com)') })
  assert.ok(!post.html.includes('<script>'))
  assert.ok(!post.html.includes('href="javascript:'))
  assert.match(post.html, /href="https:\/\/example.com"/)
})
