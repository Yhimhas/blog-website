<script setup lang="ts">
import SiteIcon from '../components/SiteIcon.vue'
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { blogApi, ApiError, type PostDetail } from '../blogApi'
import { renderMarkdown } from '../markdown'
const props = defineProps<{ id: string }>()
const post = ref<PostDetail>()
const loading = ref(true)
const error = ref('')
const missing = ref(false)
const retry = ref(0)
const html = computed(() => renderMarkdown(post.value?.contentMarkdown || ''))
watch(() => [props.id, retry.value], async (_, __, cleanup) => {
  const controller = new AbortController()
  cleanup(() => controller.abort())
  loading.value = true; post.value = undefined; error.value = ''; missing.value = false
  try {
    const result = await blogApi.post(props.id, controller.signal)
    if (!controller.signal.aborted) { post.value = result; document.title = result.title + ' · Yhimhas / NOTES' }
  } catch (e) {
    if (!controller.signal.aborted) {
      missing.value = e instanceof ApiError && e.status === 404
      error.value = e instanceof Error ? e.message : '文章加载失败。'
    }
  } finally { if (!controller.signal.aborted) loading.value = false }
}, { immediate: true })
</script>

<template>
  <section v-if="loading" class="article-page" role="status">正在加载文章…</section>
  <section v-else-if="error && !missing" class="article-page" role="alert">{{ error }} <button @click="retry++">重试</button></section>
  <article v-else-if="post" class="article-page">
    <nav class="article-breadcrumb" aria-label="面包屑"><RouterLink to="/blog">博客</RouterLink><span aria-hidden="true"> / </span><span>{{ post.category?.name || '未分类' }}</span></nav>
    <header class="article-header">
      <div class="eyebrow">JOURNAL / <time :datetime="post.publishedAt">{{ new Date(post.publishedAt).toLocaleDateString('zh-CN') }}</time></div>
      <h1>{{ post.title }}</h1>
      <div class="tags"><span v-for="tag in post.tags" :key="tag.id"># {{ tag.name }}</span></div>
    </header>
    <div class="reading-body">
      <p class="reading-intro">{{ post.summary }}</p>
      <div class="markdown-body" v-html="html" />
    </div>
    <div class="article-end"><span>— END OF NOTE —</span><RouterLink to="/blog" class="primary-link"><SiteIcon name="back" /> 返回文章列表</RouterLink></div>
  </article>
  <section v-else class="not-found"><div class="eyebrow">404 / JOURNAL</div><h1>这篇文章不存在</h1><p>文章地址可能有误，去列表看看其他文字吧。</p><RouterLink class="primary-link" to="/blog">返回文章列表 <SiteIcon name="arrow" /></RouterLink></section>
</template>
