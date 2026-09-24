import { computed, onScopeDispose, ref, watch, type Ref } from 'vue'
import { dailySelection } from './musicDaily.ts'
import { fetchRecommendations, type DailyRecommendations, type RecommendationTrack } from './musicRecommendations.ts'

type RecommendationState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'empty' | 'ready'; data: DailyRecommendations }

export function useMusicRecommendations(options: {
  mode: 'api' | 'local'
  day: Ref<string>
  localTracks: RecommendationTrack[]
  request?: typeof fetchRecommendations
  timeoutMs?: number
}) {
  const state = ref<RecommendationState>({ status: 'loading' })
  let controller: AbortController | undefined
  let generation = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  const localFallback = options.mode === 'local'

  async function reload() {
    const version = ++generation
    controller?.abort()
    clearTimeout(timer)
    if (localFallback) {
      const items = dailySelection(options.localTracks, options.day.value)
      state.value = { status: items.length ? 'ready' : 'empty', data: { date: options.day.value, timezone: 'Asia/Shanghai', items } }
      return
    }
    state.value = { status: 'loading' }
    const active = new AbortController()
    controller = active
    timer = setTimeout(() => {
      if (version !== generation) return
      ++generation
      active.abort()
      state.value = { status: 'error', message: '推荐请求超时，请稍后重试。' }
    }, options.timeoutMs ?? 10_000)
    try {
      const data = await (options.request ?? fetchRecommendations)(active.signal)
      if (version !== generation) return
      state.value = { status: data.items.length ? 'ready' : 'empty', data }
    } catch {
      if (version !== generation) return
      state.value = { status: 'error', message: '每日推荐加载失败，请稍后重试。' }
    } finally {
      if (version === generation) clearTimeout(timer)
    }
  }

  watch(options.day, reload, { immediate: true })
  onScopeDispose(() => {
    ++generation
    controller?.abort()
    clearTimeout(timer)
  })
  const recommendations = computed(() => 'data' in state.value ? state.value.data.items : [])
  const recommendationDate = computed(() => 'data' in state.value ? state.value.data.date : undefined)
  return { state, recommendations, recommendationDate, localFallback, reload }
}
