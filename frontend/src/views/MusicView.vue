<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { RouterLink } from "vue-router";
import MusicIcon from "../components/MusicIcon.vue";
import { useMusicLibrary } from '../useMusicLibrary';
import { useMusicPlayer } from '../useMusicPlayer';
import { migrateFavoriteID, legacyFavorite, missingFavorite, readFavoriteSnapshots } from '../musicFavorites';
import type { MusicTrack } from '../musicLibraryApi';
import { playlistSyncFailureMessage } from '../musicLibraryApi';
const { playlists: platformPlaylists, status: libraryStatus, error: libraryError, reload: loadLibrary } = useMusicLibrary();
const player = useMusicPlayer();
const { state: playerState, message: playerMessage, currentTime, duration, volume } = player;
const playbackCapability = player.capability;
const mediaKindLabel = computed(() => playbackCapability.value.mediaKind === 'full' ? '完整音源' : playbackCapability.value.mediaKind === 'preview' ? '试听片段' : '完整性未确认');
function timeLabel(value: number) { return `${Math.floor(value / 60)}:${String(Math.floor(value % 60)).padStart(2, '0')}`; }
import { shanghaiDate } from "../musicDaily";
import { fetchTodayRecommendations } from "../musicApi";
import { useMusicRecommendations } from "../useMusicRecommendations";
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
const playlistSyncNotices = computed(() => {
  if (activeTab.value !== 'library' || libraryStatus.value === 'loading' || libraryError.value) return [];
  return platformPlaylists.value
    .filter(item => (platform.value === 'all' || item.platform === platform.value) && item.syncStatus !== 'ready')
    .map(playlist => {
      const hasTracks = playlist.tracks.length > 0;
      switch (playlist.syncStatus) {
        case 'pending':
          return { playlist, label: '待同步', message: hasTracks ? '歌单待同步，已收录曲目仍可搜索和选择。' : '歌单已添加，曲目列表待同步。可先前往原站查看。' };
        case 'running':
          return { playlist, label: '同步中', message: hasTracks ? '歌单正在同步，已收录曲目仍可搜索和选择。' : '歌单正在同步，曲目列表尚未就绪。可先前往原站查看。' };
        case 'failed':
          return { playlist, label: '同步失败', message: playlistSyncFailureMessage(playlist) };
        default:
          return { playlist, label: '同步状态未知', message: '暂时无法确认歌单同步状态，请重新加载或前往原站查看。' };
      }
    });
});
const day = ref(shanghaiDate());
const localTracks = computed(() => [...new Map(platformPlaylists.value.flatMap(p => p.tracks).map(t => [t.id,t])).values()]);
const savedRecommendedTracks = ref<MusicTrack[]>([]);
const { state: recommendationState, recommendations, recommendationDate, localFallback, reload: loadRecommendations } = useMusicRecommendations({
  mode: 'api',
  day,
  localTracks: [],
  request: async signal => {
    const result = await fetchTodayRecommendations(signal);
    return { ...result, timezone: 'Asia/Shanghai' as const, items: result.items.map(track => ({ ...track, embedUrl: '' })) };
  },
});
const recommendationLoading = computed(() => recommendationState.value.status === 'loading');
const recommendationError = computed(() => recommendationState.value.status === 'error' ? recommendationState.value.message : '');
const selectedTrack = ref<MusicTrack>();
const uniqueTracks = computed<MusicTrack[]>(() => [...new Map(
  [...savedRecommendedTracks.value, ...recommendations.value, ...localTracks.value].map(track => [track.id, track]),
).values()]);
const currentTrack = computed(
  () =>
    selectedPlaylistId.value ? undefined : (player.track.value || selectedTrack.value || uniqueTracks.value.find((track) => track.id === selectedId.value) ||
    recommendations.value[0]),
);
const currentPlaylist = computed(() =>
  platformPlaylists.value.find((item) => item.id === (selectedPlaylistId.value || currentTrack.value?.playlistId)),
);
const officialUrl = computed(() => currentTrack.value?.url || currentPlaylist.value?.url);
const currentPlatform = computed(() => currentTrack.value?.platform || currentPlaylist.value?.platform);
const platformName = computed(() => currentPlatform.value === 'bilibili' ? 'Bilibili' : '网易云音乐');
const trackQueue = player.queue;
function openPlaylist(id: string) {
  player.stop();
  selectedPlaylistId.value = id;
  selectedId.value = '';
  selectedTrack.value = undefined;
}
const favoriteCount = computed(
  () =>
    uniqueTracks.value.filter((track) => favorites.value.includes(track.id)).length,
);
const searching = computed(() => Boolean(query.value.trim()));
const showingDaily = computed(
  () => activeTab.value === "discover" && !searching.value,
);
const visibleTracks = computed(() => {
  const source = showingDaily.value ? recommendations.value : activeTab.value === "favorites" ? uniqueTracks.value : localTracks.value;
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
  selectedTrack.value = uniqueTracks.value.find(track => track.id === id);
  if (selectedTrack.value) void player.play(selectedTrack.value, visibleTracks.value);
}
function stepTrack(direction: number) { player.step(direction); }
function favorite(id: string) {
  favorites.value = favorites.value.includes(id)
    ? favorites.value.filter((item) => item !== id)
    : [...favorites.value, id];
  savedRecommendedTracks.value = [...new Map(
    [...savedRecommendedTracks.value, ...uniqueTracks.value]
      .filter(track => favorites.value.includes(track.id)).map(track => [track.id, track]),
  ).values()];
  saveFavorites();
}
function saveFavorites() {
  try {
    localStorage.setItem('lin-music-favorite-snapshots-v2', JSON.stringify({ data: {
      date: day.value, timezone: 'Asia/Shanghai', status: 'ready',
      items: savedRecommendedTracks.value.map(track => ({
        id: track.id, title: track.title, author: track.artist, provider: track.platform,
        sourceUrl: track.url, playlistId: track.playlistId, availability: track.availability || 'unknown', externalId: track.externalId, partId: track.partId, durationSeconds: track.durationSeconds,
      })),
    } }));
    localStorage.setItem(
      "lin-music-favorites-v2",
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
  if (document.visibilityState === 'hidden') return;
  const today = shanghaiDate();
  if (today !== day.value) day.value = today;
  else if (recommendationError.value) void loadRecommendations();
}
let dayTimer: ReturnType<typeof setInterval> | undefined;
onMounted(() => {
  try {
    const snapshots = (localStorage.getItem('lin-music-favorite-snapshots-v2') || localStorage.getItem('lin-music-favorite-snapshots'));
    if (snapshots) savedRecommendedTracks.value = readFavoriteSnapshots(JSON.parse(snapshots));
  } catch {
    notice.value = '部分收藏曲目信息暂时无法读取。';
  }
  try {
    const saved: unknown = JSON.parse(
      localStorage.getItem("lin-music-favorites-v2") || localStorage.getItem("lin-music-favorites") || "[]",
    );
    if (Array.isArray(saved))
      favorites.value = [
        ...new Set(saved.filter((id): id is string => typeof id === "string").map(migrateFavoriteID)),
      ];
    if (Array.isArray(saved)) {
      for (const id of saved) { if (typeof id !== 'string') continue; const t = legacyFavorite(id) || missingFavorite(migrateFavoriteID(id)); if (!savedRecommendedTracks.value.some(x => x.id === t.id)) savedRecommendedTracks.value.push(t); }
      saveFavorites();
    }
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
                  showingDaily ? (recommendationDate?.replaceAll("-", ".") || "等待服务端日期") : "YOUR SOUND ARCHIVE"
                }}</span
              ><span
                >{{
                  `${String(visibleTracks.length).padStart(2, "0")} TRACKS`
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
                :placeholder="
                  activeTab === 'favorites'
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
              :aria-busy="showingDaily && recommendationLoading"
            >
              <Transition name="music-source" mode="out-in" @before-enter="resetTrackScroll">
              <div :key="platform" class="music-source-results">
              <div v-for="item in playlistSyncNotices" :key="item.playlist.id" class="music-playlist-status" role="status">
                <strong>{{ item.playlist.title }} · {{ item.label }}</strong>
                <p>{{ item.message }}</p>
                <div class="music-playlist-actions">
                  <button @click="openPlaylist(item.playlist.id)">选择此歌单</button>
                  <button @click="loadLibrary">重新加载</button>
                  <a :href="item.playlist.url" target="_blank" rel="noopener noreferrer">在{{ item.playlist.platform === 'netease' ? '网易云' : 'Bilibili' }}查看歌单 ↗</a>
                </div>
              </div>
              <article
                v-for="(track, index) in visibleTracks"
                :key="track.id"
                :class="[
                  'music-track',
                  { 'is-selected': player.track.value?.id === track.id },
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
              <div v-if="!showingDaily && libraryStatus === 'loading'" class="music-empty">正在加载音乐库…</div>
              <div v-if="!showingDaily && libraryError" class="music-empty"><p>{{ libraryError }}</p><button @click="loadLibrary">重新加载</button></div>
              <div v-if="!visibleTracks.length && (showingDaily || (libraryStatus !== 'loading' && !libraryError))" class="music-empty">
                <span aria-hidden="true">{{
                  activeTab === "favorites" ? "♡" : "↗"
                }}</span>
                <h3>
                  {{
                    showingDaily ? (recommendationLoading ? '正在加载每日推荐' : recommendationError ? '今日推荐暂不可用' : '今天暂无推荐曲目') : searching
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
                    showingDaily ? (recommendationLoading ? '稍等片刻，正在获取今天的歌单。' : recommendationError || '暂时没有可推荐的曲目，可以先逛逛音乐库。') : searching
                      ? "换一个歌名或作者试试。"
                      : activeTab === "favorites"
                        ? "点击曲目旁的爱心，就能在这里再次遇见。"
                        : "试试其他来源，或稍后再来。"
                  }}
                </p>
                <button v-if="showingDaily && recommendationError && !recommendationLoading" @click="loadRecommendations">重新加载 ↗</button>
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
                  ? (localFallback ? "开发期本地 fallback · 非 API 推荐" : "服务端每日推荐 · Asia/Shanghai")
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
            <a
            :href="officialUrl"
            target="_blank"
            rel="noopener noreferrer"
            >{{ currentTrack ? (currentPlatform === 'bilibili' ? '原视频' : '原曲目') : '原歌单' }} ↗</a
          >
          </div>
        </div>
        <div class="music-player-controls">
          <button :disabled="!currentTrack || playerState === 'preparing'" @click="player.track.value ? player.toggle() : currentTrack && choose(currentTrack.id)">{{ playerState === 'playing' || playerState === 'buffering' ? '暂停' : playerState === 'blocked' ? '继续播放' : '播放' }}</button>
          <button :disabled="!player.track.value" @click="player.stop">停止</button>
          <button v-if="playerState === 'error' && player.track.value" @click="player.play(player.track.value, player.queue.value)">重新播放</button>
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
            :disabled="trackQueue.length < 2"
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
            :disabled="trackQueue.length < 2"
            aria-label="下一首"
            @click="stepTrack(1)"
          >
            <MusicIcon name="next" />
          </button>
        </div>
      </div>
      <div class="music-playback-progress">
        <span>{{ timeLabel(currentTime) }} / {{ duration ? timeLabel(duration) : '--:--' }}</span>
        <progress :value="currentTime" :max="duration || 1" aria-label="播放进度（暂不支持拖动）" />
        <label>音量 <input type="range" min="0" max="1" step="0.05" :value="volume" @input="player.setVolume(Number(($event.target as HTMLInputElement).value))" /></label>
      </div>
      <p id="music-playback-status" class="music-player-status">
        {{ playerMessage }}
      </p>
      <p v-if="player.track.value" class="music-player-status" aria-live="polite">
        {{ mediaKindLabel }}
        <span v-if="playbackCapability.mediaKind === 'preview' && playbackCapability.previewStartSeconds !== null && playbackCapability.previewEndSeconds !== null"> · {{ timeLabel(playbackCapability.previewStartSeconds) }}–{{ timeLabel(playbackCapability.previewEndSeconds) }}</span>
      </p>
    </section>
    <p v-if="notice" class="music-notice" role="status">{{ notice }}</p>
    <footer class="music-colophon">
      <span>YHIMHAS / A MOMENT TO LISTEN</span><span>{{ day }} · SHANGHAI</span
      ><span>MAKE EVERY DAY A GOOD TRACK. ↗</span>
    </footer>
  </section>
</template>
