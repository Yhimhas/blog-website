<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { TechButton, TechTabs, MotionReveal } from '@field-lab/vue'
import { posts } from '../content'
const route = useRoute()
const router = useRouter()
const tabs = [{ id: 'all', label: '全部文章' }, ...['开发笔记', '生活随记', '音乐手记'].map(id => ({ id, label: id }))]
const category = computed({
  get: () => typeof route.query.category === 'string' && tabs.some(tab => tab.id === route.query.category) ? route.query.category : 'all',
  set: value => { void router.replace({ query: { ...route.query, category: value === 'all' ? undefined : value } }) },
})
const query = computed({
  get: () => typeof route.query.q === 'string' ? route.query.q : '',
  set: value => { void router.replace({ query: { ...route.query, q: value || undefined } }) },
})
const visiblePosts = computed(() => posts.filter(post =>
  (category.value === 'all' || post.category === category.value) &&
  [post.title, post.summary, ...post.tags].join(' ').toLowerCase().includes(query.value.trim().toLowerCase())
))
function resetFilters() { void router.replace({ query: {} }) }
</script>
<template>
      <section id="journal" class="content-section">
        <MotionReveal
          ><div class="section-header">
            <div>
              <div class="eyebrow">01 / THE JOURNAL</div>
              <h1 class="section-title">文字，留下思考的痕迹<span>↗</span></h1>
            </div>
            <span class="section-note"
              >开发笔记与生活切片<br />共 {{ posts.length }} 篇示例文章</span
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
                placeholder="搜索标题、内容或标签"
                aria-label="搜索文章"
              /><span class="search-hint">SEARCH</span></label
            >
            <TechTabs v-model="category" :items="tabs" label="文章分类">
              <div class="post-list" aria-live="polite">
                <article
                  v-for="(post, index) in visiblePosts"
                  :key="post.id"
                  class="post-row"
                >
                  <span class="post-number"
                    >0{{ posts.indexOf(post) + 1 }}</span
                  >
                  <div class="post-content">
                    <div class="post-meta">
                      {{ post.category }} <span>/</span> {{ post.date
                      }}<span
                        v-if="index === 0 && category === 'all' && !query"
                        class="new-label"
                        >LATEST</span
                      >
                    </div>
                    <h3>
                      <RouterLink :to="`/blog/${post.id}`">{{ post.title }}</RouterLink>
                    </h3>
                    <p>{{ post.summary }}</p>
                    <div class="tags">
                      <span v-for="tag in post.tags" :key="tag"
                        ># {{ tag }}</span
                      >
                    </div>
                  </div>
                  <RouterLink class="post-open"
                    :aria-label="`阅读：${post.title}`"
                    :to="`/blog/${post.id}`"
                  >
                    ↗</RouterLink>
                </article>
                <div v-if="!visiblePosts.length" class="empty-state">
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
          </div>
          <aside class="journal-aside">
            <div class="aside-label">TRANSMISSION / 001 <span>↗</span></div>
            <div class="mini-emblem">L<span>↗</span></div>
            <h3>一个持续更新的<br />个人实验场。</h3>
            <p>记录从不熟悉到慢慢理解的过程，也分享屏幕之外的生活。</p>
            <RouterLink to="/">返回首页 <span>↗</span></RouterLink>
            <div class="aside-status">
              <span class="status-dot" /> 当前状态
              <strong>学习中，构建中</strong>
            </div>
            <div class="aside-bottom">LESS NOISE.<br />MORE CURIOSITY.</div>
          </aside>
        </div>
      </section>
</template>

