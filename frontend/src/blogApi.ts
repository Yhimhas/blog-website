export interface Term { id: string; slug: string; name: string }
export interface PublicPost {
  id: string; slug: string; title: string; summary: string
  category: Term | null; tags: Term[]; publishedAt: string; updatedAt: string
}
export interface PostDetail extends PublicPost { contentMarkdown: string }
export interface PostInput {
  slug: string; title: string; summary: string; contentMarkdown: string
  categoryId: string | null; tagIds: string[]
}
export interface AdminPost extends PostInput {
  id: string; status: 'draft' | 'published' | 'archived'; version: number
  publishedAt: string | null; createdAt: string; updatedAt: string
}
export interface Page<T> { data: T[]; pagination: { page: number; pageSize: number; total: number } }
export class ApiError extends Error {
  status: number
  code: string
  constructor(status: number, code: string, message: string) {
    super(message); this.status = status; this.code = code
  }
}
let csrf = ''
async function request<T>(path: string, method = 'GET', body?: unknown, signal?: AbortSignal): Promise<T> {
  let response: Response
  try {
    response = await fetch(`/api/v1${path}`, {
      method, credentials: 'same-origin', cache: 'no-store',
      signal: signal ? AbortSignal.any([signal, AbortSignal.timeout(15000)]) : AbortSignal.timeout(15000),
      headers: { Accept: 'application/json', ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
        ...(method !== 'GET' && csrf ? { 'X-CSRF-Token': csrf } : {}) },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch (error) {
    if (signal?.aborted) throw error
    throw new ApiError(0, 'NETWORK_ERROR', '无法连接博客服务，请稍后重试。')
  }
  if (response.status === 204) return undefined as T
  const payload = await response.json().catch(() => null)
  if (!response.ok) {
    if (response.status === 401) csrf = ''
    throw new ApiError(response.status, payload?.error?.code || 'HTTP_ERROR', payload?.error?.message || '博客服务暂时不可用。')
  }
  if (!payload || typeof payload !== 'object' || !('data' in payload)) throw new ApiError(502, 'INVALID_RESPONSE', '博客服务返回了无效数据。')
  return payload as T
}
export const blogApi = {
  posts: (query: URLSearchParams, signal?: AbortSignal) => request<Page<PublicPost>>(`/posts?${query}`, 'GET', undefined, signal),
  post: async (slug: string, signal?: AbortSignal) => (await request<{ data: PostDetail }>(`/posts/${encodeURIComponent(slug)}`, 'GET', undefined, signal)).data,
  terms: async (kind: 'categories' | 'tags') => (await request<{ data: Term[] }>(`/${kind}`)).data,
  async session() {
    const result = await request<{ data: { user: { username: string }; csrfToken: string } }>('/admin/session')
    csrf = result.data.csrfToken
    return result.data
  },
  async login(username: string, password: string) {
    csrf = ''
    await request('/admin/session', 'POST', { username, password })
    return this.session()
  },
  async logout() { await request('/admin/session/logout', 'POST', {}); csrf = '' },
  adminPosts: (page: number) => request<Page<AdminPost>>(`/admin/posts?page=${page}&pageSize=20`),
  adminPost: async (id: string) => (await request<{ data: AdminPost }>(`/admin/posts/${encodeURIComponent(id)}`)).data,
  create: async (input: PostInput) => (await request<{ data: AdminPost }>('/admin/posts', 'POST', input)).data,
  update: async (id: string, version: number, input: Omit<PostInput, 'slug'>) =>
    (await request<{ data: AdminPost }>(`/admin/posts/${encodeURIComponent(id)}`, 'PATCH', { ...input, version })).data,
  publish: async (id: string, version: number) =>
    (await request<{ data: AdminPost }>(`/admin/posts/${encodeURIComponent(id)}/publish`, 'POST', { version })).data,
}
