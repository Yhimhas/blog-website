import { computed, ref } from 'vue'
import { ApiError, blogApi, type SessionUser } from './blogApi'

const user = ref<SessionUser | null>(null)
const isAdmin = computed(() => user.value?.role === 'admin')
let revision = 0
async function restore() {
  const current = ++revision
  try {
    const session = await blogApi.session()
    if (current === revision) user.value = session.user
  }
  catch (error) {
    if (current === revision) user.value = null
    if (!(error instanceof ApiError && error.status === 401)) throw error
  }
}
async function login(username: string, password: string) {
  ++revision
  user.value = null
  user.value = (await blogApi.login(username, password)).user
}
async function logout() {
  ++revision
  await blogApi.logout()
  user.value = null
}
export const authSession = { user, isAdmin, restore, login, logout }
