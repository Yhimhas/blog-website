package seo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gorm.io/gorm"
)

type Worker struct {
	DB     *gorm.DB
	Layout Layout
	Origin string
	Force  bool
	Logger *slog.Logger
	Build  func(context.Context, string) error
}

// Run performs at most one build. A periodic scheduler retries failures and
// drains newer revisions without an unbounded build loop or in-memory queue.
func (w Worker) Run(ctx context.Context) error {
	if w.Build == nil || !validOrigin(w.Origin) {
		return errors.New("SEO builder and valid site origin are required")
	}
	layout, err := w.Layout.Validate()
	if err != nil {
		return err
	}
	w.Layout = layout
	return withLease(ctx, w.DB, func(db *gorm.DB) error {
		p, err := Read(ctx, db)
		if err != nil {
			return err
		}
		revision := strconv.FormatInt(p.Revision, 10)
		// Trust only a symlink to a retained release in our configured store.
		// Recover a crash after rename but before the database commit. Also
		// rebuild if someone changed current despite an up-to-date DB receipt.
		if !w.Force {
			target, linkErr := filepath.EvalSymlinks(w.Layout.Current)
			if linkErr == nil && within(w.Layout.Releases, target) {
				m, manifestErr := ReadManifest(target, w.Origin)
				if manifestErr == nil && m.Revision == revision && m.SourceID == p.SourceID {
					if p.AppliedRevision == p.Revision && p.LastError == "" {
						return nil
					}
					return activate(ctx, db, revision, func() error { return nil })
				}
			}
		}
		if err := db.Model(&Publication{}).Where("singleton = true").Updates(map[string]any{"last_attempt_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		err = w.buildAndActivate(ctx, db, revision, p.SourceID)
		if err != nil {
			// Keep error receipts useful but bounded and free of SQL/build logs.
			message := "build or activation failed; inspect SEO worker logs"
			if errors.Is(err, ErrStale) {
				message = ErrStale.Error()
			}
			receiptCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			db.WithContext(receiptCtx).Model(&Publication{}).Where("singleton = true").Update("last_error", message)
		}
		return err
	})
}

func (w Worker) buildAndActivate(ctx context.Context, db *gorm.DB, revision, sourceID string) error {
	if err := os.MkdirAll(w.Layout.Releases, 0755); err != nil {
		return err
	}
	id, err := uniqueID()
	if err != nil {
		return err
	}
	release := filepath.Join(w.Layout.Releases, "revision-"+revision+"-"+id)
	// Reserve an empty directory ourselves; never reuse a failed output.
	if err := os.Mkdir(release, 0755); err != nil {
		return err
	}
	if w.Logger != nil {
		w.Logger.Info("SEO build started", "revision", revision, "release", release)
	}
	if err := w.Build(ctx, release); err != nil {
		return fmt.Errorf("SEO build failed (retained %s): %w", release, err)
	}
	m, err := ReadManifest(release, w.Origin)
	if err != nil {
		return err
	}
	if m.Revision != revision || m.SourceID != sourceID {
		return ErrStale
	}
	if err := w.Layout.PublishAssets(release); err != nil {
		return err
	}
	if err := activate(ctx, db, revision, func() error { return w.Layout.Switch(release) }); err != nil {
		return err
	}
	if w.Logger != nil {
		w.Logger.Info("SEO release activated", "revision", revision, "release", release)
	}
	return nil
}
