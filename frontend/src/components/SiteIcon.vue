<script setup lang="ts">
withDefaults(defineProps<{ name: string }>(), { name: 'arrow' })
const assets: Record<string, string> = {
  wave: 'layers', layers: 'wave', back: 'direction', arrow: 'direction', up: 'direction',
  sparkle: 'sparkle',
}
</script>

<template>
  <span class="site-icon" :class="`site-icon--${name}`" aria-hidden="true"
    :style="{ maskImage: `url(/icons/ignoredone/${assets[name] || 'direction'}.png)` }" />
</template>

<style scoped>
.site-icon { display: inline-block; width: 1em; height: 1em; flex: 0 0 auto; vertical-align: -0.12em; background: currentColor; mask-repeat: no-repeat; mask-position: center; mask-size: contain; }
.site-icon--arrow { transform: rotate(135deg); }
.site-icon--up { transform: rotate(90deg); }
</style>
