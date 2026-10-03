import { onScopeDispose, ref } from 'vue'
import { ApiError, blogApi, type SeoPublication } from './blogApi.ts'

export interface SeoPublicationState {
  status: 'checking' | 'pending' | 'failed' | 'current' | 'unknown'
  data?: SeoPublication
}

export function useSeoPublication(options: {
  request?: (signal: AbortSignal) => Promise<SeoPublication>
  intervalMs?: number
  onUnauthorized?: (error: ApiError) => void
} = {}) {
  const state = ref<SeoPublicationState>({ status: 'checking' })
  const loading = ref(false)
  let running = false
  let disposed = false
  let generation = 0
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined

  function cancel() {
    ++generation
    controller?.abort()
    controller = undefined
    clearTimeout(timer)
    timer = undefined
    loading.value = false
  }

  async function refresh() {
    if (!running || disposed) return
    cancel()
    const current = generation
    const active = new AbortController()
    controller = active
    loading.value = true
    try {
      const data = await (options.request ?? blogApi.seoPublication)(active.signal)
      if (current !== generation || !running) return
      state.value = { status: data.lastError ? 'failed' : data.revision === data.appliedRevision ? 'current' : 'pending', data }
    } catch (error) {
      if (current !== generation || !running) return
      // A failed status request never means that the article mutation failed,
      // or that an older receipt still confirms the current static version.
      state.value = { status: 'unknown' }
      if (error instanceof ApiError && (error.status === 401 || error.status === 403)) options.onUnauthorized?.(error)
    } finally {
      if (current === generation && running) {
        loading.value = false
        controller = undefined
        timer = setTimeout(() => { void refresh() }, options.intervalMs ?? 5000)
      }
    }
  }

  async function start() {
    if (disposed || running) return
    running = true
    await refresh()
  }

  function stop() {
    running = false
    cancel()
    state.value = { status: 'checking' }
  }

  function markPending() {
    cancel()
    state.value = { status: 'pending' }
    // Invalidates any poll that started before the successful article write.
    // The caller immediately requests the new receipt, independently of list
    // refresh and without changing the successful article notice on failure.
  }

  onScopeDispose(() => { disposed = true; stop() })
  return { state, loading, start, stop, refresh, markPending }
}
