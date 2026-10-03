<script setup lang="ts">
import { RouterLink } from 'vue-router';
import { useMusicPlayer } from '../useMusicPlayer';
import MusicSeek from './MusicSeek.vue';
const player = useMusicPlayer();
const { track, state, message, capability } = player;
</script>
<template>
  <aside v-if="track && state !== 'stopped'" class="mini-music" aria-label="迷你播放器">
    <div class="mini-music__heading">
      <RouterLink to="/music" class="mini-music__title">{{ track.title }} <small>{{ track.artist }}</small></RouterLink>
      <button v-if="state === 'error'" @click="player.play(track, player.queue.value)">重新播放</button>
      <button v-else :disabled="state === 'preparing'" @click="player.toggle()">{{ state === 'playing' || state === 'buffering' ? '暂停' : '播放' }}</button>
      <button @click="player.stop()" aria-label="停止播放并关闭迷你播放器">停止</button>
    </div>
    <MusicSeek />
    <div class="mini-music__status" role="status">{{ message }} · {{ capability.mediaKind === 'full' ? '完整音源' : capability.mediaKind === 'preview' ? '试听片段' : '完整性未确认' }} <a :href="track.url" target="_blank" rel="noopener noreferrer">原站 ↗</a></div>
  </aside>
</template>
<style scoped>
.mini-music { position:fixed; right:20px; bottom:20px; z-index:80; width:min(440px, calc(100vw - 40px)); padding:16px; box-sizing:border-box; background:#171b19; color:#f2f4e9; border:1px solid #67744d; box-shadow:0 8px 32px #0005; }
.mini-music__heading { display:flex; align-items:center; gap:10px; margin-bottom:12px; }
.mini-music__title { flex:1; min-width:0; overflow:hidden; white-space:nowrap; text-overflow:ellipsis; color:inherit; }
.mini-music__title small { display:block; opacity:.7; overflow:hidden; text-overflow:ellipsis; }
button { padding:6px 10px; background:#d4ef37; color:#171b19; border:0; cursor:pointer; }
button:disabled { opacity:.5; }
.mini-music__status { margin-top:8px; font-size:12px; }
.mini-music__status a { color:inherit; }
@media(max-width:600px) { .mini-music { right:12px; bottom:12px; width:calc(100vw - 24px); } }
</style>
