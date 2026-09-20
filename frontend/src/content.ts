export interface Post {
  id: string
  title: string
  category: string
  date: string
  summary: string
  tags: string[]
  body: { title: string; text: string }[]
}

// 示例内容：接入 API 时保留此结构即可替换。
export const posts: Post[] = [
  { id: 'building', title: '从一个组件开始，搭建我的数字自留地', category: '开发笔记', date: '2026.09.16', summary: '把零散的想法变成可以访问的页面。关于 Vue、组件边界，以及这个小站的第一步。', tags: ['Vue 3', '个人网站'], body: [
    { title: '先让一个小部分工作起来', text: '一个个人网站不必从复杂的系统开始。先写下想分享的内容，再做一个可以阅读的页面。标题、简介与导航构成了最小的起点，也让下一步变得具体。' },
    { title: '把状态留在合适的地方', text: '文章筛选属于列表，弹层开关属于阅读体验。把数据和展示分开，组件就能专注于一件事。等真正接入接口时，再把本地内容替换成服务端返回的数据。' },
    { title: '给未来留一点空间', text: '这里会继续记录开发过程中遇到的问题、解决办法和反思。不急着把每个功能都塞进首页，让网站跟着学习一起生长。' },
  ] },
  { id: 'components', title: '组件之间，应该如何好好沟通？', category: '开发笔记', date: '2026.09.14', summary: '用 props 描述输入，用 emit 表达变化。整理一次关于 Vue 数据流的小练习。', tags: ['TypeScript', '组件设计'], body: [
    { title: '明确输入', text: 'props 是组件与外部约定的输入。用 TypeScript 写清楚类型，可以在接入组件的时候尽早发现遗漏。比起把整个页面状态都传进去，明确传入所需字段更容易理解。' },
    { title: '明确变化', text: '子组件发出事件，父组件决定如何更新状态。比如展开简介时发送 update:visible，配合 v-model 使用，就能让显示状态有一个清楚的来源。' },
  ] },
  { id: 'slow-days', title: '偶尔，也给生活留一点空白', category: '生活随记', date: '2026.09.12', summary: '合上电脑，去散一会儿步。有些灵感，出现在什么都没有做的时候。', tags: ['日常', '片刻'], body: [
    { title: '走出屏幕', text: '写代码时常常忘记时间。走出去以后才发现，天色已经换了一种颜色。路边的树、远处的声音和缓慢移动的云，让思绪有机会重新排列。' },
    { title: '记录小事', text: '并不是每一天都需要一个显著的成果。记住一阵风、一段旋律，或者一次没有目的的散步，也是一种值得保留的日常。' },
  ] },
  { id: 'listening', title: '让一段旋律，成为今天的背景', category: '音乐手记', date: '2026.09.10', summary: '记录听歌的时刻，比记录播放次数更有意思。这里是我的声音收藏计划。', tags: ['音乐', '灵感'], body: [
    { title: '声音与记忆', text: '有些旋律会把人带回某个具体的下午。建立一份自己的音乐清单，也是为这些时刻建立索引。与其追逐最新的歌，不如记下真正让自己停留的声音。' },
    { title: '这个小站的音乐角落', text: '音乐区域目前展示收藏方向，可以保存偏好并到平台寻找相关音乐。后续加入真实曲目清单后，再接入经过验证的官方播放入口。' },
  ] },
]

export const music = [
  { id: 'ambient', title: '把世界调成静音', subtitle: 'AMBIENT / 专注时刻', genre: '环境音乐', description: '留一点空间，给正在发生的想法。', color: 'mist' },
  { id: 'piano', title: '在黄昏之前', subtitle: 'PIANO / 午后片刻', genre: '钢琴纯音乐', description: '让琴键接住今天缓慢落下的光。', color: 'sand' },
  { id: 'electronic', title: '驶向未命名的远方', subtitle: 'ELECTRONIC / 夜间漫游', genre: '氛围电子音乐', description: '戴上耳机，为日常切换一条轨道。', color: 'night' },
]
