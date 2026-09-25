package music

import (
	"blog-website/backend/internal/auth"
	"blog-website/backend/internal/music/provider"
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
	"unicode/utf8"
)

// Import is a local administration path for verified/manual platform metadata.
// It never fetches a URL and never edits an existing daily recommendation.
func Import(ctx context.Context, db *gorm.DB, source Source, items []provider.Track) error {
	if source.ID == "" || len(source.ID) > 150 || source.ExternalID == "" || strings.TrimSpace(source.Title) == "" || utf8.RuneCountInString(source.Title) > 160 || !provider.OfficialURL(source.SourceURL, source.Provider, false) || (source.EmbedURL != nil && !provider.OfficialURL(*source.EmbedURL, source.Provider, true)) {
		return provider.InvalidSource
	}
	if source.Provider == "netease" && (!provider.Decimal.MatchString(source.ExternalID) || source.ID != "netease:"+source.ExternalID) {
		return provider.InvalidSource
	}
	if err := provider.Validate(items, source.Provider); err != nil {
		return err
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		source.Enabled = true
		source.SyncStatus = "pending"
		source.SnapshotHash = ""
		source.SyncedAt = nil
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&source).Error; err != nil {
			return err
		}
		var current Source
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", source.ID).Take(&current).Error; err != nil {
			return err
		}
		if current.SyncStatus == "running" {
			return ErrBusy
		}
		if current.Provider != source.Provider || current.ExternalID != source.ExternalID {
			return provider.InvalidSource
		}
		if err := tx.Model(&Source{}).Where("id=?", source.ID).Updates(map[string]any{"title": source.Title, "source_url": source.SourceURL, "embed_url": source.EmbedURL, "enabled": true}).Error; err != nil {
			return err
		}
		run := Run{ID: auth.Token(), SourceID: source.ID, Status: "running", StartedAt: time.Now().UTC(), RequestID: "local-import"}
		if err := tx.Create(&run).Error; err != nil {
			return err
		}
		return (&Service{DB: tx}).ReplaceSnapshot(ctx, source, run, items)
	})
}
