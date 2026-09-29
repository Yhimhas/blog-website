import { ref, onScopeDispose } from 'vue';
import { createPlayback, getPlayback, stopPlayback, playbackMessage } from './musicPlaybackApi.ts';
import type { MusicTrack } from './musicLibraryApi.ts';
export function useMusicPlayer() {
    const state = ref('idle'), message = ref('选择一首曲目开始播放。'), currentTime = ref(0), duration = ref(0), volume = ref(0.7), track = ref<MusicTrack>(), queue = ref<MusicTrack[]>([]);
    let audio: HTMLAudioElement | undefined, session = '', generation = 0, controller: AbortController | undefined;
    async function release() { controller?.abort(); controller = undefined; const old = audio; audio = undefined; if (old) {
        old.pause();
        old.removeAttribute('src');
        old.load();
    } ; const id = session; session = ''; if (id)
        await stopPlayback(id); }
    async function play(selected: MusicTrack, list: MusicTrack[]) {
        const g = ++generation;
        state.value = 'preparing';
        message.value = '正在准备音频…';
        track.value = selected;
        queue.value = [...list];
        currentTime.value = 0;
        duration.value = 0;
        await release();
        if (g !== generation)
            return;
        if (selected.platform !== 'bilibili' && selected.platform !== 'netease') {
            state.value = 'error';
            message.value = playbackMessage('PLAYBACK_UNSUPPORTED');
            return;
        }
        const c = new AbortController();
        controller = c;
        try {
            let s = await createPlayback(selected.id, c.signal);
            if (g !== generation) {
                void stopPlayback(s.sessionId);
                return;
            }
            ;
            session = s.sessionId;
            const deadline = Date.now() + 26000;
            while (s.status === 'preparing') {
                await new Promise(r => setTimeout(r, 750));
                if (g !== generation)
                    return;
                if (Date.now() > deadline)
                    throw Error(playbackMessage('PLAYBACK_TIMEOUT'));
                s = await getPlayback(session, c.signal);
            }
            if (g !== generation)
                return;
            if (s.status !== 'ready' || !s.streamUrl)
                throw Error(playbackMessage(s.errorCode));
            const a = new Audio();
            audio = a;
            a.preload = 'none';
            a.volume = volume.value;
            a.src = s.streamUrl;
            duration.value = s.durationSeconds || 0;
            const live = () => g === generation && audio === a;
            a.addEventListener('loadedmetadata', () => { if (live() && Number.isFinite(a.duration))
                duration.value = a.duration; });
            a.addEventListener('playing', () => { if (live()) {
                state.value = 'playing';
                message.value = '正在播放';
            } });
            a.addEventListener('waiting', () => { if (live()) {
                state.value = 'buffering';
                message.value = '正在缓冲…';
            } });
            a.addEventListener('pause', () => { if (live() && !a.ended) {
                state.value = 'paused';
                message.value = '已暂停';
            } });
            a.addEventListener('timeupdate', () => { if (live())
                currentTime.value = a.currentTime; });
            a.addEventListener('ended', () => { if (!live())
                return; state.value = 'ended'; message.value = '播放结束'; const i = queue.value.findIndex(t => t.id === selected.id); const next = queue.value[i + 1]; if (next)
                void play(next, queue.value); });
            a.addEventListener('error', () => { if (live()) {
                state.value = 'error';
                message.value = '音频连接中断，请重新播放。';
                const id = session;
                void getPlayback(id).then(v => { if (live() && v.errorCode)
                    message.value = playbackMessage(v.errorCode); }).catch(() => { });
                void stopPlayback(id);
            } });
            await resumeAudio(a, g);
        }
        catch (e) {
            if (g !== generation)
                return;
            state.value = 'error';
            message.value = e instanceof Error ? e.message : '播放失败，请重试';
            await release();
        }
    }
    async function resumeAudio(a: HTMLAudioElement, g: number) { try {
        await a.play();
    }
    catch (e) {
        if (g !== generation || audio !== a)
            return;
        if (e instanceof DOMException && e.name === 'NotAllowedError') {
            state.value = 'blocked';
            message.value = '点击继续播放';
        }
        else {
            state.value = 'error';
            message.value = '无法恢复音频，请重新播放。';
        }
    } }
    async function toggle() {
        if (!audio) {
            if (track.value)
                void play(track.value, queue.value);
            return;
        }
        ;
        const a = audio, g = generation;
        if (!a.paused) {
            a.pause();
            return;
        }
        try {
            const s = await getPlayback(session);
            if (g !== generation)
                return;
            if (!['ready', 'streaming', 'ended'].includes(s.status))
                throw Error();
            await resumeAudio(a, g);
        }
        catch {
            if (g === generation) {
                state.value = 'error';
                message.value = '播放会话已过期，请重新播放。';
            }
        }
    }
    function stop() { ++generation; void release(); state.value = 'stopped'; message.value = '已停止'; currentTime.value = 0; }
    function step(offset: number) { const list = queue.value; if (!list.length)
        return; const index = list.findIndex(t => t.id === track.value?.id); const next = list[(index + offset + list.length) % list.length]; if (next)
        void play(next, list); }
    function setVolume(v: number) { volume.value = Math.min(1, Math.max(0, v)); if (audio)
        audio.volume = volume.value; }
    onScopeDispose(stop);
    return { state, message, currentTime, duration, volume, track, queue, play, toggle, stop, step, setVolume };
}
