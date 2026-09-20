<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { RouterLink, RouterView, useRoute } from "vue-router";
import { FieldProvider } from "@field-lab/vue";
import { posts } from "./content";
import EntryIntro from "./components/EntryIntro.vue";
const route = useRoute();
const immersiveMusic = computed(() => route.path === "/music");
const paused = ref(false);
const expanded = ref(false);
const hovered = ref(false);
const navigationToggle = ref<HTMLButtonElement>();
// Adapted from FIELD UI 0.1.1 RailNav; RouterLink keeps page URLs intact.
// License: ../vendor/FIELD-UI-LICENSE
function enter(event: PointerEvent) {
  if (
    event.pointerType !== "mouse" ||
    !window.matchMedia("(min-width: 761px) and (hover: hover)").matches
  )
    return;
  hovered.value = true;
  expanded.value = true;
}
function leave(event: PointerEvent) {
  hovered.value = false;
  const nav = event.currentTarget as HTMLElement;
  if (!nav.querySelector(":focus-visible")) expanded.value = false;
}
function focusIn(event: FocusEvent) {
  if (
    (event.target as HTMLElement).matches(":focus-visible") &&
    window.matchMedia("(min-width: 761px)").matches
  )
    expanded.value = true;
}
function focusOut(event: FocusEvent) {
  if (
    !(event.currentTarget as HTMLElement).contains(
      event.relatedTarget as Node | null,
    ) &&
    !hovered.value
  )
    expanded.value = false;
}

function dismissNavigation() {
  // Keep keyboard focus on a visible control when the mobile links are hidden.
  navigationToggle.value?.focus({ preventScroll: true });
  expanded.value = false;
}

const content = ref<HTMLElement>();
const nav = [
  { path: "/", label: "首页", symbol: "⌂" },
  { path: "/blog", label: "博客", symbol: "≡" },
  { path: "/music", label: "音乐", symbol: "♫" },
];
const active = computed(() =>
  route.path.startsWith("/blog") ? "/blog" : route.path,
);
watch(
  () => route.fullPath,
  async (_, previous) => {
    expanded.value = false;
    const title = route.path.startsWith("/blog/")
      ? posts.find((post) => post.id === route.params.id)?.title || "文章不存在"
      : route.meta.title;
    document.title = `${title} · Yhimhas / NOTES`;
    if (previous && previous.split("?")[0] !== route.path) {
      await nextTick();
      content.value?.focus({ preventScroll: true });
    }
  },
  { immediate: true },
);
</script>

<template>
  <EntryIntro v-if="route.path === '/'" />
  <FieldProvider :duration="580" :paused="paused" accent="#d4ef37">
    <a class="skip-link" href="#page-content">跳转到内容</a>
    <nav
      v-if="!immersiveMusic"
      class="f-rail"
      :class="{ 'is-expanded': expanded }"
      aria-label="页面导航"
      @pointerenter="enter"
      @pointerleave="leave"
      @focusin="focusIn"
      @focusout="focusOut"
      @keydown.esc.prevent="dismissNavigation"
    >
      <RouterLink
        class="f-rail__brand"
        to="/"
        aria-label="返回首页"
        @click="expanded = false"
        ><img
          class="site-logo rail-logo"
          src="/yhimhas-logo.jpg"
          alt="Yhimhas"
          width="44"
          height="44"
      /></RouterLink>
      <button
        ref="navigationToggle"
        type="button"
        class="f-rail__toggle"
        :aria-expanded="expanded"
        aria-controls="page-navigation"
        aria-label="展开或收起导航"
        @click="expanded = !expanded"
      >
        {{ expanded ? "−" : "☰" }}
      </button>
      <div id="page-navigation" class="f-rail__items">
        <RouterLink
          v-for="item in nav"
          :key="item.path"
          :to="item.path"
          :aria-label="item.label"
          :aria-current="active === item.path ? 'page' : undefined"
          @click="expanded = false"
        >
          <span class="f-rail__icon" aria-hidden="true">{{ item.symbol }}</span
          ><span class="f-rail__label">{{ item.label }}</span>
        </RouterLink>
      </div>
      <div class="f-rail__bottom" aria-hidden="true">
        <span class="f-rail__barcode" /><span>FIELD / UI</span><span>V.01</span>
      </div>
    </nav>
    <div class="page site-layout" :class="{ 'music-layout': immersiveMusic }">
      <header v-if="!immersiveMusic" class="topbar">
        <RouterLink to="/" class="wordmark"
          ><img
            class="site-logo wordmark-logo"
            src="/yhimhas-logo.jpg"
            alt=""
            width="30"
            height="30"
          />Yhimhas<span> / </span></RouterLink
        >
        <span class="topbar-caption">记录 · 探索 · 保持好奇</span>
        <button
          class="motion-toggle"
          :aria-pressed="paused"
          @click="paused = !paused"
        >
          <span :class="['status-dot', { muted: paused }]" />{{
            paused ? "动态已暂停" : "动态开启"
          }}
        </button>
      </header>
      <main id="page-content" ref="content" tabindex="-1"><RouterView /></main>
      <footer v-if="!immersiveMusic">
        <RouterLink class="wordmark" to="/"
          ><img
            class="site-logo wordmark-logo"
            src="/yhimhas-logo.jpg"
            alt=""
            width="30"
            height="30"
          />Yhimhas<span> / </span>NOTES</RouterLink
        >
        <span>© {{ new Date().getFullYear() }}</span>
        <a href="#page-content">回到顶部 ↑</a>
      </footer>
    </div>
  </FieldProvider>
</template>
