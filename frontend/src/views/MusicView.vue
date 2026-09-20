<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import MusicIcon from '../components/MusicIcon.vue'
import { officialEmbed, platformPlaylists } from '../musicSources'
import '../assets/music.css'

const tabs = ['发现', '音乐库', '我的收藏']
const activeTab = ref('音乐库')
const query = ref('')
const platform = ref<'netease' | 'bilibili'>('bilibili')
const selectedPlaylist = ref('')
const selectedTrack = ref('')
const favorites = ref<string[]>([])
const notice = ref('')
const playerOpen = ref(false)
const currentPlaylist = computed(() => platformPlaylists.find(item => item.id === selectedPlaylist.value && item.platform === platform.value) || platformPlaylists.find(item => item.platform === platform.value))
const currentTrack = computed(() => currentPlaylist.value?.tracks.find(item => item.id === selectedTrack.value) || currentPlaylist.value?.tracks[0])
const embedUrl = computed(() => officialEmbed(currentTrack.value?.embedUrl || currentPlaylist.value?.embedUrl))
const platformName = computed(() => platform.value === 'netease' ? '网易云音乐' : '哔哩哔哩')
const panelVisible = computed(() => activeTab.value !== '发现' || !!query.value.trim())
const filteredPlaylists = computed(() => platformPlaylists.filter(item => item.platform === platform.value).map(item => ({
  ...item,
  tracks: item.tracks.filter(track => (activeTab.value !== '我的收藏' || favorites.value.includes(track.id)) && `${track.title} ${track.artist}`.toLowerCase().includes(query.value.trim().toLowerCase())),
})).filter(item => item.tracks.length || (activeTab.value !== '我的收藏' && !query.value.trim())))
function switchPlatform(value: 'netease' | 'bilibili') { platform.value = value; selectedPlaylist.value = ''; selectedTrack.value = ''; playerOpen.value = false; query.value = '' }
function choose(playlistId: string, trackId = '') { selectedPlaylist.value = playlistId; selectedTrack.value = trackId; playerOpen.value = true }
function stepTrack(direction: number) {
  const playlist = currentPlaylist.value
  if (!playlist?.tracks.length) return
  const index = playlist.tracks.findIndex(track => track.id === currentTrack.value?.id)
  const track = playlist.tracks[(index + direction + playlist.tracks.length) % playlist.tracks.length]
  if (track) choose(playlist.id, track.id)
}
function favorite(id: string) {
  favorites.value = favorites.value.includes(id) ? favorites.value.filter(item => item !== id) : [...favorites.value, id]
  try { localStorage.setItem('lin-music-favorites', JSON.stringify(favorites.value)) } catch { notice.value = '当前浏览器无法保存收藏，本次浏览仍可使用。' }
}
onMounted(() => {
  try { const saved: unknown = JSON.parse(localStorage.getItem('lin-music-favorites') || '[]'); if (Array.isArray(saved)) favorites.value = saved.filter((id): id is string => typeof id === 'string') } catch { notice.value = '收藏记录暂时无法读取。' }
})
</script>

<template>
  <section class="rain-music" aria-label="音乐空间">
    <div class="rain-background" aria-hidden="true" />
    <header class="rain-header">
      <RouterLink to="/blog" class="rain-back" aria-label="返回博客"><MusicIcon name="back" /></RouterLink>
      <nav class="rain-tabs" aria-label="音乐导航"><button v-for="tab in tabs" :key="tab" :class="{ active: activeTab === tab }" :aria-pressed="activeTab === tab" @click="activeTab = tab; query = ''">{{ tab }}</button></nav>
      <label class="rain-search"><MusicIcon name="search" /><input v-model="query" type="search" placeholder="搜索歌单中的歌曲或视频…" aria-label="搜索音乐" /></label>
      <RouterLink class="rain-avatar" to="/" aria-label="返回主页"><img src="/yhimhas-logo.jpg" alt="" width="40" height="40" /></RouterLink>
    </header>
    <h1 class="rain-sr-only">音乐，让雨声有了旋律</h1>
    <section v-if="panelVisible" class="rain-library rain-glass" aria-label="平台歌单">
      <div class="rain-library-heading"><div><span>YOUR SOUND COLLECTION</span><h2>{{ query ? '搜索结果' : activeTab }}</h2></div><button class="rain-icon-button" aria-label="返回发现" @click="activeTab = '发现'; query = ''"><MusicIcon name="close" /></button></div>
      <div class="rain-platforms" aria-label="歌单平台"><button :aria-pressed="platform === 'netease'" @click="switchPlatform('netease')">网易云音乐</button><button :aria-pressed="platform === 'bilibili'" @click="switchPlatform('bilibili')">哔哩哔哩</button></div>
      <p v-if="currentPlaylist?.syncedAt" class="rain-library-note">{{ currentPlaylist.tracks.length }} 个视频 · 同步于 {{ new Date(currentPlaylist.syncedAt).toLocaleDateString('zh-CN') }}</p>
      <div v-for="playlist in filteredPlaylists" :key="playlist.id" class="rain-results">
        <button class="rain-playlist-title" @click="choose(playlist.id)">{{ playlist.title }} <span>↗</span></button>
        <div v-for="track in playlist.tracks" :key="track.id" :class="['rain-track-row', { 'is-selected': currentTrack?.id === track.id }]"><button class="rain-track-choice" :aria-current="currentTrack?.id === track.id ? 'true' : undefined" @click="choose(playlist.id, track.id)"><MusicIcon name="play" /><span><strong>{{ track.title }}</strong><small>{{ track.artist }}</small></span></button><button class="rain-icon-button" :aria-label="`${favorites.includes(track.id) ? '取消收藏' : '收藏'}${track.title}`" :aria-pressed="favorites.includes(track.id)" @click="favorite(track.id)"><MusicIcon name="heart" /></button></div>
        <p v-if="!playlist.tracks.length" class="rain-library-note">{{ playlist.embedUrl ? '曲目列表将在平台播放器中展示。' : '暂无可播放曲目。' }}</p>
      </div>
      <div v-if="!filteredPlaylists.length" class="rain-empty"><MusicIcon name="note" /><p>{{ !currentPlaylist ? `${platformName}歌单尚未配置` : activeTab === '我的收藏' ? '还没有收藏的曲目。' : '没有找到匹配的曲目。' }}</p><p v-if="!currentPlaylist">{{ platform === 'netease' ? '等待添加网易云歌单链接。' : '等待添加 Bilibili 收藏夹或合集链接。' }}</p></div>
    </section>
    <section class="rain-player rain-glass rain-platform-player" aria-label="平台音乐播放器">
      <div class="rain-platforms" aria-label="播放平台"><button :aria-pressed="platform === 'netease'" @click="switchPlatform('netease')">网易云音乐</button><button :aria-pressed="platform === 'bilibili'" @click="switchPlatform('bilibili')">哔哩哔哩</button></div>
      <div class="rain-player-top"><img class="rain-cover" src="/music-rain-background.jpg" alt="雨夜音乐空间" width="92" height="92" /><div class="rain-track-info"><h2>{{ currentTrack?.title || currentPlaylist?.title || '等待你的歌单' }}</h2><p>{{ currentTrack?.artist || platformName }}</p><p class="rain-source-status">{{ currentPlaylist ? `${currentPlaylist.title} · ${currentPlaylist.tracks.length} 个视频` : '尚未配置歌单来源' }}</p></div><button v-if="currentTrack" class="rain-icon-button" :aria-label="favorites.includes(currentTrack.id) ? '取消收藏当前曲目' : '收藏当前曲目'" :aria-pressed="favorites.includes(currentTrack.id)" @click="favorite(currentTrack.id)"><MusicIcon name="heart" /></button></div>
      <div v-if="currentPlaylist && currentPlaylist.tracks.length > 1" class="rain-track-navigation"><button class="rain-icon-button" aria-label="上一首" @click="stepTrack(-1)"><MusicIcon name="previous" /></button><span>{{ currentPlaylist.tracks.findIndex(track => track.id === currentTrack?.id) + 1 }} / {{ currentPlaylist.tracks.length }}</span><button class="rain-icon-button" aria-label="下一首" @click="stepTrack(1)"><MusicIcon name="next" /></button></div>
      <div v-if="playerOpen && embedUrl" class="rain-embed"><iframe :key="embedUrl" :src="embedUrl" :title="`${platformName}：${currentTrack?.title || currentPlaylist?.title}`" :class="{ 'is-video': platform === 'bilibili' }" allow="autoplay; fullscreen; picture-in-picture" allowfullscreen /><button class="rain-source-button" @click="playerOpen = false">关闭播放器</button><p class="rain-library-note">播放、进度和音量由平台播放器控制。</p></div>
      <div v-else class="rain-source-actions"><button v-if="embedUrl" class="rain-source-button" @click="playerOpen = true"><MusicIcon name="play" />打开站内播放器</button><button class="rain-source-button" @click="activeTab = '音乐库'; query = ''"><MusicIcon name="note" />查看{{ platformName }}歌单</button></div>
      <p v-if="!currentPlaylist" class="rain-player-note">{{ platform === 'netease' ? '连接你的歌单，让喜欢的旋律在这里相遇。' : '收藏夹与视频合集，让每一段声音都有画面。' }}</p>
      <a v-if="currentPlaylist" class="rain-official-link" :href="currentTrack?.url || currentPlaylist.url" target="_blank" rel="noopener noreferrer">若站内播放不可用，在{{ platformName }}打开 ↗</a>
      <p v-if="notice" class="rain-player-note" role="status">{{ notice }}</p>
    </section>
    <div class="rain-signature" aria-hidden="true">YHIMHAS <span>/</span> A MOMENT TO LISTEN</div>
  </section>
</template>
