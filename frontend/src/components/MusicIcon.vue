<script setup lang="ts">
defineProps<{ name: string }>()
const paths: Record<string, string> = {
  search: 'm21 21-5-5 M19 10.5a8.5 8.5 0 1 1-17 0 8.5 8.5 0 0 1 17 0',
  heart: 'M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21l8.8-8.6a5.5 5.5 0 0 0 0-7.8Z',
  play: 'm9 5 11 7-11 7Z', pause: 'M8 5v14 M16 5v14',
  previous: 'M5 5v14 M19 5 7 12l12 7Z', next: 'M19 5v14 M5 5l12 7-12 7Z',
  volume: 'm11 4-6 5H2v6h3l6 5Z M15 8a6 6 0 0 1 0 8 M18 5a10 10 0 0 1 0 14',
  mute: 'm11 4-6 5H2v6h3l6 5Z M16 9l6 6 M22 9l-6 6',
  shuffle: 'M3 6h3c5 0 7 12 12 12h3 M17 14l4 4-4 4 M3 18h3c2 0 4-3 6-6s4-6 6-6h3 M17 2l4 4-4 4',
  more: 'M4 12h.01 M12 12h.01 M20 12h.01',
  upload: 'M12 16V3 M7 8l5-5 5 5 M4 16v5h16v-5',
  close: 'm6 6 12 12 M18 6 6 18', back: 'M20 12H4 M10 6l-6 6 6 6',
  note: 'M9 18V5l12-2v13 M9 8l12-2 M9 18a3 3 0 1 1-3-3h3 M21 16a3 3 0 1 1-3-3h3',
}
</script>
<template><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path :d="paths[name] || paths.note" /></svg></template>
