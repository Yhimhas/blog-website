<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { RouterLink } from "vue-router";
import MusicIcon from "../components/MusicIcon.vue";
import { platformPlaylists } from "../musicSources";
import { dailySelection, shanghaiDate } from "../musicDaily";
import "../assets/music-atlus.css";

type Scene = "discover" | "library" | "favorites";
const tabs = [
  {
    id: "discover" as const,
    label: "发现",
    en: "DISCOVER",
  },
  {
    id: "library" as const,
    label: "音乐库",
    en: "COLLECTION",
  },
  {
    id: "favorites" as const,
    label: "我的收藏",
    en: "FAVORITES",
  },
];
const backgrounds = [
  { name: "Aegis", url: "/music-golden-rain.jpeg", focalPoint: "68% 42%" },
  { name: "Yukari", url: "/music-blue-rain.jpeg", focalPoint: "68% 42%" },
  { name: "Makoto", url: "/music-pink-rain.jpeg", focalPoint: "50% 42%" },
];
const backgroundChoices = ref<Record<Scene, number>>({
  discover: 0,
  library: 1,
  favorites: 2,
});
const activeTab = ref<Scene>("discover");
const scene = computed(() => tabs.find((tab) => tab.id === activeTab.value)!);
const backgroundIndex = computed(
  () => backgroundChoices.value[activeTab.value],
);
const background = computed(() => backgrounds[backgroundIndex.value]!);
const motionOff = ref(false);
const direction = ref(1);
const query = ref("");
const platform = ref<"all" | "netease" | "bilibili">("all");
const platforms = [
  { id: 'all' as const, name: '全部' },
  { id: 'bilibili' as const, name: 'Bilibili' },
  { id: 'netease' as const, name: '网易云' },
];
const platformIndex = computed(() => platforms.findIndex(item => item.id === platform.value));
const trackList = ref<HTMLElement>();
function resetTrackScroll() {
  if (trackList.value) trackList.value.scrollTop = 0;
}
const selectedId = ref("");
const favorites = ref<string[]>([]);
const notice = ref("");
const selectedPlaylistId = ref('');
const neteasePlaylist = platformPlaylists.find(item => item.id === 'netease:595975585');
const pendingPlaylist = computed(() => activeTab.value === 'library' && platform.value === 'netease' && neteasePlaylist?.syncStatus === 'pending');
const day = ref(shanghaiDate());
const allTracks = platformPlaylists.flatMap((playlist) =>
  playlist.tracks.map((track) => ({
    ...track,
    playlistId: playlist.id,
    playlistTitle: playlist.title,
    platform: playlist.platform,
  })),
);
const uniqueTracks = [
  ...new Map(allTracks.map((track) => [track.id, track])).values(),
];
const recommendations = computed(() => dailySelection(uniqueTracks, day.value));
const currentTrack = computed(
  () =>
    selectedPlaylistId.value ? undefined : (uniqueTracks.find((track) => track.id === selectedId.value) ||
    recommendations.value[0]),
);
const currentPlaylist = computed(() =>
  platformPlaylists.find((item) => item.id === (selectedPlaylistId.value || currentTrack.value?.playlistId)),
);
const officialUrl = computed(() => currentTrack.value?.url || currentPlaylist.value?.url);
const platformName = computed(() => currentPlaylist.value?.platform === 'bilibili' ? 'Bilibili' : '网易云音乐');
function openPlaylist(id: string) {
  selectedPlaylistId.value = id;
  selectedId.value = '';
}
const favoriteCount = computed(
  () =>
    uniqueTracks.filter((track) => favorites.value.includes(track.id)).length,
);
const searching = computed(() => Boolean(query.value.trim()));
const showingDaily = computed(
  () => activeTab.value === "discover" && !searching.value,
);
const visibleTracks = computed(() => {
  const source = showingDaily.value ? recommendations.value : uniqueTracks;
  return source.filter(
    (track) =>
      (activeTab.value !== "favorites" || favorites.value.includes(track.id)) &&
      (activeTab.value === "discover" ||
        platform.value === "all" ||
        track.platform === platform.value) &&
      `${track.title} ${track.artist}`
        .toLowerCase()
        .includes(query.value.trim().toLowerCase()),
  );
});
const panelTitle = computed(() =>
  searching.value
    ? "搜索结果"
    : showingDaily.value
      ? "每日推荐"
      : scene.value.label,
);
const panelEnglish = computed(() =>
  searching.value
    ? "SEARCH RESULTS"
    : showingDaily.value
      ? "DAILY MIX"
      : scene.value.en,
);
function switchTab(id: Scene) {
  direction.value =
    tabs.findIndex((tab) => tab.id === id) >=
    tabs.findIndex((tab) => tab.id === activeTab.value)
      ? 1
      : -1;
  activeTab.value = id;
  query.value = "";
  platform.value = "all";
}
function moveTab(event: KeyboardEvent) {
  if (
    ![
      "ArrowLeft",
      "ArrowRight",
      "ArrowUp",
      "ArrowDown",
      "Home",
      "End",
    ].includes(event.key)
  )
    return;
  event.preventDefault();
  const index = tabs.findIndex((tab) => tab.id === activeTab.value);
  const next =
    event.key === "Home"
      ? 0
      : event.key === "End"
        ? tabs.length - 1
        : (index +
            (["ArrowRight", "ArrowDown"].includes(event.key) ? 1 : -1) +
            tabs.length) %
          tabs.length;
  switchTab(tabs[next]!.id);
  (event.currentTarget as HTMLElement)
    .querySelectorAll<HTMLButtonElement>('[role="tab"]')
    [next]?.focus();
}
function choose(id: string) {
  selectedPlaylistId.value = '';
  selectedId.value = id;
}
function stepTrack(direction: number) {
  const queue = currentPlaylist.value?.tracks || [];
  if (queue.length < 2) return;
  const index = queue.findIndex((track) => track.id === currentTrack.value?.id);
  const track = queue[(index + direction + queue.length) % queue.length];
  if (track) choose(track.id);
}
function favorite(id: string) {
  favorites.value = favorites.value.includes(id)
    ? favorites.value.filter((item) => item !== id)
    : [...favorites.value, id];
  try {
    localStorage.setItem(
      "lin-music-favorites",
      JSON.stringify(favorites.value),
    );
  } catch {
    notice.value = "收藏暂时无法保存到浏览器，本次浏览仍可使用。";
  }
}
function cycleBackground() {
  backgroundChoices.value[activeTab.value] =
    (backgroundIndex.value + 1) % backgrounds.length;
  try {
    localStorage.setItem(
      "yhimhas:music-backgrounds:v1",
      JSON.stringify(backgroundChoices.value),
    );
  } catch {
    notice.value = "背景偏好暂时无法保存，本次浏览仍可切换。";
  }
}
function updateDay() {
  day.value = shanghaiDate();
}
let dayTimer: ReturnType<typeof setInterval> | undefined;
onMounted(() => {
  try {
    const saved: unknown = JSON.parse(
      localStorage.getItem("lin-music-favorites") || "[]",
    );
    if (Array.isArray(saved))
      favorites.value = [
        ...new Set(saved.filter((id): id is string => typeof id === "string")),
      ];
    const choices: unknown = JSON.parse(
      localStorage.getItem("yhimhas:music-backgrounds:v1") || "{}",
    );
    if (choices && typeof choices === "object") {
      for (const tab of tabs) {
        const value = (choices as Record<string, unknown>)[tab.id];
        if (
          typeof value === "number" &&
          Number.isInteger(value) &&
          value >= 0 &&
          value < backgrounds.length
        )
          backgroundChoices.value[tab.id] = value;
      }
    }
  } catch {
    notice.value = "浏览器偏好暂时无法读取，已使用默认设置。";
  }
  dayTimer = setInterval(updateDay, 30_000);
  document.addEventListener("visibilitychange", updateDay);
});
onBeforeUnmount(() => {
  clearInterval(dayTimer);
  document.removeEventListener("visibilitychange", updateDay);
});
</script>

<template>
  <section
    :class="[
      'music-room',
      `music-room--${activeTab}`,
      { 'music-motion-off': motionOff },
    ]"
    :style="{ '--travel': `${direction * 35}px` }"
    aria-label="音乐空间"
  >
    <div class="music-backdrops" aria-hidden="true">
      <img
        v-for="(item, index) in backgrounds"
        :key="item.url"
        :src="item.url"
        alt=""
        :style="{ objectPosition: item.focalPoint }"
        :class="{ 'is-visible': backgroundIndex === index }"
      />
    </div>
    <div class="music-screenprint" aria-hidden="true">
      <span>SOUND<br />OF MY<br />DAYS.</span>
    </div>
    <header class="music-masthead">
      <RouterLink to="/blog" class="music-exit" aria-label="返回博客"
        ><MusicIcon name="back" /><span>返回博客</span></RouterLink
      >
      <RouterLink to="/" class="music-wordmark" aria-label="返回首页"
        >YHIMHAS<span> / MUSIC ROOM</span></RouterLink
      >
      <span class="music-edition">PERSONAL SELECTION — VOL. 01</span>
      <button
        class="music-motion"
        :aria-pressed="motionOff"
        @click="motionOff = !motionOff"
      >
        {{ motionOff ? "动态已暂停" : "动态开启" }}
        <span aria-hidden="true">{{ motionOff ? "○" : "✳" }}</span>
      </button>
    </header>

    <div class="music-stage">
      <aside class="music-menu">
        <div class="music-menu-heading"></div>
        <nav
          class="music-scene-nav"
          role="tablist"
          aria-label="音乐栏目"
          @keydown="moveTab"
        >
          <button
            v-for="(tab, index) in tabs"
            :id="`music-tab-${tab.id}`"
            :key="tab.id"
            type="button"
            role="tab"
            :aria-selected="activeTab === tab.id"
            aria-controls="music-panel"
            :tabindex="activeTab === tab.id ? 0 : -1"
            :class="{ 'is-active': activeTab === tab.id }"
            @click="switchTab(tab.id)"
          >
            <span class="music-nav-number">0{{ index + 1 }}</span
            ><span class="music-nav-label"
              ><b>{{ tab.en }}</b
              ><span>{{ tab.label }}</span></span
            ><span class="music-nav-arrow" aria-hidden="true">↗</span>
          </button>
        </nav>
        <p class="music-scene-caption"></p>
        <button class="music-background-choice" @click="cycleBackground">
          <span aria-hidden="true">◈</span> 切换背景 <b>{{ background.name }}</b
          ><span aria-hidden="true">↗</span>
        </button>
      </aside>

      <div class="music-content-shell">
        <Transition name="music-panel" mode="out-in">
          <section
            :key="activeTab"
            id="music-panel"
            class="music-content"
            role="tabpanel"
            :aria-labelledby="`music-tab-${activeTab}`"
          >
            <div class="music-panel-meta">
              <span
                >{{ panelEnglish }} /
                {{
                  showingDaily ? day.replaceAll("-", ".") : "YOUR SOUND ARCHIVE"
                }}</span
              ><span
                >{{
                  pendingPlaylist ? '待同步' : `${String(visibleTracks.length).padStart(2, "0")} TRACKS`
                }}
                </span
              >
            </div>
            <header class="music-panel-heading">
              <div>
                <h2>{{ panelTitle }}<span aria-hidden="true">↗</span></h2>
                <!-- <p>
                  {{
                    showingDaily
                      ? "每天三首，从熟悉的收藏里听见新鲜感。"
                      : activeTab === "favorites"
                        ? "那些舍不得跳过的声音。"
                        : "找到你此刻想听的那一首。"
                  }}
                </p> -->
              </div>
              <span class="music-panel-star" aria-hidden="true"><MusicIcon name="sparkle" /></span>
            </header>
            <label class="music-search"
              ><MusicIcon name="search" /><input
                v-model="query"
                type="search"
                :disabled="pendingPlaylist"
                :placeholder="
                  pendingPlaylist ? '歌单已添加，曲目列表待同步' : activeTab === 'favorites'
                    ? '搜索我的收藏…'
                    : '搜索歌名、作者…'
                "
                aria-label="搜索音乐"
              /><span aria-hidden="true">SEARCH</span></label
            >
            <div
              v-if="activeTab !== 'discover'"
              class="music-platform-filter"
              aria-label="筛选来源"
            >
              <div class="music-platform-switch" :style="{ '--platform-index': platformIndex }">
              <span class="music-platform-indicator" aria-hidden="true" />
              <button
                v-for="item in platforms"
                :key="item.id"
                :aria-pressed="platform === item.id"
                @click="platform = item.id"
              >
                {{ item.name }}
              </button>
              </div>
              <span>{{
                activeTab === "favorites"
                  ? `${favoriteCount} 首收藏`
                  : `${uniqueTracks.length} 首收录`
              }}</span>
            </div>
            <div
              ref="trackList"
              :class="['music-track-list', { 'is-daily': showingDaily }]"
              aria-live="polite"
            >
              <Transition name="music-source" mode="out-in" @before-enter="resetTrackScroll">
              <div :key="platform" class="music-source-results">
              <article
                v-for="(track, index) in visibleTracks"
                :key="track.id"
                :class="[
                  'music-track',
                  { 'is-selected': selectedId === track.id },
                ]"
              >
                <button
                  class="music-track-pick"
                  :aria-label="`选择曲目：${track.title}`"
                  :aria-current="selectedId === track.id ? 'true' : undefined"
                  @click="choose(track.id)"
                >
                  <span class="music-track-number">{{
                    String(index + 1).padStart(2, "0")
                  }}</span
                  ><span class="music-track-copy"
                    ><span v-if="showingDaily" class="music-daily-tag">{{
                      ["TODAY’S OPENING", "A LITTLE DETOUR", "ONE MORE REPEAT"][
                        index
                      ]
                    }}</span
                    ><strong>{{ track.title }}</strong
                    ><small
                      >{{ track.artist }} <span>/</span>
                      {{
                        track.platform === "bilibili"
                          ? "Bilibili"
                          : "网易云音乐"
                      }}</small
                    ></span
                  ><span aria-hidden="true">↗</span>
                </button>
                <button
                  class="music-favorite"
                  :aria-label="`${favorites.includes(track.id) ? '取消收藏' : '收藏'}：${track.title}`"
                  :aria-pressed="favorites.includes(track.id)"
                  @click="favorite(track.id)"
                >
                  <MusicIcon name="heart" />
                </button>
              </article>
              <div v-if="!visibleTracks.length" class="music-empty">
                <span aria-hidden="true">{{
                  activeTab === "favorites" ? "♡" : "↗"
                }}</span>
                <h3>
                  {{
                    pendingPlaylist ? neteasePlaylist?.title : searching
                      ? "暂时没有找到这段旋律"
                      : platform === "netease" && activeTab === 'favorites'
                        ? "还没有收藏的网易云曲目"
                        : activeTab === "favorites"
                          ? "下一次心动，留在这里"
                          : "歌单正在等待第一首歌"
                  }}
                </h3>
                <p>
                  {{
                    pendingPlaylist ? '歌单已添加，曲目列表待同步。请前往网易云官方页面查看和播放。' : searching
                      ? "换一个歌名或作者试试。"
                      : activeTab === "favorites"
                        ? "点击曲目旁的爱心，就能在这里再次遇见。"
                        : "试试其他来源，或稍后再来。"
                  }}
                </p>
                <div v-if="pendingPlaylist && neteasePlaylist" class="music-playlist-actions">
                  <button @click="openPlaylist(neteasePlaylist.id)">选择此歌单</button>
                  <a :href="neteasePlaylist.url" target="_blank" rel="noopener noreferrer">在网易云查看歌单 ↗</a>
                </div>
                <button
                  v-if="
                    activeTab === 'favorites' &&
                    !searching &&
                    platform === 'all'
                  "
                  @click="switchTab('discover')"
                >
                  去发现音乐 ↗</button
                ><button v-if="searching" @click="query = ''">
                  清空搜索 ↗
                </button>
              </div>
              </div>
              </Transition>
            </div>
            <footer class="music-panel-footer">
              <span>{{
                showingDaily
                  ? "按上海日期更新 · 选自已收录歌单"
                  : "所有喜欢，都值得被记住。"
              }}</span
              ><span aria-hidden="true">LISTEN / REPEAT</span>
            </footer>
          </section>
        </Transition>
      </div>
    </div>

    <section class="music-player" aria-label="曲目信息与控制" aria-describedby="music-playback-status">
      <div class="music-player-strip">
        <div class="music-player-title" aria-live="polite">
          <span>{{ currentTrack ? "当前曲目" : currentPlaylist ? "当前歌单" : "尚未选择曲目" }}</span>
          <h2>{{ currentTrack?.title || currentPlaylist?.title || "选择一首，开始今天的旋律" }}</h2>
          <div v-if="officialUrl" class="music-player-attribution">
            <span v-if="currentTrack">{{ currentTrack.artist }} · </span>
            <span>{{ platformName }} · </span>
            <a :href="officialUrl" target="_blank" rel="noopener noreferrer">
              {{ currentTrack ? (currentPlaylist?.platform === 'bilibili' ? '原视频' : '原曲目') : '原歌单' }} ↗
            </a>
          </div>
        </div>
        <div class="music-player-controls">
          <button
            :disabled="!currentTrack"
            class="music-favorite"
            :aria-label="
              currentTrack && favorites.includes(currentTrack.id)
                ? '取消收藏当前曲目'
                : '收藏当前曲目'
            "
            :aria-pressed="
              !!currentTrack && favorites.includes(currentTrack.id)
            "
            @click="currentTrack && favorite(currentTrack.id)"
          >
            <MusicIcon name="heart" />
          </button>
          <button
            :disabled="(currentPlaylist?.tracks.length || 0) < 2"
            aria-label="上一首"
            @click="stepTrack(-1)"
          >
            <MusicIcon name="previous" />
          </button>
          <a
            v-if="officialUrl"
            class="music-player-open"
            :href="officialUrl"
            target="_blank"
            rel="noopener noreferrer"
          >
            <span>在{{ platformName }}打开 ↗</span>
          </a>
          <button
            :disabled="(currentPlaylist?.tracks.length || 0) < 2"
            aria-label="下一首"
            @click="stepTrack(1)"
          >
            <MusicIcon name="next" />
          </button>
        </div>
      </div>
      <p id="music-playback-status" class="music-player-status">
        站内纯音频播放尚未接入，请在官方页面播放。上一首／下一首仅切换所选曲目；播放、暂停、进度和音量请在官方页面控制。
      </p>
    </section>
    <p v-if="notice" class="music-notice" role="status">{{ notice }}</p>
    <footer class="music-colophon">
      <span>YHIMHAS / A MOMENT TO LISTEN</span><span>{{ day }} · SHANGHAI</span
      ><span>MAKE EVERY DAY A GOOD TRACK. ↗</span>
    </footer>
  </section>
</template>
