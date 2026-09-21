import { readdirSync, readFileSync } from 'node:fs'
import { loadPosts } from '../src/content/library.ts'

const directory = new URL('../src/content/posts/', import.meta.url)
const files = Object.fromEntries(readdirSync(directory)
  .filter(name => name.endsWith('.md'))
  .map(name => [name, readFileSync(new URL(name, directory), 'utf8')]))
const posts = loadPosts(files)
console.log(`文章检查通过：${posts.length} 篇，slug 无重复，元数据与正文有效。`)
