<script setup lang="ts">
import SiteIcon from '../components/SiteIcon.vue'
import { computed, ref, watch, onMounted } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { TechButton, TechTabs, MotionReveal } from '@field-lab/vue'
import { blogApi, type PublicPost, type Term } from '../blogApi'
const route = useRoute()
const router = useRouter()
const posts = ref<PublicPost[]>([])
const categories = ref<Term[]>([])
const total = ref(0)
const loading = ref(true)
const error = ref('')
const termError = ref('')
const retry = ref(0)
const tabs = computed(() => [{ id: 'all', label: '全部文章' }, ...categories.value.map(t => ({ id: t.slug, label: t.name }))])
const category = computed({
  get: () => typeof route.query.category === 'string' ? route.query.category : 'all',
  set: value => { void router.replace({ query: { ...route.query, page: undefined, category: value === 'all' ? undefined : value } }) },
})
const query = computed({
  get: () => typeof route.query.q === 'string' ? route.query.q : '',
  set: value => { void router.replace({ query: { ...route.query, page: undefined, q: value || undefined } }) },
})
const page = computed(() => Math.max(1, Number(route.query.page) || 1))
function goPage(value: number) { void router.replace({ query: { ...route.query, page: value } }) }
function resetFilters() { void router.replace({ query: {} }) }
async function loadCategories() {
  termError.value = ''
  try { categories.value = await blogApi.terms('categories') }
  catch { termError.value = '分类暂时无法加载。' }
}
onMounted(loadCategories)
watch(() => [route.query.q, route.query.category, route.query.tag, route.query.page, retry.value], (_, __, cleanup) => {
  const controller = new AbortController()
  const params = new URLSearchParams({ page: String(page.value), pageSize: '20' })
  for (const key of ['q', 'category', 'tag']) {
    const value = route.query[key]
    if (typeof value === 'string' && value && !(key === 'category' && value === 'all')) params.set(key, value)
  }
  loading.value = true; error.value = ''; posts.value = []; total.value = 0
  const timer = setTimeout(async () => {
    try {
      const result = await blogApi.posts(params, controller.signal)
      if (!controller.signal.aborted) { posts.value = result.data; total.value = result.pagination.total }
    } catch (e) { if (!controller.signal.aborted) error.value = e instanceof Error ? e.message : '文章加载失败。' }
    finally { if (!controller.signal.aborted) loading.value = false }
  }, 200)
  cleanup(() => { clearTimeout(timer); controller.abort() })
}, { immediate: true })
</script>
<template>
      <section id="journal" class="content-section">
        <MotionReveal
          ><div class="section-header">
            <div>
              <div class="eyebrow">01 / THE JOURNAL</div>
              <h1 class="section-title">文字，留下思考的痕迹<SiteIcon name="arrow" /></h1>
            </div>
            <span class="section-note"
              >开发笔记与生活切片<br />共 {{ total }} 篇文章</span
            >
          </div></MotionReveal
        >
        <div class="journal-layout">
          <div class="journal-main">
            <label class="search f-input-shell"
              ><span>⌕</span
              ><input
                v-model="query"
                type="search"
                placeholder="搜索标题或摘要"
                aria-label="搜索文章"
              /><span class="search-hint">SEARCH</span></label
            >
            <p v-if="termError" role="status">{{ termError }} <button @click="loadCategories">重试分类</button></p>
            <TechTabs v-model="category" :items="tabs" label="文章分类">
              <div class="post-list" aria-live="polite" :aria-busy="loading">
                <p v-if="loading">正在加载文章…</p>
                <div v-else-if="error" role="alert">{{ error }} <button @click="retry++">重试</button></div>
                <article
                  v-for="(post, index) in posts"
                  :key="post.id"
                  class="post-row"
                >
                  <span class="post-number"
                    >{{ String((page - 1) * 20 + index + 1).padStart(2, '0') }}</span
                  >
                  <div class="post-content">
                    <div class="post-meta">
                      {{ post.category?.name || '未分类' }} <span>/</span> {{ new Date(post.publishedAt).toLocaleDateString('zh-CN')
                      }}<span
                        v-if="index === 0 && category === 'all' && !query"
                        class="new-label"
                        >LATEST</span
                      >
                    </div>
                    <h3>
                      <RouterLink :to="`/blog/${post.slug}`">{{ post.title }}</RouterLink>
                    </h3>
                    <p>{{ post.summary }}</p>
                    <div class="tags">
                      <span v-for="tag in post.tags" :key="tag.id"
                        ># {{ tag.name }}</span
                      >
                    </div>
                  </div>
                  <RouterLink class="post-open"
                    :aria-label="`阅读：${post.title}`"
                    :to="`/blog/${post.slug}`"
                  >
                    ↗</RouterLink>
                </article>
                <div v-if="!loading && !error && !posts.length" class="empty-state">
                  <h3>暂时没有找到相关文字</h3>
                  <p>换个关键词，或看看全部文章。</p>
                  <TechButton
                    variant="outline"
                    @click="
                      resetFilters()
                    "
                    >重置筛选</TechButton
                  >
                </div>
              </div>
            </TechTabs>
            <nav v-if="!loading && !error && total > 20" aria-label="文章分页" class="blog-pagination">
              <button :disabled="page <= 1" @click="goPage(page - 1)">上一页</button>
              <span>{{ page }} / {{ Math.ceil(total / 20) }}</span>
              <button :disabled="page * 20 >= total" @click="goPage(page + 1)">下一页</button>
            </nav>
          </div>
          <aside class="journal-aside">
            <div class="aside-label">TRANSMISSION / 001 <SiteIcon name="arrow" /></div>
            <div class="mini-emblem">L<SiteIcon name="arrow" /></div>
            <h3>一个持续更新的<br />个人实验场。</h3>
            <p>记录从不熟悉到慢慢理解的过程，也分享屏幕之外的生活。</p>
            <RouterLink to="/">返回首页 <SiteIcon name="arrow" /></RouterLink>
            <div class="aside-status">
              <span class="status-dot" /> 当前状态
              <strong>学习中，构建中</strong>
            </div>
            <div class="aside-bottom">LESS NOISE.<br />MORE CURIOSITY.</div>
          </aside>
        </div>
      </section>
</template>

