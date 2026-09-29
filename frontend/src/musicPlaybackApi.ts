export interface PlaybackSession {
    sessionId: string;
    status: 'preparing' | 'ready' | 'streaming' | 'ended' | 'failed' | 'stopped' | 'expired';
    streamUrl?: string;
    mimeType?: string;
    durationSeconds: number | null;
    errorCode?: string;
    expiresAt: string;
    seekMode: 'none';
}
const messages: Record<string, string> = { AUDIO_SOURCE_UNAVAILABLE: '此曲目暂无可用的官方音源，请前往原站播放。', TRANSCODE_UNAVAILABLE: '音频转换服务暂不可用，请稍后重试。', TRANSCODE_FAILED: '音频转换失败，请重试或前往原站。', PLAYBACK_DISABLED: '站内播放暂未开放，请前往原站。', PLAYBACK_UNSUPPORTED: '此来源或音频格式暂不支持站内播放，请前往原站。', PLAYBACK_BUSY: '播放请求较多，请稍后重试。', PLAYBACK_TIMEOUT: '音源响应超时，请重新播放。', UPSTREAM_RATE_LIMITED: '来源平台暂时限流，请稍后重试。', UPSTREAM_UNAVAILABLE: '来源暂不可用，请重试或前往原站。', SESSION_EXPIRED: '播放会话已过期，请重新播放。', TRACK_NOT_FOUND: '曲目已不在音乐库，请重新加载。' };
export function playbackMessage(code?: string) { return messages[code || ''] || '播放失败，请重试或前往原站。'; }
async function request(path: string, method: string, body?: unknown, signal?: AbortSignal): Promise<PlaybackSession> {
    const response = await fetch(`/api/v1/music/playback-sessions${path}`, { method, signal: signal ? AbortSignal.any([signal, AbortSignal.timeout(12000)]) : AbortSignal.timeout(12000), credentials: 'same-origin', cache: 'no-store', headers: body ? { 'Content-Type': 'application/json' } : undefined, body: body ? JSON.stringify(body) : undefined });
    if (response.status === 204)
        return {} as PlaybackSession;
    const payload = await response.json();
    if (!response.ok)
        throw Error(playbackMessage(payload.error?.code));
    const s = payload.data as PlaybackSession;
    if (!s || typeof s.sessionId !== 'string' || !['preparing', 'ready', 'streaming', 'ended', 'failed', 'stopped', 'expired'].includes(s.status))
        throw Error('播放响应无效');
    if (s.streamUrl && s.streamUrl !== `/api/v1/music/streams/${s.sessionId}`)
        throw Error('播放地址无效');
    return s;
}
export const createPlayback = (trackId: string, signal: AbortSignal) => request('', 'POST', { trackId }, signal);
export const getPlayback = (id: string, signal?: AbortSignal) => request(`/${encodeURIComponent(id)}`, 'GET', undefined, signal);
export async function stopPlayback(id: string) { await fetch(`/api/v1/music/playback-sessions/${encodeURIComponent(id)}/stop`, { method: 'POST', signal: AbortSignal.timeout(3000), credentials: 'same-origin', keepalive: true }).catch(() => { }); }
