<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'

const dialog = ref<HTMLDialogElement>()
const video = ref<HTMLVideoElement>()
const leaving = ref(false)
const visible = ref(false)
const reducedMotion = ref(false)
const mobile = ref(false)
const storageKey = 'yhimhas:intro:v1'
const base = import.meta.env.BASE_URL
const mediaPath = computed(() => `${base}intro-p3re${mobile.value ? '-mobile' : ''}`)
let exitTimer: ReturnType<typeof setTimeout> | undefined
let previousOverflow = ''
let locked = false
let motion: MediaQueryList | undefined
let viewport: MediaQueryList | undefined

const fragments = Array.from({ length: 32 }, (_, index) => ({
  '--x': `${(index * 37) % 100}%`,
  '--y': `${(index * 23) % 100}%`,
  '--size': `${18 + (index * 13) % 65}px`,
  '--delay': `${(index % 7) * 45}ms`,
  '--drift': `${((index % 5) - 2) * 110}px`,
  '--turn': `${index * 47}deg`,
}))

function remember() {
  try { sessionStorage.setItem(storageKey, 'seen') } catch { /* Storage can be unavailable. */ }
}

function restoreScroll() {
  if (!locked) return
  document.body.style.overflow = previousOverflow
  locked = false
}

function close() {
  clearTimeout(exitTimer)
  video.value?.pause()
  dialog.value?.close()
  visible.value = false
  restoreScroll()
  document.getElementById('page-content')?.focus({ preventScroll: true })
}

function finish() {
  if (leaving.value || !visible.value) return
  leaving.value = true
  remember()
  if (motion?.matches) close()
  else exitTimer = setTimeout(close, 760)
}

function motionChanged() {
  reducedMotion.value = Boolean(motion?.matches)
  if (reducedMotion.value) video.value?.pause()
}

function viewportChanged() {
  mobile.value = Boolean(viewport?.matches)
}

onMounted(async () => {
  motion = window.matchMedia('(prefers-reduced-motion: reduce)')
  let seen = false
  try { seen = sessionStorage.getItem(storageKey) === 'seen' } catch { /* Continue without persistence. */ }
  const replay = new URLSearchParams(window.location.search).get('intro') === '1'
  if (seen && !replay) return
  viewport = window.matchMedia('(max-width: 600px)')
  viewportChanged()
  viewport.addEventListener('change', viewportChanged)
  reducedMotion.value = motion.matches
  visible.value = true
  await nextTick()
  previousOverflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
  locked = true
  dialog.value?.showModal()
  motion.addEventListener('change', motionChanged)
})

onBeforeUnmount(() => {
  clearTimeout(exitTimer)
  motion?.removeEventListener('change', motionChanged)
  viewport?.removeEventListener('change', viewportChanged)
  restoreScroll()
})
</script>

<template>
  <dialog ref="dialog" class="entry-intro" :class="{ 'is-leaving': leaving }"
    aria-label="Yhimhas 开屏动画" @cancel.prevent="finish">
    <template v-if="visible">
      <div class="entry-intro__scene">
        <img class="entry-intro__poster" :src="`${mediaPath}.jpg`" alt="" />
        <video v-if="!reducedMotion" :key="mediaPath" ref="video" class="entry-intro__video" :src="`${mediaPath}.mp4`"
          :poster="`${mediaPath}.jpg`" autoplay muted loop playsinline preload="auto"
          aria-hidden="true" />
        <div class="entry-intro__fragments" aria-hidden="true">
          <i v-for="(style, index) in fragments" :key="index" :style="style" />
        </div>
        <div class="entry-intro__identity">
          <span class="entry-intro__index">PERSONAL ARCHIVE / 01</span>
          <p>Yhimhas<span> / </span></p>
          <span class="entry-intro__caption">记录 · 探索 · 保持好奇</span>
        </div>
        <span class="entry-intro__signature" aria-hidden="true">NOTES / CODE / SOUND</span>
      </div>
      <button class="entry-intro__skip" autofocus @click="finish">进入首页 <span aria-hidden="true">↗</span></button>
    </template>
  </dialog>
</template>

<style scoped>
.entry-intro { position: fixed; inset: 0; width: 100%; max-width: none; height: 100%; height: 100dvh; max-height: none; margin: 0; padding: 0; border: 0; overflow: hidden; background: #fff; color: #fff; letter-spacing: 0; }
.entry-intro::backdrop { background: transparent; }
.entry-intro__scene { position: absolute; inset: 0; overflow: hidden; }
.entry-intro__video, .entry-intro__poster { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; object-position: 35% center; }
.entry-intro__identity { position: absolute; top: 49%; left: 56%; right: 5%; animation: identity-in 1s 250ms both; }
.entry-intro__index, .entry-intro__signature { font: 11px/1.6 Consolas, monospace; }
.entry-intro__identity p { font: 700 64px/1.2 Arial, sans-serif; margin: 16px 0 18px; }
.entry-intro__identity p span { color: #74faff; }
.entry-intro__caption { font-size: 14px; }
.entry-intro__signature { position: absolute; bottom: 36px; left: 36px; padding: 5px 9px; background: #001c64; }
.entry-intro__skip { position: absolute; right: 36px; bottom: 36px; min-height: 44px; padding: 10px 0 10px 16px; color: white; border: 0; border-bottom: 1px solid #ffffff80; background: #001c64; font-size: 13px; }
.entry-intro__skip span { display: inline-block; width: 40px; font-size: 21px; }
.entry-intro__skip:focus-visible { outline: 2px solid white; outline-offset: 6px; }
.entry-intro__fragments { position: absolute; inset: 0; pointer-events: none; }
.entry-intro__fragments i { position: absolute; left: var(--x); top: var(--y); width: var(--size); height: var(--size); background: #abffff; clip-path: polygon(25% 0, 78% 10%, 100% 48%, 69% 100%, 13% 83%, 0 32%); animation: fragment-in 1.3s var(--delay) both; }
.entry-intro__fragments i:nth-child(3n) { background: white; }
.entry-intro__fragments i:nth-child(3n + 1) { background: #12bcf0; }
.entry-intro.is-leaving { pointer-events: none; animation: intro-out 740ms cubic-bezier(.76, 0, .24, 1) forwards; }
@keyframes fragment-in { 0% { opacity: 0; transform: translate(var(--drift), 40vh) scale(3) rotate(var(--turn)); } 18% { opacity: .95; } 100% { opacity: 0; transform: translate(0, -38vh) scale(.15) rotate(160deg); } }
@keyframes identity-in { from { opacity: 0; transform: translateY(22px); } to { opacity: 1; transform: translateY(0); } }
@keyframes intro-out { from { clip-path: polygon(0 0, 160% 0, 100% 100%, 0 100%); } to { clip-path: polygon(0 0, 0 0, -60% 100%, 0 100%); } }
@media (max-width: 900px) { .entry-intro__identity p { font-size: 44px; } }
@media (max-width: 600px) {
  .entry-intro { background: #0008aa; }
  .entry-intro__video, .entry-intro__poster { inset: 0; height: 100%; object-fit: cover; object-position: center; }
  .entry-intro__identity { top: max(32px, env(safe-area-inset-top)); bottom: auto; left: 24px; right: 24px; padding: 20px 0; color: #001c64; }
  .entry-intro__identity p span { color: #007ca8; }
  .entry-intro__identity p { font-size: 42px; }
  .entry-intro__signature { bottom: 30px; left: 20px; font-size: 9px; }
  .entry-intro__skip { right: 20px; bottom: 25px; }
}
@media (max-width: 600px) and (max-height: 650px) { .entry-intro__identity { top: 16px; } .entry-intro__identity p { font-size: 36px; margin-block: 8px; } }
@media (prefers-reduced-motion: reduce) { .entry-intro, .entry-intro * { animation: none !important; } }
</style>
