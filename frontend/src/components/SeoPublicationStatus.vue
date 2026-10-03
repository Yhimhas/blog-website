<script setup lang="ts">
import type { SeoPublicationState } from '../useSeoPublication'

defineProps<{ state: SeoPublicationState; loading: boolean }>()
defineEmits<{ refresh: [] }>()
</script>

<template>
  <section class="seo-publication" :data-status="state.status" aria-label="静态页面更新状态">
    <div :role="state.status === 'failed' ? 'alert' : 'status'">
      <template v-if="state.status === 'current'">
        <strong>静态页面已更新</strong>
        <p>当前公开内容的静态版本已确认发布。</p>
      </template>
      <template v-else-if="state.status === 'failed'">
        <strong>上次静态页面更新失败</strong>
        <p>请检查自动发布任务。更新完成前，新文章可能直接访问 404，归档文章的旧页面仍可能公开返回。</p>
      </template>
      <template v-else-if="state.status === 'pending'">
        <strong>静态页面尚未更新</strong>
        <p>数据库内容已更新，正在等待静态版本确认。新文章可能直接访问 404，归档文章的旧页面仍可能公开返回。</p>
        <p v-if="state.data?.lastAttemptAt === null">发布任务尚无执行记录，请确认自动刷新服务已启用。</p>
      </template>
      <template v-else-if="state.status === 'unknown'">
        <strong>无法确认静态页面是否更新</strong>
        <p>状态获取失败，请刷新状态或检查后台服务。尚不能确认新文章可直接访问，或归档文章的旧页面已撤回。</p>
      </template>
      <template v-else>
        <strong>正在检查静态页面状态…</strong>
        <p>文章操作与静态页面更新分别确认。</p>
      </template>
      <p v-if="state.data?.publishedAt">最近完成更新：<time :datetime="state.data.publishedAt">{{ new Date(state.data.publishedAt).toLocaleString('zh-CN') }}</time></p>
    </div>
    <button type="button" :disabled="loading" @click="$emit('refresh')">{{ loading ? '正在检查…' : '刷新静态状态' }}</button>
  </section>
</template>

<style scoped>
.seo-publication { display: flex; flex-wrap: wrap; align-items: flex-start; gap: 16px; border: 1px solid #777; border-left: 4px solid #879a16; padding: 16px; margin: 24px 0; }
.seo-publication > div { flex: 1; min-width: min(100%, 260px); }
.seo-publication[data-status=pending], .seo-publication[data-status=unknown] { border-left-color: #b17b25; }
.seo-publication[data-status=failed] { border-left-color: #b13b3b; }
.seo-publication p { margin: 8px 0 0; line-height: 1.6; }
.seo-publication button { border: 1px solid #53594b; background: #d4ef37; color: #171a13; padding: 10px 16px; cursor: pointer; }
.seo-publication button:disabled { opacity: .5; cursor: not-allowed; }
</style>
