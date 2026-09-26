<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, RouterLink } from 'vue-router'
import { FieldDialog, TechButton, TechTabs } from '@field-lab/vue'
import MarkdownIt from 'markdown-it'
import { stringify } from 'yaml'
import { posts, categories } from '../content'
import { loadPosts } from '../content/library'

type Draft = { id: string; title: string; slug: string; published: string; category: string; summary: string; tags: string; body: string; updated: string }
const storageKey = 'yhimhas:editor:drafts:v1'
const drafts = ref<Draft[]>([])
const status = ref('草稿仅保存在此浏览器，不会发布到网站。')
const storageBlocked = ref(false)
try {
  const raw = localStorage.getItem(storageKey)
  if (raw) {
    const data: unknown = JSON.parse(raw)
    const fields = ['id', 'title', 'slug', 'published', 'category', 'summary', 'tags', 'body', 'updated']
    if (!Array.isArray(data) || !data.every(d => d && fields.every(k => typeof d[k] === 'string')) || new Set(data.map(d => d.id)).size !== data.length) throw new Error('invalid')
    drafts.value = data
  }
} catch {
  storageBlocked.value = true
  status.value = '无法读取本地草稿。为保护已有数据，本次不会覆盖存储；可编辑并导出 Markdown。'
}
const tab = ref('published')
const tabs = computed(() => [{ id: 'published', label: `已收录 · ${posts.length}` }, { id: 'drafts', label: `本地草稿 · ${drafts.value.length}` }])
const query = ref('')
const category = ref('all')
const visiblePosts = computed(() => posts.filter(p => (category.value === 'all' || p.category === category.value) && p.searchText.includes(query.value.trim().toLowerCase())))
const visibleDrafts = computed(() => drafts.value.filter(d => (category.value === 'all' || d.category === category.value) && `${d.title} ${d.summary} ${d.tags}`.toLowerCase().includes(query.value.trim().toLowerCase())))
const filterCategories = computed(() => [...new Set([...categories, ...drafts.value.map(d => d.category).filter(Boolean)])])
function emptyDraft(): Draft {
  const now = new Date()
  return { id: crypto.randomUUID(), title: '', slug: '', published: `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`, category: '开发笔记', summary: '', tags: '', body: '', updated: now.toISOString() }
}
const editor = reactive<Draft>(emptyDraft())
const editing = ref(false)
const titleInput = ref<HTMLInputElement>()
const dirty = ref(false)
const preview = ref(false)
const markdown = new MarkdownIt({ html: false })
const previewHtml = computed(() => markdown.render(editor.body))
const source = computed(() => `---\n${stringify({ title: editor.title.trim(), slug: editor.slug.trim(), published: editor.published, category: editor.category.trim(), summary: editor.summary.trim(), tags: [...new Set(editor.tags.split(/[,，]/).map(t => t.trim()).filter(Boolean))] })}---\n\n${editor.body.trim()}\n`)
watch(editor, () => { if (editing.value) dirty.value = true }, { flush: 'sync' })
function save(): boolean {
  if (!editing.value || !dirty.value) return true
  if (storageBlocked.value) { status.value = '本地存储不可用，请先导出当前草稿。'; return false }
  const draft = { ...editor, updated: new Date().toISOString() }
  const next = [draft, ...drafts.value.filter(d => d.id !== draft.id)]
  try {
    localStorage.setItem(storageKey, JSON.stringify(next))
    drafts.value = next
    dirty.value = false
    status.value = `已保存到此浏览器 · ${new Date().toLocaleTimeString('zh-CN')}`
    return true
  } catch { status.value = '保存失败，浏览器存储可能已满或被禁用。请导出当前草稿。'; return false }
}
function openDraft(draft?: Draft) {
  if (dirty.value && !save()) return
  Object.assign(editor, draft ?? emptyDraft())
  editing.value = true
  dirty.value = !draft
  tab.value = 'drafts'
  void nextTick(() => titleInput.value?.focus())
}
function exportDraft() {
  try {
    loadPosts({ 'draft.md': source.value })
    if (posts.some(p => p.slug === editor.slug.trim())) throw new Error('slug 与已收录文章重复，请使用新的地址。')
  } catch (error) { status.value = error instanceof Error ? error.message : '请检查文章字段。'; return }
  const blob = new Blob([source.value], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `${editor.slug.trim()}.md`
  link.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  dirty.value = false
  status.value = '已发起 Markdown 下载。放入文章目录并重新构建部署后，网站才会更新。'
}
onBeforeRouteLeave(() => !dirty.value || save())
// Protect unsaved edits when the tab is closed, including storage failure cases.
function beforeUnload(event: BeforeUnloadEvent) { if (dirty.value && !save()) event.preventDefault() }
onMounted(() => window.addEventListener('beforeunload', beforeUnload))
onUnmounted(() => window.removeEventListener('beforeunload', beforeUnload))
</script>

<template>
  <div class="admin-page">
    <header class="admin-heading"><div><div class="eyebrow">04 / CONTENT WORKSPACE</div><h1>内容管理<span> / CONSOLE</span></h1><p>整理已经写下的，也为下一个想法留出位置。</p></div><TechButton variant="accent" @click="openDraft()">新建草稿 +</TechButton></header>
    <div class="admin-mode"><span><i class="status-dot" /> 本地工作台</span><p>文章从 Markdown 读取 · 草稿保存在此浏览器 · 尚未连接发布服务</p><RouterLink to="/blog">查看博客 ↗</RouterLink></div>
    <section class="admin-stats" aria-label="内容概览"><div><span>01 / ARCHIVE</span><strong>{{ String(posts.length).padStart(2, '0') }}</strong><p>已收录文章</p></div><div><span>02 / WORK IN PROGRESS</span><strong>{{ String(drafts.length).padStart(2, '0') }}</strong><p>本地草稿</p></div><div><span>03 / TOPICS</span><strong>{{ String(categories.length).padStart(2, '0') }}</strong><p>文章分类</p></div><div class="admin-latest"><span>04 / LAST ENTRY</span><strong>{{ posts[0]?.date || '—' }}</strong><p>最近文章日期</p></div></section>
    <div class="admin-workspace"><section class="admin-library" aria-label="文章与草稿"><div class="admin-section-title"><h2>内容档案</h2><span>CONTENT INDEX</span></div>
      <div class="admin-filters"><label class="f-input-shell"><span>⌕</span><input v-model="query" type="search" placeholder="搜索标题、摘要或标签" aria-label="搜索内容"></label><select v-model="category" aria-label="筛选分类"><option value="all">全部分类</option><option v-for="item in filterCategories" :key="item" :value="item">{{ item }}</option></select></div>
      <TechTabs v-model="tab" :items="tabs" label="内容状态"><div v-if="tab === 'published'"><article v-for="(post, index) in visiblePosts" :key="post.id" class="admin-row"><span class="admin-row-number">{{ String(index + 1).padStart(2, '0') }}</span><div><span class="admin-row-meta">{{ post.category }} / {{ post.date }}</span><h3><RouterLink :to="`/blog/${post.id}`">{{ post.title }}</RouterLink></h3><p>{{ post.summary }}</p></div><RouterLink class="admin-row-action" :to="`/blog/${post.id}`" :aria-label="`查看文章：${post.title}`">查看 ↗</RouterLink></article><div v-if="!visiblePosts.length" class="admin-empty">没有匹配的文章，试试其他关键词或分类。</div></div><div v-else><article v-for="draft in visibleDrafts" :key="draft.id" class="admin-row" :class="{ selected: editing && editor.id === draft.id }"><span class="admin-row-number">↳</span><div><span class="admin-row-meta">{{ draft.category || '未分类' }} / 本地草稿</span><h3>{{ draft.title || '未命名草稿' }}</h3><p>{{ draft.summary || '还没有摘要，继续写下你的想法。' }}</p></div><button class="admin-row-action" @click="openDraft(draft)" :aria-label="`编辑草稿：${draft.title || '未命名草稿'}`">编辑 ↗</button></article><div v-if="!visibleDrafts.length" class="admin-empty"><strong>{{ drafts.length ? '没有匹配的草稿' : '下一个想法，从这里开始。' }}</strong><p>{{ drafts.length ? '试试其他关键词或分类。' : '点击「新建草稿」，写下第一行文字。' }}</p></div></div></TechTabs>
    </section><aside class="admin-guide"><span class="eyebrow">WORKFLOW / 01—03</span><h2>从想法，<br>到一篇文字。</h2><ol><li><strong>在这里写作</strong><p>编辑标题、摘要与 Markdown 正文。</p></li><li><strong>保存与预览</strong><p>本地草稿支持继续编辑，离开页面时尝试保存。</p></li><li><strong>导出并发布</strong><p>下载 .md 文件，放入项目的文章目录，重新构建并部署。</p></li></ol><div class="admin-guide-note">此页面是公开可访问的本地编辑工具，没有登录或云端同步。更换浏览器不会带上草稿，请及时导出备份。</div></aside></div>
    <p class="admin-feedback" role="status" aria-live="polite">{{ status }}</p>
    <section v-if="editing" class="admin-editor" aria-labelledby="editor-title"><div class="admin-section-title"><h2 id="editor-title">草稿编辑器</h2><span>{{ dirty ? '● 尚未保存' : '✓ 已保存或导出' }}</span></div><form @submit.prevent="save"><div class="editor-fields"><label class="editor-wide">文章标题<input ref="titleInput" v-model="editor.title" placeholder="给这个想法起一个名字" maxlength="200"></label><label>文章地址 / slug<input v-model="editor.slug" placeholder="my-new-post"></label><label>发布日期<input v-model="editor.published" type="date"></label><label>分类<input v-model="editor.category" list="editor-categories"><datalist id="editor-categories"><option v-for="item in categories" :key="item" :value="item" /></datalist></label><label>标签（逗号分隔）<input v-model="editor.tags" placeholder="Vue 3, 生活"></label><label class="editor-wide">文章摘要<textarea v-model="editor.summary" rows="2" placeholder="用一两句话介绍这篇文章" /></label><label class="editor-wide">正文 / Markdown<textarea v-model="editor.body" class="editor-body" rows="14" placeholder="## 从一个想法开始&#10;&#10;在这里写下正文……" /></label></div><div class="editor-actions"><span>{{ editor.body.length }} 字符 · 草稿可保存未完成内容</span><TechButton variant="outline" :arrow="false" @click="preview = true">预览</TechButton><TechButton type="submit" variant="solid" :arrow="false">保存草稿</TechButton><TechButton variant="accent" @click="exportDraft">导出 Markdown</TechButton></div></form></section>
    <FieldDialog v-model="preview" title="文章预览" eyebrow="LOCAL PREVIEW"><h2>{{ editor.title || '未命名草稿' }}</h2><p class="preview-summary">{{ editor.summary }}</p><div class="admin-preview" v-html="previewHtml" /><p v-if="!editor.body">还没有正文，写下第一行再来看看。</p></FieldDialog>
  </div>
</template>

<style scoped>
.admin-page{padding:42px 4.4%;}.admin-heading{display:flex;justify-content:space-between;align-items:center;gap:24px}.admin-heading h1{font-size:38px;margin:15px 0 12px}.admin-heading h1 span{font:12px var(--f-mono);color:#7d866f;letter-spacing:.1em;margin-left:16px}.admin-heading p{font-size:13px;color:#727b66}.admin-mode{display:flex;align-items:center;gap:24px;border-block:1px solid #cbd2be;padding:12px 0;margin:28px 0;font-size:11px;color:#6c775e}.admin-mode>span{white-space:nowrap}.admin-mode i{margin-right:7px}.admin-mode p{flex:1;margin:0;line-height:1.8}.admin-mode a{white-space:nowrap}.admin-stats{display:grid;grid-template-columns:repeat(4,1fr);border:1px solid #cbd2be;background:#f5f6f0}.admin-stats>div{padding:24px;border-right:1px solid #cbd2be}.admin-stats>div:last-child{border:0;background:#222a20;color:#e6ebdc}.admin-stats span{font:9px var(--f-detail);color:#758065;letter-spacing:.05em}.admin-stats strong{display:block;font:44px var(--f-mono);margin:24px 0 12px}.admin-stats p{font-size:12px;margin:0}.admin-stats .admin-latest strong{font-size:clamp(15px,1.7vw,25px);margin-top:36px;margin-bottom:19px}.admin-latest span{color:#b3c398}.admin-workspace{display:grid;grid-template-columns:minmax(0,1fr) 250px;gap:32px;margin-top:40px}.admin-section-title{display:flex;align-items:center;justify-content:space-between;gap:15px;margin-bottom:24px}.admin-section-title h2{font-size:23px;margin:0}.admin-section-title>span{font:9px var(--f-detail);color:#758166}.admin-filters{display:flex;gap:12px;margin-bottom:22px}.admin-filters label{display:flex;align-items:center;gap:12px;flex:1;min-width:0;border:1px solid #cbd2be;padding:0 12px;background:#f5f6f0}.admin-filters input{padding:13px 0;border:0;background:transparent;width:100%;min-width:0;font-size:12px}.admin-filters select{max-width:140px;background:#f5f6f0;border:1px solid #cbd2be;padding:10px;color:inherit;font:12px inherit}.admin-row{display:flex;gap:16px;align-items:start;padding:24px 0;border-bottom:1px solid #cbd2be}.admin-row>div{flex:1;min-width:0}.admin-row-number{font:11px var(--f-mono);color:#8d977f;padding-top:4px}.admin-row-meta{font-size:10px;color:#78836a}.admin-row h3{font-size:16px;line-height:1.6;margin:9px 0;overflow-wrap:anywhere}.admin-row p{font-size:12px;line-height:1.8;color:#7a836e;margin:0;overflow-wrap:anywhere}.admin-row-action{flex-shrink:0;padding:8px;border:1px solid #bfc9ad;font-size:11px;background:transparent;margin-top:20px;color:#4c5b3a}.admin-row-action:hover{background:#d4ef37}.admin-row.selected{background:#e5ead9}.admin-guide{padding:25px;background:#e2e7d9;align-self:start;border-top:3px solid #4d5a3e}.admin-guide h2{font-size:24px;line-height:1.5;margin:25px 0}.admin-guide ol{padding-left:18px;font-size:12px}.admin-guide li{padding:0 0 16px 7px}.admin-guide li::marker{color:#869467}.admin-guide p{font-size:12px;line-height:1.9;color:#758166}.admin-guide-note{border-top:1px solid #c3cdb5;padding-top:18px;font-size:11px;line-height:1.9;color:#667254}.admin-empty{padding:45px 15px;color:#6c785d;font-size:13px;line-height:1.9}.admin-feedback{padding:15px 18px;border-left:3px solid #8d9f49;background:#e5eadb;font-size:12px;line-height:1.8;overflow-wrap:anywhere;margin:30px 0}.admin-editor{border:1px solid #cbd2be;background:#f5f6f0;padding:28px}.editor-fields{display:grid;grid-template-columns:1fr 1fr;gap:20px}.editor-fields label{display:flex;flex-direction:column;gap:10px;font-size:12px;color:#657155;min-width:0}.editor-wide{grid-column:1/-1}.editor-fields input,.editor-fields textarea{width:100%;border:1px solid #c7cfbc;padding:12px;background:#fafbf6;font:14px/1.7 var(--f-sans);color:#293223;border-radius:0}.editor-fields textarea{resize:vertical}.editor-fields .editor-body{font:13px/1.9 var(--f-mono);min-height:240px}.editor-actions{display:flex;justify-content:flex-end;flex-wrap:wrap;align-items:center;gap:12px;margin-top:22px}.editor-actions>span{margin-right:auto;font-size:11px;color:#758166}.admin-page :is(textarea,select):focus-visible{outline:2px solid #687a13;outline-offset:3px}.preview-summary{color:#738260;line-height:1.8}.admin-preview{overflow-wrap:anywhere;font-size:14px;line-height:1.9}.admin-preview :deep(img){max-width:100%}.admin-preview :deep(pre){overflow:auto;padding:16px;background:#e4e9da}.admin-preview :deep(table){display:block;overflow:auto}.admin-preview :deep(a){text-decoration:underline}.admin-preview :deep(h2){margin-top:24px}@media(max-width:1050px){.admin-workspace{grid-template-columns:minmax(0,1fr) 220px;gap:20px}.admin-stats>div{padding:18px}}@media(max-width:760px){.admin-page{padding-top:28px}.admin-heading{align-items:start;flex-direction:column;gap:12px}.admin-heading h1{font-size:30px}.admin-heading h1 span{font-size:9px;margin-left:8px}.admin-mode{flex-wrap:wrap;gap:10px}.admin-mode p{flex-basis:100%;order:3}.admin-mode a{margin-left:auto}.admin-stats{grid-template-columns:1fr 1fr}.admin-stats>div{border-bottom:1px solid #cbd2be}.admin-stats>div:nth-child(2){border-right:0}.admin-stats strong{font-size:34px}.admin-stats .admin-latest strong{font-size:19px;margin-top:32px}.admin-workspace{grid-template-columns:1fr}.admin-guide{margin-top:12px}.admin-row{gap:10px}.admin-row h3{font-size:15px}.admin-editor{padding:18px}.editor-fields{grid-template-columns:1fr}.editor-wide{grid-column:auto}.editor-actions>span{width:100%;margin-bottom:5px}.editor-actions{justify-content:start}.admin-filters{flex-wrap:wrap}.admin-filters label{flex-basis:100%}}
</style>
