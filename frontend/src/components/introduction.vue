<script setup lang="ts">
defineProps<{
  msg: string;
  visible: boolean;
}>();

const emit = defineEmits<{
  "update:visible": [value: boolean];
}>();
</script>

<template>
  <section aria-labelledby="introduction-heading">
    <h2 id="introduction-heading">网站简介</h2>
    <div class="box">
      <button class="but" type="button" aria-controls="site-introduction"
        :aria-expanded="visible" :disabled="visible" @click="emit('update:visible', true)">显示</button>
      <button class="but" type="button" aria-controls="site-introduction"
        :aria-expanded="visible" :disabled="!visible" @click="emit('update:visible', false)">收起</button>
    </div>
    <p id="site-introduction" v-show="visible">{{ msg }}</p>
  </section>
</template>

<style scoped>
h2 {
  color: var(--color-heading);
  font-weight: 600;
}

.but {
  padding: 0.5rem 1rem;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
  background: var(--color-background-soft);
  color: var(--color-heading);
  font: inherit;
  cursor: pointer;
}

.but:disabled {
  cursor: default;
  opacity: 0.6;
}

.but:focus-visible {
  outline: 2px solid var(--color-heading);
  outline-offset: 3px;
}

.box {
  display: flex;
  gap: 0.75rem;
  margin: 1rem 0;
}

p {
  color: var(--color-heading);
  line-height: 1.8;
  overflow-wrap: anywhere;
}
</style>
