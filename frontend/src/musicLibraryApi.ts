import type { RecommendedTrack } from './musicApi.ts';
export type MusicTrack = RecommendedTrack & {
    externalId?: string;
    partId?: string | null;
    durationSeconds?: number | null;
    availability?: 'unknown' | 'available' | 'unavailable';
    embedUrl?: string;
};
export interface MusicPlaylist {
    id: string;
    platform: 'bilibili' | 'netease';
    title: string;
    url: string;
    syncStatus: string;
    syncErrorCode?: string | null;
    syncedAt?: string | null;
    tracks: MusicTrack[];
}
export function mapTrack(raw: unknown): MusicTrack {
    const t = raw as Record<string, unknown>;
    if (!t || typeof t.id !== 'string' || typeof t.title !== 'string' || typeof t.sourceUrl !== 'string' || (t.provider !== 'bilibili' && t.provider !== 'netease') || !['unknown', 'available', 'unavailable'].includes(String(t.availability)))
        throw Error('曲目信息不完整');
    const u = new URL(t.sourceUrl);
    if (u.protocol !== 'https:' || u.host !== (t.provider === 'bilibili' ? 'www.bilibili.com' : 'music.163.com') || u.username || u.password)
        throw Error('曲目来源无效');
    return { id: t.id, title: t.title, artist: typeof t.author === 'string' ? t.author : '未知作者', url: u.href, platform: t.provider, playlistId: typeof t.playlistId === 'string' ? t.playlistId : '', externalId: typeof t.externalId === 'string' ? t.externalId : undefined, partId: typeof t.partId === 'string' ? t.partId : null, durationSeconds: typeof t.durationSeconds === 'number' ? t.durationSeconds : null, availability: t.availability as MusicTrack['availability'], embedUrl: '' };
}
async function get(path: string, signal: AbortSignal) { const r = await fetch(path, { signal, cache: 'no-store' }); if (!r.ok)
    throw Error('音乐库加载失败，请重试'); return r.json(); }
export async function fetchMusicLibrary(signal: AbortSignal): Promise<MusicPlaylist[]> {
    const payload = await get('/api/v1/music/playlists', signal);
    if (!Array.isArray(payload.data))
        throw Error('歌单响应无效');
    const result: MusicPlaylist[] = [];
    for (const p of payload.data) {
        if (typeof p.id !== 'string' || typeof p.title !== 'string' || (p.provider !== 'bilibili' && p.provider !== 'netease') || typeof p.sourceUrl !== 'string')
            throw Error('歌单响应无效');
        const u = new URL(p.sourceUrl);
        if (u.protocol !== 'https:' || u.username || u.password || !['space.bilibili.com', 'music.163.com'].includes(u.host))
            throw Error('歌单来源无效');
        const tracks: MusicTrack[] = [];
        const seen = new Set<string>();
        let expectedTotal: number | undefined;
        for (let page = 1; page <= 40; page++) {
            const response = await get(`/api/v1/music/playlists/${encodeURIComponent(p.id)}/tracks?page=${page}&pageSize=50`, signal);
            if (!Array.isArray(response.data) || response.data.length > 50 || !Number.isInteger(response.pagination?.total) || response.pagination.total < 0 || response.pagination.total > 2000)
                throw Error('曲目分页无效');
            const total: number = response.pagination.total;
            if (expectedTotal !== undefined && expectedTotal !== total)
                throw Error('歌单在加载期间发生变化，请重新加载');
            expectedTotal = total;
            for (const raw of response.data) {
                const track = mapTrack(raw);
                if (track.platform !== p.provider || seen.has(track.id))
                    throw Error('曲目分页包含重复或来源不符的曲目，请重新加载');
                seen.add(track.id);
                tracks.push({ ...track, playlistId: p.id });
            }
            if (tracks.length > total)
                throw Error('曲目分页数量不一致，请重新加载');
            if (tracks.length === total)
                break;
            if (!response.data.length || page === 40)
                throw Error('曲目分页未加载完整');
        }
        result.push({ id: p.id, title: p.title, url: p.sourceUrl, platform: p.provider, syncStatus: p.syncStatus, syncErrorCode: typeof p.syncErrorCode === 'string' ? p.syncErrorCode : null, syncedAt: typeof p.syncedAt === 'string' ? p.syncedAt : null, tracks });
    }
    return result;
}

/** Only fixed messages reach visitors; upstream messages and unknown codes stay private. */
export function playlistSyncFailureMessage(playlist: MusicPlaylist): string {
    const source = playlist.platform === 'netease' ? '网易云' : '来源平台';
    let reason: string;
    switch (playlist.syncErrorCode) {
        case 'UPSTREAM_ACCESS_RESTRICTED':
        case 'UPSTREAM_REJECTED':
            reason = `${source}未提供此歌单的可用数据，本次未完成同步。`;
            break;
        case 'UPSTREAM_UNAUTHORIZED':
            reason = `${source}拒绝了本次歌单读取请求。`;
            break;
        case 'UPSTREAM_RATE_LIMITED':
            reason = `${source}请求受限，请稍后再试。`;
            break;
        case 'INCOMPLETE_PLAYLIST':
            reason = '未取得完整的歌单曲目，本次未完成同步。';
            break;
        case 'INVALID_PAYLOAD':
            reason = `${source}响应格式异常，本次未完成同步。`;
            break;
        case 'RESPONSE_TOO_LARGE':
            reason = `${source}响应超出处理范围，本次未完成同步。`;
            break;
        case 'UPSTREAM_TIMEOUT':
            reason = '读取歌单超时，请稍后再试。';
            break;
        case 'UPSTREAM_HTTP':
        case 'UPSTREAM_UNAVAILABLE':
            reason = '暂时无法从来源平台读取歌单，请稍后再试。';
            break;
        case 'SYNC_INTERRUPTED':
            reason = '本次歌单同步已中断。';
            break;
        default:
            reason = '歌单同步失败，暂时无法确认具体原因。';
    }
    const snapshot = playlist.syncedAt
        ? (playlist.tracks.length ? '已保留原有歌单，已收录曲目仍可搜索和选择。' : '已保留原有的空歌单快照。')
        : (playlist.tracks.length ? '尚无完整同步记录，已收录曲目仍可搜索和选择。' : '尚无完整同步记录。');
    return `${reason}${snapshot}可重新加载查看同步状态，或前往原站查看。`;
}
