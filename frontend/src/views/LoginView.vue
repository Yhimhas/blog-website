<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { authSession } from '../authSession'

const route = useRoute()
const router = useRouter()
const { user, isAdmin } = authSession
const username = ref('')
const password = ref('')
const busy = ref(false)
const checking = ref(true)
const error = ref('')
onMounted(async () => {
  try { await authSession.restore() }
  catch { error.value = '暂时无法确认登录状态，请稍后重试。' }
  finally { checking.value = false }
})
async function login() {
  if (busy.value) return
  busy.value = true; error.value = ''
  try {
    await authSession.login(username.value, password.value)
    const next = route.query.next
    await router.replace(isAdmin.value ? (next === '/admin/local' ? next : '/admin') : '/blog')
  } catch (e) { error.value = e instanceof Error ? e.message : '登录失败，请重试。' }
  finally { password.value = ''; busy.value = false }
}
async function logout() {
  busy.value = true; error.value = ''
  try { await authSession.logout() }
  catch (e) { error.value = e instanceof Error ? e.message : '退出失败，请重试。' }
  finally { busy.value = false }
}
</script>

<template>
  <section class="account-page content-section">
    <div class="eyebrow">ACCOUNT</div>
    <h1 class="section-title">{{ user ? '我的账号' : '用户登录' }}</h1>
    <p v-if="checking" role="status">正在检查登录状态…</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <template v-if="!checking && user">
      <p>你好，{{ user.username }}。</p>
      <p>{{ isAdmin ? '管理员账号' : '普通用户账号' }}</p>
      <RouterLink v-if="isAdmin" to="/admin">文章管理 ↗</RouterLink>
      <p v-else-if="route.query.next">当前账号没有文章管理权限。</p>
      <button :disabled="busy" @click="logout">退出登录</button>
    </template>
    <form v-else-if="!checking" @submit.prevent="login">
      <label>用户名<input v-model="username" autocomplete="username" required :disabled="busy" /></label>
      <label>密码<input v-model="password" type="password" autocomplete="current-password" required :disabled="busy" /></label>
      <button :disabled="busy">{{ busy ? '登录中…' : '登录' }}</button>
    </form>
  </section>
</template>

<style scoped>
.account-page { max-width: 560px; margin: auto; }
form, label { display: grid; gap: 12px; }
form { gap: 24px; margin-top: 32px; }
input { width: 100%; box-sizing: border-box; padding: 14px; border: 1px solid #73786d; background: #fff; color: #171a13; font: inherit; }
button { display: block; margin-top: 20px; padding: 12px 24px; border: 1px solid #53594b; background: #d4ef37; color: #171a13; cursor: pointer; }
button:disabled { opacity: .6; cursor: wait; }
[role="alert"] { color: #a12c24; }
</style>
