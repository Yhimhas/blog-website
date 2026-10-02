package music

import (
	"blog-website/backend/internal/music/playback"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"gorm.io/gorm"
	"time"
)

func controlHash(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }

type controlRow struct {
	Slot               int
	KeyHash, OwnerHash string
	Used               int64
	ExpiresAt          time.Time
}

func (s *Service) control(ctx context.Context, kind, key, owner string, limit int, ttl time.Duration, amount, ceiling int64) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(current_schema() || ?))", "music-control:"+kind).Error; err != nil {
			return playback.Busy
		}
		now := time.Now().UTC()
		hash := controlHash(key)
		ownerHash := ""
		if owner != "" {
			ownerHash = controlHash(owner)
		}
		var rows []controlRow
		if err := tx.Table("music_playback_control").Where("kind=?", kind).Find(&rows).Error; err != nil {
			return playback.Busy
		}
		slot := 0
		active := 0
		used := int64(0)
		occupied := map[int]bool{}
		for _, row := range rows {
			if row.ExpiresAt.After(now) {
				active++
				occupied[row.Slot] = true
				if row.KeyHash == hash {
					slot = row.Slot
					used = row.Used
					continue
				}
				if ownerHash != "" && row.OwnerHash == ownerHash {
					return playback.Busy
				}
			}
		}
		if slot == 0 {
			if active >= limit {
				return playback.Busy
			}
			for i := 1; i <= limit; i++ {
				if !occupied[i] {
					slot = i
					break
				}
			}
		}
		if ceiling > 0 && (amount < 0 || used > ceiling-amount) {
			return playback.BudgetExceeded
		}
		expires := now.Add(ttl)
		if amount > 0 {
			expires = now.Truncate(ttl).Add(ttl)
		}
		if err := tx.Exec(`INSERT INTO music_playback_control(kind,slot,key_hash,owner_hash,used,expires_at) VALUES (?,?,?,?,?,?) ON CONFLICT(kind,slot) DO UPDATE SET key_hash=EXCLUDED.key_hash,owner_hash=EXCLUDED.owner_hash,used=EXCLUDED.used,expires_at=EXCLUDED.expires_at`, kind, slot, hash, ownerHash, used+amount, expires).Error; err != nil {
			return playback.Busy
		}
		return nil
	})
}
func (s *Service) Lease(ctx context.Context, kind, id, owner string, limit int, ttl time.Duration) error {
	return s.control(ctx, "lease:"+kind, id, owner, limit, ttl, 0, 0)
}
func (s *Service) ReleaseLease(ctx context.Context, kind, id string) {
	s.DB.WithContext(ctx).Exec("UPDATE music_playback_control SET expires_at=now() WHERE kind=? AND key_hash=?", "lease:"+kind, controlHash(id))
}
func (s *Service) Rate(ctx context.Context, key string, limit int, window time.Duration) error {
	return s.control(ctx, "rate", key, "", 8192, window, 1, int64(limit))
}
func (s *Service) Spend(ctx context.Context, key string, n, limit int64, window time.Duration) error {
	return s.control(ctx, "budget:"+key, key, "", 1, window, n, limit)
}

func (s *Service) RecordPlayback(ctx context.Context, id string, check playback.PlaybackCheck) {
	if check.SourceMode == "" {
		check.SourceMode = "public"
	}
	if check.Audience == "" {
		check.Audience = "public"
	}
	if check.MediaKind == "" {
		check.MediaKind = "unknown"
	}
	if check.Error == playback.SourceAuthExpired {
		check.SourceMode = "account"
		check.Audience = "blocked"
	}
	s.DB.WithContext(ctx).Exec(`INSERT INTO music_playback_checks(item_id,source_mode,audience,credential_version,media_kind,error_code) VALUES (?,?,?,?,?,?) ON CONFLICT(item_id,source_mode,audience,credential_version) DO UPDATE SET media_kind=EXCLUDED.media_kind,error_code=EXCLUDED.error_code,checked_at=now()`, id, check.SourceMode, check.Audience, check.CredentialVersion, check.MediaKind, string(check.Error))
	if check.SourceMode != "public" || check.Audience != "public" || check.Error == playback.PublicNotAllowed {
		return
	}
	s.Checked(ctx, id, check.Error)
	s.DB.WithContext(ctx).Exec("UPDATE music_items SET public_playback_scope='public',public_media_kind=? WHERE id=?", check.MediaKind, id)
}
