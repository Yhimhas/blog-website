export interface PlaybackSession {
    sessionId: string;
    status: 'preparing' | 'ready' | 'streaming' | 'ended' | 'failed' | 'stopped' | 'expired';
    streamUrl?: string;
    mimeType?: string;
    durationSeconds: number | null;
    errorCode?: string;
    expiresAt: string;
    seekMode: 'none';
    capability?: PlaybackCapability;
}
export interface PlaybackCapability {
    mediaKind: 'full' | 'preview' | 'unknown';
    trackDurationSeconds: number | null;
    streamDurationSeconds: number | null;
    previewStartSeconds: number | null;
    previewEndSeconds: number | null;
}
export const unknownCapability = (): PlaybackCapability => ({ mediaKind: 'unknown', trackDurationSeconds: null, streamDurationSeconds: null, previewStartSeconds: null, previewEndSeconds: null });
export function parseCapability(raw: unknown): PlaybackCapability {
    if (raw === undefined) return unknownCapability();
    if (!raw || typeof raw !== 'object') throw Error('音源状态无效');
    const c = raw as PlaybackCapability;
    if (!['full', 'preview', 'unknown'].includes(c.mediaKind)) throw Error('音源状态无效');
    for (const v of [c.trackDurationSeconds, c.streamDurationSeconds, c.previewStartSeconds, c.previewEndSeconds]) {
        if (v !== null && (!Number.isInteger(v) || v < 0 || v > 604800)) throw Error('音源时长无效');
    }
    if (c.mediaKind === 'preview' && (c.previewStartSeconds === null || c.previewEndSeconds === null || c.previewEndSeconds <= c.previewStartSeconds)) throw Error('试听区间无效');
    return { ...c };
}
export class PlaybackError extends Error {
    code: string;
    constructor(code: string) { super(playbackMessage(code)); this.code = code; }
}
const messages: Record<string, string> = { AUDIO_SOURCE_UNAVAILABLE: '此曲目暂无可用的官方音源，请前往原站播放。', TRANSCODE_UNAVAILABLE: '音频转换服务暂不可用，请稍后重试。', TRANSCODE_FAILED: '音频转换失败，请重试或前往原站。', PLAYBACK_DISABLED: '站内播放暂未开放，请前往原站。', PLAYBACK_UNSUPPORTED: '此来源或音频格式暂不支持站内播放，请前往原站。', PLAYBACK_BUSY: '播放请求较多，请稍后重试。', PLAYBACK_TIMEOUT: '音源响应超时，请重新播放。', UPSTREAM_RATE_LIMITED: '来源平台暂时限流，请稍后重试。', UPSTREAM_UNAVAILABLE: '来源暂不可用，请重试或前往原站。', SESSION_EXPIRED: '播放会话已过期，请重新播放。', TRACK_NOT_FOUND: '曲目已不在音乐库，请重新加载。' };
export function playbackMessage(code?: string) { return messages[code || ''] || '播放失败，请重试或前往原站。'; }
Object.assign(messages, { ENTITLEMENT_REQUIRED: '此曲目暂无可用的站内音源，请前往原站收听。', SOURCE_AUTH_EXPIRED: '音源暂不可用，请稍后重试或前往原站。', PUBLIC_PLAYBACK_NOT_ALLOWED: '此音源暂不支持在本站公开播放，请前往原站收听。', UPSTREAM_ACCESS_RESTRICTED: '来源暂时拒绝了音频请求，请稍后重试。', PLAYBACK_BUDGET_EXCEEDED: '本站音频流量已达到限额，请稍后重试或前往原站。' });
async function request(path: string, method: string, body?: unknown, signal?: AbortSignal): Promise<PlaybackSession> {
    const response = await fetch(`/api/v1/music/playback-sessions${path}`, { method, signal: signal ? AbortSignal.any([signal, AbortSignal.timeout(12000)]) : AbortSignal.timeout(12000), credentials: 'same-origin', cache: 'no-store', headers: body ? { 'Content-Type': 'application/json' } : undefined, body: body ? JSON.stringify(body) : undefined });
    if (response.status === 204)
        return {} as PlaybackSession;
    const payload = await response.json();
    if (!response.ok)
        throw new PlaybackError(typeof payload.error?.code === 'string' ? payload.error.code : 'UPSTREAM_UNAVAILABLE');
    const s = payload.data as PlaybackSession;
    if (!s || typeof s.sessionId !== 'string' || !['preparing', 'ready', 'streaming', 'ended', 'failed', 'stopped', 'expired'].includes(s.status))
        throw Error('播放响应无效');
    if (s.streamUrl && s.streamUrl !== `/api/v1/music/streams/${s.sessionId}`)
        throw Error('播放地址无效');
    s.capability = parseCapability(s.capability);
    return s;
}
export const createPlayback = (trackId: string, signal: AbortSignal) => request('', 'POST', { trackId }, signal);
export const getPlayback = (id: string, signal?: AbortSignal) => request(`/${encodeURIComponent(id)}`, 'GET', undefined, signal);
export async function stopPlayback(id: string) { await fetch(`/api/v1/music/playback-sessions/${encodeURIComponent(id)}/stop`, { method: 'POST', signal: AbortSignal.timeout(3000), credentials: 'same-origin', keepalive: true }).catch(() => { }); }
