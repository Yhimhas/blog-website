package music

import (
	"blog-website/backend/internal/music/playback"
	"blog-website/backend/internal/music/provider"
	"context"
)

func (s *Service) Find(ctx context.Context, id string) (provider.Track, error) {
	tracks, err := ReadTracks(s.DB.WithContext(ctx).Table("music_items m").Select("m.*").Where("m.id=? AND EXISTS (SELECT 1 FROM source_items si JOIN music_sources ms ON ms.id=si.source_id WHERE si.item_id=m.id AND si.active AND ms.enabled)", id))
	if err != nil {
		return provider.Track{}, err
	}
	if len(tracks) == 0 {
		return provider.Track{}, playback.NotFound
	}
	return tracks[0], nil
}
func (s *Service) Checked(ctx context.Context, id string, code playback.Failure) {
	availability := "unknown"
	if code == "" {
		availability = "available"
	} else if code == playback.Unsupported || code == playback.SourceUnavailable {
		availability = "unavailable"
	}
	if availability == "unknown" {
		s.DB.WithContext(ctx).Exec("UPDATE music_items SET playback_checked_at=now(),playback_error_code=? WHERE id=?", string(code), id)
		return
	}
	s.DB.WithContext(ctx).Exec("UPDATE music_items SET availability=?,playback_checked_at=now(),playback_error_code=? WHERE id=?", availability, string(code), id)
}
