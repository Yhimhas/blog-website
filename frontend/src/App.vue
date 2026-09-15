<script setup lang="ts">
import introduction from "./components/introduction.vue";
import { ref } from "vue";
import type { Ref } from "vue";
const headmsg: Ref<string> = ref("");
</script>

<template>
  <header>
    <div class="wrapper">
      <h2>Welcome</h2>
    </div>
  </header>

  <main>
    <div>
      <introduction msg="headmsg" />
    </div>
  </main>
</template>

<style scoped>
header {
  line-height: 1.5;
}

@media (min-width: 1024px) {
  header {
    display: flex;
    place-items: center;
    padding-right: calc(var(--section-gap) / 2);
  }

  .logo {
    margin: 0 2rem 0 0;
  }

  header .wrapper {
    display: flex;
    place-items: flex-start;
    flex-wrap: wrap;
  }
}
</style>
