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
        for (let page = 1; page <= 40; page++) {
            const response = await get(`/api/v1/music/playlists/${encodeURIComponent(p.id)}/tracks?page=${page}&pageSize=50`, signal);
            if (!Array.isArray(response.data) || !Number.isInteger(response.pagination?.total) || response.pagination.total > 2000)
                throw Error('曲目分页无效');
            tracks.push(...response.data.map((t: unknown) => ({ ...mapTrack(t), playlistId: p.id })));
            if (tracks.length >= response.pagination.total)
                break;
            if (!response.data.length || page === 40)
                throw Error('曲目分页未加载完整');
        }
        result.push({ id: p.id, title: p.title, url: p.sourceUrl, platform: p.provider, syncStatus: p.syncStatus, tracks });
    }
    return result;
}
