import { createRouter, createWebHistory } from 'vue-router'
import HomeView from './views/HomeView.vue'
import BlogView from './views/BlogView.vue'
import MusicView from './views/MusicView.vue'
import ArticleView from './views/ArticleView.vue'
import NotFoundView from './views/NotFoundView.vue'

export const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', component: HomeView, meta: { title: '首页' } },
    { path: '/blog', component: BlogView, meta: { title: '博客' } },
    { path: '/blog/:id', component: ArticleView, meta: { title: '文章' } },
    { path: '/music', component: MusicView, meta: { title: '音乐' } },
    { path: '/:pathMatch(.*)*', component: NotFoundView, meta: { title: '页面不存在' } },
  ],
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) return savedPosition
    if (to.path === from.path) return false
    return { top: 0, behavior: 'instant' }
  },
})
