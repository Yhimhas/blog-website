import snapshot from './data/bilibili-playlist.json' with { type: 'json' };
import type { MusicTrack } from './musicLibraryApi.ts';
const legacyMap = new Map<string, MusicTrack>(snapshot.tracks.map(track => [track.id, {
        id: `${track.id}:1`, title: track.title, artist: track.artist, url: track.url,
        platform: 'bilibili', playlistId: snapshot.id, externalId: track.id.slice(9),
        partId: '1', availability: 'unknown',
    }]));
export function migrateFavoriteID(id: string) { return legacyMap.get(id)?.id || id; }
export function legacyFavorite(id: string): MusicTrack | undefined {
    return legacyMap.get(id) || (id.endsWith(':1') ? legacyMap.get(id.slice(0, -2)) : undefined);
}
export function missingFavorite(id: string): MusicTrack {
    return { id, title: `暂不可用的收藏 · ${id}`, artist: '曲目信息待恢复', url: '',
        platform: id.startsWith('bilibili:') ? 'bilibili' : 'netease', playlistId: '', availability: 'unavailable' };
}
export function readFavoriteSnapshots(payload: unknown): MusicTrack[] {
    const data = (payload as {
        data?: {
            items?: unknown[];
        };
    })?.data;
    if (!Array.isArray(data?.items))
        throw Error('收藏格式无效');
    return data.items.map(raw => {
        const track = raw as Record<string, unknown>;
        if (typeof track?.id !== 'string')
            throw Error('收藏 ID 无效');
        const id = migrateFavoriteID(track.id);
        const fallback = legacyFavorite(track.id) || missingFavorite(id);
        if (typeof track.sourceUrl !== 'string' || !track.sourceUrl)
            return fallback;
        try {
            const url = new URL(track.sourceUrl);
            if (url.protocol !== 'https:' || url.username || url.password || !['www.bilibili.com', 'music.163.com'].includes(url.host))
                return fallback;
        }
        catch {
            return fallback;
        }
        return {
            ...fallback, title: typeof track.title === 'string' ? track.title : fallback.title,
            artist: typeof track.author === 'string' ? track.author : fallback.artist,
            url: track.sourceUrl, platform: track.provider === 'netease' ? 'netease' : 'bilibili',
            playlistId: typeof track.playlistId === 'string' ? track.playlistId : '',
            externalId: typeof track.externalId === 'string' ? track.externalId : fallback.externalId,
            partId: typeof track.partId === 'string' ? track.partId : fallback.partId,
            durationSeconds: typeof track.durationSeconds === 'number' ? track.durationSeconds : null,
            availability: ['unknown', 'available', 'unavailable'].includes(String(track.availability)) ? track.availability as MusicTrack['availability'] : 'unknown',
        };
    });
}
