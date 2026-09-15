<script setup lang="ts">
import { ref } from "vue";
import type { Ref } from "vue";
const introduction: Ref<string> = ref("blog and music");
</script>

<template>
  <main>
    <div class="box">
      <button class="but" @click="">显示</button>
      <button class="but" @click="">收起</button>
    </div>
  </main>
</template>

<style>
.but {
  color: aliceblue;
}

.box {
  display: flex;
}
</style>
