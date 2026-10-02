<script setup lang="ts">
import { ref } from 'vue';
import { useMusicPlayer } from '../useMusicPlayer';
const player = useMusicPlayer();
const { currentTime, duration, canSeek } = player;
const draft = ref<number>();
function label(seconds: number) { return `${Math.floor(seconds / 60)}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`; }
function commit(event: Event) {
  const target = Number((event.target as HTMLInputElement).value);
  draft.value = undefined;
  void player.seek(target);
}
</script>
<template>
  <div class="music-seek">
    <span>{{ label(draft ?? currentTime) }} / {{ duration ? label(duration) : '--:--' }}</span>
    <input type="range" min="0" :max="Math.max(0, duration - 0.1)" step="0.1"
      :value="draft ?? currentTime" :disabled="!canSeek" aria-label="播放进度"
      :aria-valuetext="label(draft ?? currentTime)" :title="canSeek ? '拖动以跳转播放位置' : '音源准备中或尚无可定位时长'"
      @input="draft = Number(($event.target as HTMLInputElement).value)" @change="commit" @pointercancel="draft = undefined" @blur="draft = undefined" />
  </div>
</template>
<style scoped>
.music-seek { display:flex; align-items:center; gap:12px; flex:1; min-width:0; font-size:12px; }
.music-seek span { white-space:nowrap; font-variant-numeric:tabular-nums; }
.music-seek input { flex:1; min-width:40px; width:100%; accent-color:currentColor; cursor:pointer; }
.music-seek input:disabled { cursor:default; opacity:.45; }
</style>
