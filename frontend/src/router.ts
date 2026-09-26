import { createRouter, createWebHistory } from 'vue-router'
import HomeView from './views/HomeView.vue'
import BlogView from './views/BlogView.vue'
import MusicView from './views/MusicView.vue'
import ArticleView from './views/ArticleView.vue'
import NotFoundView from './views/NotFoundView.vue'
import { waitForPage } from './pageTransition'

export const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', component: HomeView, meta: { title: '首页' } },
    { path: '/blog', component: BlogView, meta: { title: '博客' } },
    { path: '/blog/:id', component: ArticleView, props: true, meta: { title: '文章' } },
    { path: '/admin', component: () => import('./views/AdminView.vue'), meta: { title: '文章管理' } },
    { path: '/music', component: MusicView, meta: { title: '音乐' } },
    { path: '/:pathMatch(.*)*', component: NotFoundView, meta: { title: '页面不存在' } },
  ],
  async scrollBehavior(to, from, savedPosition) {
    if (to.path === from.path) return savedPosition ? { ...savedPosition, behavior: 'instant' } : false
    if (from.matched.length) await waitForPage(to.path)
    // Ignore scroll requests superseded by rapid navigation.
    if (router.currentRoute.value.fullPath !== to.fullPath) return false
    return savedPosition ? { ...savedPosition, behavior: 'instant' } : { top: 0, behavior: 'instant' }
  },
})
