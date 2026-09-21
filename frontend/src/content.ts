// Shared content entry point for the existing views.
export { posts, categories } from './content/posts'
export type { Post } from './content/library'

export const music = [
  { id: 'ambient', title: '把世界调成静音', subtitle: 'AMBIENT / 专注时刻', genre: '环境音乐', description: '留一点空间，给正在发生的想法。', color: 'mist' },
  { id: 'piano', title: '在黄昏之前', subtitle: 'PIANO / 午后片刻', genre: '钢琴纯音乐', description: '让琴键接住今天缓慢落下的光。', color: 'sand' },
  { id: 'electronic', title: '驶向未命名的远方', subtitle: 'ELECTRONIC / 夜间漫游', genre: '氛围电子音乐', description: '戴上耳机，为日常切换一条轨道。', color: 'night' },
]
