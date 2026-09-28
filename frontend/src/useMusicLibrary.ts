import { ref, onScopeDispose } from 'vue';
import { fetchMusicLibrary, type MusicPlaylist } from './musicLibraryApi.ts';
export function useMusicLibrary() {
    const playlists = ref<MusicPlaylist[]>([]), status = ref('loading'), error = ref('');
    let request: AbortController | undefined, generation = 0;
    async function reload() {
        const g = ++generation;
        request?.abort();
        const c = new AbortController();
        request = c;
        status.value = 'loading';
        error.value = '';
        const timer = setTimeout(() => c.abort(), 15000);
        try {
            const data = await fetchMusicLibrary(c.signal);
            if (g !== generation)
                return;
            playlists.value = data;
            status.value = data.some(p => p.tracks.length) ? 'ready' : 'empty';
        }
        catch {
            if (g === generation) {
                status.value = 'error';
                error.value = '音乐库暂时无法加载，请重试。';
            }
        }
        finally {
            clearTimeout(timer);
        }
    }
    void reload();
    onScopeDispose(() => { ++generation; request?.abort(); });
    return { playlists, status, error, reload };
}
