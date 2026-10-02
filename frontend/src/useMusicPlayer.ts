import { ref, computed, onScopeDispose, inject, provide, getCurrentInstance } from 'vue';
import type { InjectionKey } from 'vue';
import { createPlayback, getPlayback, stopPlayback, playbackMessage, PlaybackError, unknownCapability } from './musicPlaybackApi.ts';
import type { MusicTrack } from './musicLibraryApi.ts';
const playerKey: InjectionKey<ReturnType<typeof createMusicPlayer>> = Symbol('music-player');
export function provideMusicPlayer() {
    const player = createMusicPlayer();
    provide(playerKey, player);
    return player;
}
export function useMusicPlayer() {
    if (!getCurrentInstance()) return createMusicPlayer();
    return inject(playerKey, undefined) ?? provideMusicPlayer();
}
export function createMusicPlayer() {
    const state = ref('idle'), message = ref('选择一首曲目开始播放。'), currentTime = ref(0), duration = ref(0), volume = ref(0.7), track = ref<MusicTrack>(), queue = ref<MusicTrack[]>([]);
    const capability = ref(unknownCapability()), errorCode = ref('');
    const seekMode = ref('none');
    const canSeek = computed(() => seekMode.value === 'restart' && duration.value > 0 && ['playing', 'paused', 'buffering', 'blocked'].includes(state.value));
    let operations: Promise<void> = Promise.resolve();
    let audio: HTMLAudioElement | undefined, session = '', generation = 0;
    let releasing: Promise<void> = Promise.resolve();
    async function release() { const old = audio; audio = undefined; if (old) {
        old.pause();
        old.removeAttribute('src');
        old.load();
    } ; const id = session; session = ''; if (id)
        releasing = Promise.all([releasing, stopPlayback(id)]).then(() => {});
        await releasing; }
    function play(selected: MusicTrack, list: MusicTrack[], startSeconds = 0, paused = false) {
        const g = ++generation;
        audio?.pause();
        state.value = 'preparing';
        message.value = startSeconds > 0 ? '正在跳转到指定位置…' : '正在准备音频…';
        // Serialize release/create so rapid changes cannot leave an orphan session
        // or race the previous stop against the server's one-session limit.
        operations = operations.then(async () => {
            if (g === generation) await startPlay(selected, list, startSeconds, paused, g);
        });
        return operations;
    }
    async function startPlay(selected: MusicTrack, list: MusicTrack[], startSeconds: number, paused: boolean, g: number) {
        track.value = selected;
        queue.value = [...list];
        currentTime.value = startSeconds;
        duration.value = 0;
        capability.value = unknownCapability();
        errorCode.value = '';
        seekMode.value = 'none';
        await release();
        if (g !== generation)
            return;
        if (selected.platform !== 'bilibili' && selected.platform !== 'netease') {
            state.value = 'error';
            message.value = playbackMessage('PLAYBACK_UNSUPPORTED');
            return;
        }
        const c = new AbortController();
        try {
            let s = await createPlayback(selected.id, c.signal, startSeconds);
            if (g !== generation) {
                await stopPlayback(s.sessionId);
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
                throw new PlaybackError(s.errorCode || 'UPSTREAM_UNAVAILABLE');
            capability.value = s.capability || unknownCapability();
            seekMode.value = s.seekMode || 'none';
            const offset = s.startSeconds ?? startSeconds;
            const a = new Audio();
            audio = a;
            a.preload = 'none';
            a.volume = volume.value;
            a.src = s.streamUrl;
            const cap = capability.value;
            const previewLength = cap.mediaKind === 'preview' ? (cap.previewEndSeconds ?? 0) - (cap.previewStartSeconds ?? 0) : 0;
            const knownDuration = cap.mediaKind === 'preview' ? Math.min(cap.streamDurationSeconds ?? previewLength, previewLength) : cap.streamDurationSeconds;
            const timelineDuration = knownDuration || cap.trackDurationSeconds || 0;
            duration.value = timelineDuration;
            const live = () => g === generation && audio === a;
            const updateDuration = () => { if (live() && !timelineDuration && Number.isFinite(a.duration) && a.duration > 0)
                duration.value = offset + a.duration; };
            a.addEventListener('loadedmetadata', updateDuration);
            a.addEventListener('durationchange', updateDuration);
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
                currentTime.value = offset + a.currentTime; });
            a.addEventListener('ended', () => { if (!live())
                return;
                if (capability.value.mediaKind === 'preview') {
                    state.value = 'preview-ended'; message.value = '试听结束，可前往原站收听。'; void release(); return;
                }
                state.value = 'ended'; message.value = capability.value.mediaKind === 'full' ? '播放结束' : '音频播放结束，完整性未确认。'; const i = queue.value.findIndex(t => t.id === selected.id); const next = queue.value[i + 1]; if (next)
                void play(next, queue.value); });
            a.addEventListener('error', () => { if (live()) {
                state.value = 'error';
                message.value = '音频连接中断，请重新播放。';
                const id = session;
                void getPlayback(id).then(v => { if (live() && v.errorCode) {
                    errorCode.value = v.errorCode; message.value = playbackMessage(v.errorCode);
                } }).catch(() => { });
                void stopPlayback(id);
            } });
            if (paused) { state.value = 'paused'; message.value = '已定位，点击播放继续'; }
            else await resumeAudio(a, g);
        }
        catch (e) {
            if (g !== generation)
                return;
            state.value = 'error';
            errorCode.value = e instanceof PlaybackError ? e.code : '';
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
    async function seek(seconds: number) {
        if (!canSeek.value || !track.value || !Number.isFinite(seconds)) return;
        const target = Math.max(0, Math.min(seconds, duration.value - 0.1));
        await play(track.value, queue.value, target, state.value === 'paused' || state.value === 'blocked');
    }
    onScopeDispose(stop);
    return { state, message, currentTime, duration, volume, track, queue, capability, errorCode, canSeek, play, toggle, stop, step, setVolume, seek };
}
