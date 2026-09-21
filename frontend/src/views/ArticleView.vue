<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { posts } from '../content'
// Route props retain the outgoing article while its leave animation finishes.
const props = defineProps<{ id: string }>()
const post = computed(() => posts.find(item => item.id === props.id))
</script>

<template>
  <article v-if="post" class="article-page">
    <nav class="article-breadcrumb" aria-label="面包屑"><RouterLink to="/blog">博客</RouterLink><span aria-hidden="true"> / </span><span>{{ post.category }}</span></nav>
    <header class="article-header">
      <div class="eyebrow">JOURNAL / <time :datetime="post.published">{{ post.date }}</time></div>
      <h1>{{ post.title }}</h1>
      <div class="tags"><span v-for="tag in post.tags" :key="tag"># {{ tag }}</span></div>
    </header>
    <div class="reading-body">
      <p class="reading-intro">{{ post.summary }}</p>
      <div class="markdown-body" v-html="post.html" />
    </div>
    <div class="article-end"><span>— END OF NOTE —</span><RouterLink to="/blog" class="primary-link">← 返回文章列表</RouterLink></div>
  </article>
  <section v-else class="not-found"><div class="eyebrow">404 / JOURNAL</div><h1>这篇文章不存在</h1><p>文章地址可能有误，去列表看看其他文字吧。</p><RouterLink class="primary-link" to="/blog">返回文章列表 ↗</RouterLink></section>
</template>
