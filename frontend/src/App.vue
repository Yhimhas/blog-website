<script setup lang="ts">
import SiteIcon from "./components/SiteIcon.vue";
import { computed, ref, watch } from "vue";
import { RouterLink, RouterView, useRoute } from "vue-router";
import { FieldProvider } from "@field-lab/vue";
import { posts } from "./content";
import { pageLeaving, pageReady } from "./pageTransition";
import EntryIntro from "./components/EntryIntro.vue";
const route = useRoute();
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
  { path: "/blog", label: "博客", symbol: "layers" },
  { path: "/music", label: "音乐", symbol: "wave" },
  { path: "/about", label: "关于", symbol: "sparkle" },
  { path: "/admin", label: "管理后台", symbol: "layers" },
];
const active = computed(() =>
  route.path.startsWith("/blog") ? "/blog" : route.path,
);
watch(
  () => [route.fullPath, route.meta.title],
  () => {
    expanded.value = false;
    const title = route.path.startsWith("/blog/")
      ? posts.find((post) => post.id === route.params.id)?.title || "文章不存在"
      : route.meta.title || "首页";
    document.title = `${title} · Yhimhas / NOTES`;
  },
  { immediate: true },
);
function onPageReady() {
  pageReady(route.path);
}
function onPageEntered() {
  // Do not steal focus if someone already started typing during the entrance.
  if (document.activeElement === document.body && !document.querySelector('dialog[open]')) {
    content.value?.focus({ preventScroll: true });
  }
}
</script>

<template>
  <EntryIntro v-if="route.path === '/'" />
  <FieldProvider :duration="580" :paused="paused" accent="#d4ef37" :class="{ 'motion-paused': paused }">
    <a class="skip-link" href="#page-content">跳转到内容</a>
    <RouterView v-slot="{ Component, route: pageRoute }">
      <Transition name="page-scene" mode="out-in" @before-leave="pageLeaving" @enter="onPageReady" @after-enter="onPageEntered">
        <div :key="pageRoute.path" class="route-scene">
          <nav
            v-if="pageRoute.path !== '/music'"
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
                <span class="f-rail__icon" aria-hidden="true"><SiteIcon v-if="item.path !== '/'" :name="item.symbol" /><template v-else>{{ item.symbol }}</template></span
                ><span class="f-rail__label">{{ item.label }}</span>
              </RouterLink>
            </div>
            <div class="f-rail__bottom" aria-hidden="true">
              <span class="f-rail__barcode" /><span>FIELD / UI</span><span>V.01</span>
            </div>
          </nav>
          <div class="page site-layout" :class="{ 'music-layout': pageRoute.path === '/music' }">
            <header v-if="pageRoute.path !== '/music'" class="topbar">
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
            <main id="page-content" ref="content" tabindex="-1"><component :is="Component" /></main>
            <footer v-if="pageRoute.path !== '/music'">
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
              <a href="#page-content">回到顶部 <SiteIcon name="up" /></a>
            </footer>
          </div>
        </div>
      </Transition>
    </RouterView>
  </FieldProvider>
</template>
