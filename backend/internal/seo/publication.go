// Package seo coordinates durable public-content revisions and static releases.
package seo

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrStale = errors.New("public content changed; retry with a new release")
var ErrBusy = errors.New("another SEO publisher is running")

type Publication struct {
	Singleton       bool       `json:"-"`
	SourceID        string     `json:"sourceId"`
	Revision        int64      `json:"revision,string"`
	AppliedRevision int64      `json:"appliedRevision,string"`
	LastAttemptAt   *time.Time `json:"lastAttemptAt"`
	PublishedAt     *time.Time `json:"publishedAt"`
	LastError       string     `json:"lastError"`
}

func (Publication) TableName() string { return "seo_publication" }

func Read(ctx context.Context, db *gorm.DB) (Publication, error) {
	var p Publication
	err := db.WithContext(ctx).Where("singleton = true").Take(&p).Error
	return p, err
}

// activate checks under the same row lock used by the article trigger. The
// filesystem switch precedes acknowledgement. A crash in between is recovered
// from the current release manifest on the next run (at-least-once delivery).
func activate(ctx context.Context, db *gorm.DB, revision string, switchRelease func() error) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p Publication
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("singleton = true").Take(&p).Error; err != nil {
			return err
		}
		if strconv.FormatInt(p.Revision, 10) != revision {
			return ErrStale
		}
		if err := switchRelease(); err != nil {
			return err
		}
		return tx.Model(&Publication{}).Where("singleton = true").Updates(map[string]any{
			"applied_revision": p.Revision, "published_at": time.Now().UTC(), "last_error": "",
		}).Error
	})
}

// withLease pins the PostgreSQL session for the entire build. A lost session
// cannot activate: activation uses this same connection. Each schema's singleton
// has its own key, allowing isolated integration suites to run independently.
func withLease(ctx context.Context, db *gorm.DB, work func(*gorm.DB) error) error {
	return db.WithContext(ctx).Connection(func(conn *gorm.DB) error {
		var locked bool
		if err := conn.Raw("SELECT pg_try_advisory_lock(1936027437, 'seo_publication'::regclass::oid::int)").Scan(&locked).Error; err != nil {
			return err
		}
		if !locked {
			return ErrBusy
		}
		defer func() {
			// The build context may have expired. Still release the session lock
			// before its connection is returned to the pool.
			unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var unlocked bool
			result := conn.WithContext(unlockCtx).Raw("SELECT pg_advisory_unlock(1936027437, 'seo_publication'::regclass::oid::int)").Scan(&unlocked)
			if result.Error != nil || !unlocked {
				if c, ok := conn.Statement.ConnPool.(*sql.Conn); ok {
					// Never return a possibly still-locked session to the pool.
					_ = c.Raw(func(any) error { return driver.ErrBadConn })
				}
			}
		}()
		return work(conn)
	})
}
