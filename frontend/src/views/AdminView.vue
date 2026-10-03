<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, RouterLink, useRouter } from 'vue-router'
import { ApiError, blogApi, type AdminPost, type PostInput, type Term, type TermKind } from '../blogApi'
import { renderMarkdown } from '../markdown'
import { authSession } from '../authSession'
import { useSeoPublication } from '../useSeoPublication'
import SeoPublicationStatus from '../components/SeoPublicationStatus.vue'

const checking = ref(true)
const router = useRouter()
const authenticated = ref(false)
const staticPages = useSeoPublication({ onUnauthorized: handleError })
watch(authenticated, active => { if (active) void staticPages.start(); else staticPages.stop() }, { flush: 'sync' })
const busy = ref(false)
const error = ref('')
const notice = ref('')
const conflict = ref(false)
const posts = ref<AdminPost[]>([])
const categories = ref<Term[]>([])
const tags = ref<Term[]>([])
const page = ref(1)
const statusFilter = ref<AdminPost['status'] | ''>('')
const total = ref(0)
const selected = ref<AdminPost>()
const editing = ref(false)
const blank = (): PostInput => ({ slug: '', title: '', summary: '', contentMarkdown: '', categoryId: null, tagIds: [] })
const form = reactive<PostInput>(blank())
const snapshot = ref(JSON.stringify(form))
const dirty = computed(() => editing.value && snapshot.value !== JSON.stringify(form))
const termKind = ref<TermKind>('categories')
const termPanel = ref<HTMLDetailsElement>()
const termEditing = ref(false)
const termId = ref<string>()
const termForm = reactive({ name: '', slug: '' })
const termSnapshot = ref(JSON.stringify(termForm))
const termDirty = computed(() => termEditing.value && termSnapshot.value !== JSON.stringify(termForm))
const termItems = computed(() => termKind.value === 'categories' ? categories.value : tags.value)
const termLabel = computed(() => termKind.value === 'categories' ? '分类' : '标签')
const preview = computed(() => renderMarkdown(form.contentMarkdown))
const statusNames = { draft: '草稿', published: '已发布', archived: '已归档' }
const allowedToLeave = () => !busy.value && (!(dirty.value || termDirty.value) || window.confirm('有尚未保存的文章或分类标签修改，确定离开吗？'))
onBeforeRouteLeave(allowedToLeave)
function beforeUnload(event: BeforeUnloadEvent) {
  if (dirty.value || termDirty.value || busy.value) { event.preventDefault(); event.returnValue = '' }
}
onMounted(() => { window.addEventListener('beforeunload', beforeUnload); void restore() })
onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))
function handleError(e: unknown, scope: 'post' | 'term' = 'post') {
  if (e instanceof ApiError && (e.status === 401 || e.code === 'CSRF_FAILED' || e.code === 'ADMIN_REQUIRED')) {
    authenticated.value = false
    authSession.user.value = null
    error.value = '会话已过期，请重新登录。当前编辑内容仍保留在此页面。'
  } else if (e instanceof ApiError && e.status === 409) {
    if (scope === 'term') {
      error.value = e.code === 'TERM_IN_USE' ? e.message : '这个分类或标签 slug 已被使用，请修改后重试。'
    } else {
      conflict.value = !!selected.value
      error.value = selected.value ? '文章已被其他会话修改。当前输入已保留，请对照服务器版本后重新编辑。' : '这个 slug 已被使用，请修改后重试。'
    }
  } else { error.value = e instanceof Error ? e.message : '操作失败，请重试。' }
}
async function run(action: () => Promise<void>, scope: 'post' | 'term' = 'post') {
  if (busy.value) return
  busy.value = true; error.value = ''; notice.value = ''
  try { await action() } catch (e) { handleError(e, scope) } finally { busy.value = false }
}
async function refreshList() {
  let result = await blogApi.adminPosts(page.value, statusFilter.value)
  const lastPage = Math.max(1, Math.ceil(result.pagination.total / 20))
  if (page.value > lastPage) {
    page.value = lastPage
    result = await blogApi.adminPosts(page.value, statusFilter.value)
  }
  posts.value = result.data; total.value = result.pagination.total
}
async function loadWorkspace() {
  const [cats, terms] = await Promise.all([blogApi.terms('categories'), blogApi.terms('tags')])
  categories.value = cats; tags.value = terms
  await refreshList()
}
async function restore() {
  checking.value = true
  try { await authSession.restore(); authenticated.value = authSession.isAdmin.value; if (authenticated.value) await loadWorkspace() }
  catch (e) { if (!(e instanceof ApiError && e.status === 401)) handleError(e) }
  finally { checking.value = false }
}
function fill(post?: AdminPost) {
  selected.value = post
  const content = post?.revision ?? post
  Object.assign(form, post ? {
    slug: post.slug, title: content!.title, summary: content!.summary, contentMarkdown: content!.contentMarkdown,
    categoryId: content!.categoryId, tagIds: [...content!.tagIds],
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
  if (publish && !window.confirm('确认发布当前编辑内容？正文、摘要、分类和标签将公开可见。')) return
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
      notice.value = result.revision ? '修订草稿已保存，公开内容未改变。审核预览后可确认发布。' : '文章已保存。'
    }
    if (publish) {
      result = await blogApi.publish(result.id, result.version)
      fill(result)
      notice.value = '文章已发布到数据库，静态页面更新情况请查看下方状态。'
      staticPages.markPending()
      void staticPages.refresh()
    }
    await refreshList()
  })
}
async function archive() {
  const post = selected.value
  if (!post || busy.value || dirty.value || conflict.value) return
  if (!window.confirm('确认归档这篇文章？公开 API 将隐藏文章，旧静态页面要等更新完成才会撤回。内容和修订草稿将保留，可再次发布。')) return
  await run(async () => {
    fill(await blogApi.archive(post.id, post.version))
    notice.value = '文章已归档，公开 API 已隐藏；旧静态页面是否撤回请查看下方状态。'
    staticPages.markPending()
    void staticPages.refresh()
    await refreshList()
  })
}
async function filterPosts(event: Event) {
  const value = (event.target as HTMLSelectElement).value as typeof statusFilter.value
  await run(async () => { statusFilter.value = value; page.value = 1; await refreshList() })
}
function editTerm(term?: Term, kind = termKind.value) {
  if (busy.value || (termDirty.value && !window.confirm('分类标签有未保存的修改，确定放弃这些修改吗？'))) return
  termKind.value = kind; termId.value = term?.id
  Object.assign(termForm, { name: term?.name ?? '', slug: term?.slug ?? '' })
  termSnapshot.value = JSON.stringify(termForm); termEditing.value = true
}
function openTermManager() {
  if (!termPanel.value) return
  termPanel.value.open = true
  termPanel.value.scrollIntoView({ block: 'start' })
  termPanel.value.querySelector('summary')?.focus()
}
async function saveTerm() {
  await run(async () => {
    const term = await blogApi.saveTerm(termKind.value, { name: termForm.name.trim(), slug: termForm.slug }, termId.value)
    termId.value = term.id; Object.assign(termForm, { name: term.name, slug: term.slug })
    termSnapshot.value = JSON.stringify(termForm)
    notice.value = `${termLabel.value}已保存。`
    await loadWorkspace()
  }, 'term')
}
async function deleteTerm(term: Term) {
  if (busy.value) return
  if (editing.value && (termKind.value === 'categories' ? form.categoryId === term.id : form.tagIds.includes(term.id))) {
    error.value = '当前编辑内容正在使用此项，请先解除关联并保存文章。'; return
  }
  if (termEditing.value && termId.value === term.id && termDirty.value) {
    error.value = '此项有未保存的修改，请先保存或取消编辑。'; return
  }
  if (!window.confirm(`确认删除${termLabel.value}「${term.name}」？被文章或修订草稿使用的项目不能删除。`)) return
  await run(async () => {
    await blogApi.deleteTerm(termKind.value, term.id)
    if (termId.value === term.id) { termEditing.value = false; termId.value = undefined }
    notice.value = `${termLabel.value}已删除。`
    await loadWorkspace()
  }, 'term')
}
function cancelTerm() {
  if (termDirty.value && !window.confirm('确定放弃尚未保存的分类标签修改吗？')) return
  termEditing.value = false
}
async function changePage(value: number) { await run(async () => { page.value = value; await refreshList() }) }
async function logout() {
  if (!allowedToLeave()) return
  await run(async () => { await authSession.logout(); authenticated.value = false; editing.value = false; termEditing.value = false; selected.value = undefined; Object.assign(form, blank()); posts.value = [] })
  if (!authenticated.value) await router.replace('/login')
}
</script>

<template>
  <section class="admin-page content-section">
    <div class="section-header"><div><div class="eyebrow">OWNER / JOURNAL</div><h1 class="section-title">文章管理</h1></div><RouterLink to="/blog">查看公开博客 ↗</RouterLink></div>
    <p v-if="authenticated"><RouterLink to="/admin/local">打开本地草稿工作台 ↗</RouterLink></p>
    <p v-if="checking" role="status">正在检查登录状态…</p>
    <p v-if="error" role="alert" class="admin-error">{{ error }}</p>
    <p v-if="notice" role="status">{{ notice }}</p>
    <p v-if="!checking && !authenticated"><RouterLink to="/login?next=/admin">前往用户登录</RouterLink></p>
    <template v-if="!checking && authenticated">
      <div class="admin-actions"><button :disabled="busy" @click="newPost">新建文章</button><button @click="openTermManager">分类与标签管理</button><button :disabled="busy" @click="run(loadWorkspace)">刷新列表与分类</button><button :disabled="busy" @click="logout">退出登录</button></div>
      <SeoPublicationStatus :state="staticPages.state.value" :loading="staticPages.loading.value" @refresh="staticPages.refresh" />
      <div class="admin-layout">
        <aside aria-label="管理文章列表">
          <label>文章状态<select :value="statusFilter" :disabled="busy" @change="filterPosts"><option value="">全部文章</option><option value="draft">草稿</option><option value="published">已发布</option><option value="archived">已归档</option></select></label>
          <p>共 {{ total }} 篇</p>
          <p v-if="!posts.length">当前状态暂无文章。</p>
          <button v-for="post in posts" :key="post.id" class="admin-post" :class="{ selected: selected?.id === post.id }" :disabled="busy" @click="open(post)">
            <strong>{{ post.revision?.title ?? post.title }}</strong><span>{{ statusNames[post.status] }}{{ post.revision ? ' · 有待发布修订' : '' }} · v{{ post.version }}</span>
          </button>
          <nav v-if="total > 20" class="admin-actions" aria-label="管理文章分页"><button :disabled="busy || page <= 1" @click="changePage(page - 1)">上一页</button><span>{{ page }} / {{ Math.ceil(total / 20) }}</span><button :disabled="busy || page * 20 >= total" @click="changePage(page + 1)">下一页</button></nav>
        </aside>
        <form v-if="editing" @submit.prevent="save()">
          <fieldset :disabled="busy">
            <legend>{{ selected ? statusNames[selected.status] : '新草稿' }}{{ dirty ? ' · 未保存' : '' }}</legend>
            <p v-if="selected?.status === 'published'">这篇文章已发布到数据库。保存仅更新修订草稿；确认发布后更新公开内容，静态页面以更新状态为准。</p>
            <p v-if="selected?.revision" role="status">正在编辑待发布修订 · 最近保存 {{ new Date(selected.revision.updatedAt).toLocaleString('zh-CN') }}</p>
            <p v-if="selected?.status === 'archived'">这篇文章已归档，公开 API 已隐藏。静态更新完成前，旧页面仍可能公开访问；确认发布可恢复数据库公开版本。</p>
            <label>标题<input v-model="form.title" required maxlength="160" /></label>
            <label>slug · 公开地址<input v-model="form.slug" required maxlength="100" pattern="[a-z0-9]+(-[a-z0-9]+)*" :readonly="!!selected" placeholder="my-first-post" /><small>仅小写英文、数字和连字符；创建后不可修改。</small></label>
            <label>摘要<textarea v-model="form.summary" maxlength="500" rows="3" /></label>
            <label>分类<select v-model="form.categoryId"><option :value="null">未分类</option><option v-for="term in categories" :key="term.id" :value="term.id">{{ term.name }}</option></select></label>
            <div class="admin-tags"><span>标签</span><label v-for="tag in tags" :key="tag.id"><input v-model="form.tagIds" type="checkbox" :value="tag.id" />{{ tag.name }}</label><small v-if="!tags.length">暂无标签，可直接保存和发布。</small></div>
            <label>Markdown 正文<textarea v-model="form.contentMarkdown" rows="18" class="admin-markdown" spellcheck="false" /></label>
            <div class="admin-actions"><button type="submit" :disabled="conflict && !!selected">{{ busy ? '处理中…' : selected?.status === 'published' || selected?.revision ? '保存修订草稿' : '保存文章' }}</button><button type="button" :disabled="conflict && !!selected" @click="save(true)">{{ selected?.status === 'archived' ? '确认重新发布' : '确认发布' }}</button><button v-if="selected && selected.status !== 'archived'" type="button" :disabled="dirty || conflict" @click="archive">归档文章</button><button v-if="conflict && selected" type="button" @click="reload">重新读取服务器版本</button></div>
            <small v-if="selected && dirty">归档前请先保存当前修改。</small>
            <RouterLink v-if="selected?.status === 'published'" :to="`/blog/${selected.slug}`">公开访问：/blog/{{ selected.slug }} ↗</RouterLink>
          </fieldset>
          <details open><summary>正文预览</summary><div class="markdown-body admin-preview" v-html="preview" /></details>
        </form>
        <p v-else>选择文章继续编辑，或新建一篇草稿。</p>
      </div>
      <details ref="termPanel" class="term-manager">
        <summary>分类与标签管理 · {{ categories.length }} 个分类 / {{ tags.length }} 个标签</summary>
        <p>名称和 slug 的修改会立即影响公开分类标签；修改 slug 后原筛选链接将不再匹配。已被文章或修订草稿使用的项目不能删除。</p>
        <div class="admin-actions"><button :disabled="busy" :aria-pressed="termKind === 'categories'" @click="editTerm(undefined, 'categories')">管理分类</button><button :disabled="busy" :aria-pressed="termKind === 'tags'" @click="editTerm(undefined, 'tags')">管理标签</button><button :disabled="busy" @click="editTerm()">新增{{ termLabel }}</button></div>
        <ul class="term-list"><li v-for="term in termItems" :key="term.id"><span><strong>{{ term.name }}</strong> <small>{{ term.slug }}</small></span><button :disabled="busy" @click="editTerm(term)">编辑</button><button :disabled="busy" @click="deleteTerm(term)">删除</button></li></ul>
        <p v-if="!termItems.length">暂无{{ termLabel }}。</p>
        <form v-if="termEditing" @submit.prevent="saveTerm">
          <fieldset :disabled="busy"><legend>{{ termId ? '编辑' : '新增' }}{{ termLabel }}{{ termDirty ? ' · 未保存' : '' }}</legend>
            <label>名称<input v-model="termForm.name" required maxlength="160" /></label>
            <label>slug<input v-model="termForm.slug" required maxlength="100" pattern="[a-z0-9]+(-[a-z0-9]+)*" placeholder="development" /></label>
            <div class="admin-actions"><button type="submit">保存{{ termLabel }}</button><button type="button" @click="cancelTerm">取消编辑</button></div>
          </fieldset>
        </form>
      </details>
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
.term-manager { border-top: 1px solid #777; padding-top: 24px; }
.term-manager summary { cursor: pointer; font-weight: bold; }
.term-list { list-style: none; padding: 0; }
.term-list li { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; padding: 12px 0; border-bottom: 1px solid #aaa; }
.term-list span { flex: 1; min-width: 150px; overflow-wrap: anywhere; }
.term-manager form { max-width: 600px; margin-top: 24px; }
@media (max-width: 760px) { .admin-layout { grid-template-columns: 1fr; } }
</style>
