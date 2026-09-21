import { loadPosts } from './library'

const files = import.meta.glob('./posts/*.md', {
  query: '?raw', import: 'default', eager: true,
}) as Record<string, string>

export const posts = loadPosts(files)
export const categories = [...new Set(posts.map(post => post.category))]
