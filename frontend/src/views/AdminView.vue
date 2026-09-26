<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, reactive, ref } from 'vue'
import { onBeforeRouteLeave, RouterLink } from 'vue-router'
import { ApiError, blogApi, type AdminPost, type PostInput, type Term } from '../blogApi'
import { renderMarkdown } from '../markdown'

const checking = ref(true)
const authenticated = ref(false)
const username = ref('')
const password = ref('')
const busy = ref(false)
const error = ref('')
const notice = ref('')
const conflict = ref(false)
const posts = ref<AdminPost[]>([])
const categories = ref<Term[]>([])
const tags = ref<Term[]>([])
const page = ref(1)
const total = ref(0)
const selected = ref<AdminPost>()
const editing = ref(false)
const blank = (): PostInput => ({ slug: '', title: '', summary: '', contentMarkdown: '', categoryId: null, tagIds: [] })
const form = reactive<PostInput>(blank())
const snapshot = ref(JSON.stringify(form))
const dirty = computed(() => editing.value && snapshot.value !== JSON.stringify(form))
const preview = computed(() => renderMarkdown(form.contentMarkdown))
const statusNames = { draft: '草稿', published: '已发布', archived: '已归档' }
const allowedToLeave = () => !busy.value && (!dirty.value || window.confirm('有尚未保存的修改，确定离开当前文章吗？'))
onBeforeRouteLeave(allowedToLeave)
function beforeUnload(event: BeforeUnloadEvent) {
  if (dirty.value || busy.value) { event.preventDefault(); event.returnValue = '' }
}
onMounted(() => { window.addEventListener('beforeunload', beforeUnload); void restore() })
onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))
function handleError(e: unknown) {
  if (e instanceof ApiError && (e.status === 401 || e.code === 'CSRF_FAILED')) {
    authenticated.value = false
    error.value = '会话已过期，请重新登录。当前编辑内容仍保留在此页面。'
  } else if (e instanceof ApiError && e.status === 409) {
    conflict.value = true
    error.value = selected.value ? '文章已被其他会话修改。当前输入已保留，请对照服务器版本后重新编辑。' : '这个 slug 已被使用，请修改后重试。'
  } else { error.value = e instanceof Error ? e.message : '操作失败，请重试。' }
}
async function run(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true; error.value = ''; notice.value = ''
  try { await action() } catch (e) { handleError(e) } finally { busy.value = false }
}
async function refreshList() {
  const result = await blogApi.adminPosts(page.value)
  posts.value = result.data; total.value = result.pagination.total
}
async function loadWorkspace() {
  const [cats, terms] = await Promise.all([blogApi.terms('categories'), blogApi.terms('tags')])
  categories.value = cats; tags.value = terms
  await refreshList()
}
async function restore() {
  checking.value = true
  try { await blogApi.session(); authenticated.value = true; await loadWorkspace() }
  catch (e) { if (!(e instanceof ApiError && e.status === 401)) handleError(e) }
  finally { checking.value = false }
}
async function login() {
  await run(async () => {
    try { await blogApi.login(username.value, password.value) } finally { password.value = '' }
    authenticated.value = true
    await loadWorkspace()
  })
}
function fill(post?: AdminPost) {
  selected.value = post
  Object.assign(form, post ? {
    slug: post.slug, title: post.title, summary: post.summary, contentMarkdown: post.contentMarkdown,
    categoryId: post.categoryId, tagIds: [...post.tagIds],
  } : blank())
  snapshot.value = JSON.stringify(form); editing.value = true; conflict.value = false
}
function newPost() {
  if (!allowedToLeave()) return
  fill(); error.value = ''; notice.value = ''
}
async function open(post: AdminPost) {
  if (!allowedToLeave()) return
  await run(async () => fill(await blogApi.adminPost(post.id)))
}
async function reload() {
  if (!selected.value || !allowedToLeave()) return
  const id = selected.value.id
  await run(async () => fill(await blogApi.adminPost(id)))
}
async function save(publish = false) {
  await run(async () => {
    if (!form.title.trim() || [...form.title].length > 160 || [...form.summary].length > 500 ||
      !/^[a-z0-9]+(-[a-z0-9]+)*$/.test(form.slug) || form.slug.length > 100 ||
      new TextEncoder().encode(form.contentMarkdown).length > 200 * 1024 ||
      (publish && !form.contentMarkdown.trim())) throw new Error('请检查标题、slug 和正文：标题最多 160 字，摘要最多 500 字，正文最多 200 KiB；发布时正文不能为空。')
    if (conflict.value && selected.value) throw new Error('请先处理版本冲突，再保存。')
    let result = selected.value
    if (!result || dirty.value) {
      const { slug, ...input } = { ...form, tagIds: [...form.tagIds] }
      result = result ? await blogApi.update(result.id, result.version, input) : await blogApi.create({ ...input, slug })
      fill(result)
      notice.value = '文章已保存。'
    }
    if (publish) {
      result = await blogApi.publish(result.id, result.version)
      fill(result)
      notice.value = '发布成功，可以通过下方链接公开访问。'
    }
    await refreshList()
  })
}
async function changePage(value: number) { await run(async () => { page.value = value; await refreshList() }) }
async function logout() {
  if (!allowedToLeave()) return
  await run(async () => { await blogApi.logout(); authenticated.value = false; editing.value = false; selected.value = undefined; Object.assign(form, blank()); posts.value = [] })
}
</script>

<template>
  <section class="admin-page content-section">
    <div class="section-header"><div><div class="eyebrow">OWNER / JOURNAL</div><h1 class="section-title">文章管理</h1></div><RouterLink to="/blog">查看公开博客 ↗</RouterLink></div>
    <p v-if="checking" role="status">正在检查登录状态…</p>
    <p v-if="error" role="alert" class="admin-error">{{ error }}</p>
    <p v-if="notice" role="status">{{ notice }}</p>
    <form v-if="!checking && !authenticated" class="admin-login" @submit.prevent="login">
      <h2>站长登录</h2>
      <label>用户名<input v-model="username" autocomplete="username" required :disabled="busy" /></label>
      <label>密码<input v-model="password" type="password" autocomplete="current-password" required :disabled="busy" /></label>
      <button :disabled="busy">{{ busy ? '登录中…' : '登录' }}</button>
      <p>使用 Go 后端创建的站长账号。</p>
    </form>
    <template v-if="!checking && authenticated">
      <div class="admin-actions"><button :disabled="busy" @click="newPost">新建文章</button><button :disabled="busy" @click="run(loadWorkspace)">刷新列表与分类</button><button :disabled="busy" @click="logout">退出登录</button></div>
      <div class="admin-layout">
        <aside aria-label="管理文章列表">
          <p v-if="!posts.length">暂无文章，点击「新建文章」开始。</p>
          <button v-for="post in posts" :key="post.id" class="admin-post" :class="{ selected: selected?.id === post.id }" :disabled="busy" @click="open(post)">
            <strong>{{ post.title }}</strong><span>{{ statusNames[post.status] }} · v{{ post.version }}</span>
          </button>
          <nav v-if="total > 20" class="admin-actions" aria-label="管理文章分页"><button :disabled="busy || page <= 1" @click="changePage(page - 1)">上一页</button><span>{{ page }} / {{ Math.ceil(total / 20) }}</span><button :disabled="busy || page * 20 >= total" @click="changePage(page + 1)">下一页</button></nav>
        </aside>
        <form v-if="editing" @submit.prevent="save()">
          <fieldset :disabled="busy">
            <legend>{{ selected ? statusNames[selected.status] : '新草稿' }}{{ dirty ? ' · 未保存' : '' }}</legend>
            <p v-if="selected?.status === 'published'">这篇文章已公开，保存修改会立即更新公开内容。</p>
            <label>标题<input v-model="form.title" required maxlength="160" /></label>
            <label>slug · 公开地址<input v-model="form.slug" required maxlength="100" pattern="[a-z0-9]+(-[a-z0-9]+)*" :readonly="!!selected" placeholder="my-first-post" /><small>仅小写英文、数字和连字符；创建后不可修改。</small></label>
            <label>摘要<textarea v-model="form.summary" maxlength="500" rows="3" /></label>
            <label>分类<select v-model="form.categoryId"><option :value="null">未分类</option><option v-for="term in categories" :key="term.id" :value="term.id">{{ term.name }}</option></select></label>
            <div class="admin-tags"><span>标签</span><label v-for="tag in tags" :key="tag.id"><input v-model="form.tagIds" type="checkbox" :value="tag.id" />{{ tag.name }}</label><small v-if="!tags.length">暂无标签，可直接保存和发布。</small></div>
            <label>Markdown 正文<textarea v-model="form.contentMarkdown" rows="18" class="admin-markdown" spellcheck="false" /></label>
            <div class="admin-actions"><button type="submit" :disabled="conflict && !!selected">{{ busy ? '处理中…' : '保存文章' }}</button><button type="button" :disabled="conflict && !!selected" @click="save(true)">{{ selected?.status === 'published' ? '保存并更新发布' : '保存并发布' }}</button><button v-if="conflict && selected" type="button" @click="reload">重新读取服务器版本</button></div>
            <RouterLink v-if="selected?.status === 'published'" :to="`/blog/${selected.slug}`">公开访问：/blog/{{ selected.slug }} ↗</RouterLink>
          </fieldset>
          <details open><summary>正文预览</summary><div class="markdown-body admin-preview" v-html="preview" /></details>
        </form>
        <p v-else>选择文章继续编辑，或新建一篇草稿。</p>
      </div>
    </template>
  </section>
</template>

<style scoped>
.admin-page { max-width: 1400px; margin: auto; }
.admin-layout { display: grid; grid-template-columns: minmax(180px, 260px) minmax(0, 1fr); gap: 32px; margin-top: 28px; }
.admin-actions { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; margin: 18px 0; }
.admin-page button { border: 1px solid #53594b; background: #d4ef37; color: #171a13; padding: 10px 16px; cursor: pointer; }
.admin-page button:disabled { opacity: .5; cursor: not-allowed; }
.admin-page label { display: grid; gap: 8px; margin-bottom: 18px; }
.admin-page input:not([type=checkbox]), .admin-page textarea, .admin-page select { width: 100%; padding: 12px; border: 1px solid #777; background: #fff; color: #222; font: inherit; box-sizing: border-box; }
.admin-page input:read-only { background: #eee; }
.admin-page fieldset { border: 0; padding: 0; min-width: 0; }
.admin-login { max-width: 420px; padding: 24px 0; }
.admin-page .admin-post { display: grid; gap: 8px; text-align: left; width: 100%; margin-bottom: 10px; background: transparent; color: inherit; overflow-wrap: anywhere; }
.admin-post.selected { border-left: 5px solid #879a16; }
.admin-post span, small { font-size: 12px; opacity: .8; }
.admin-error { border-left: 3px solid #b13b3b; padding: 12px; }
.admin-tags { display: flex; flex-wrap: wrap; gap: 16px; margin-bottom: 18px; }
.admin-tags label { display: flex; align-items: center; margin: 0; }
.admin-markdown { font-family: monospace !important; resize: vertical; }
.admin-preview { padding: 24px 0; overflow-wrap: anywhere; }
details { margin-top: 24px; }
@media (max-width: 760px) { .admin-layout { grid-template-columns: 1fr; } }
</style>
