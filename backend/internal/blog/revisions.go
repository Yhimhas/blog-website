package blog

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// One pending revision per article. The article version serializes all writes.
type Revision struct {
	PostID          string    `json:"-" gorm:"primaryKey"`
	Title           string    `json:"title"`
	Summary         string    `json:"summary"`
	ContentMarkdown string    `json:"contentMarkdown"`
	CategoryID      *string   `json:"categoryId"`
	UpdatedAt       time.Time `json:"updatedAt"`
	TagIDs          []string  `json:"tagIds" gorm:"-"`
}

func (Revision) TableName() string { return "post_revisions" }

func readRevision(tx *gorm.DB, id string) (*Revision, error) {
	var revision Revision
	err := tx.Where("post_id=?", id).Take(&revision).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	revision.TagIDs = []string{}
	err = tx.Table("post_revision_tags").Where("post_id=?", id).Order("tag_id").Pluck("tag_id", &revision.TagIDs).Error
	return &revision, err
}

func applyRevision(p *Record) {
	if revision := p.Revision; revision != nil {
		p.Title, p.Summary, p.ContentMarkdown = revision.Title, revision.Summary, revision.ContentMarkdown
		p.CategoryID, p.TagIDs = revision.CategoryID, revision.TagIDs
	}
}

func saveRevision(tx *gorm.DB, p Record) error {
	revision := Revision{PostID: p.ID, Title: p.Title, Summary: p.Summary, ContentMarkdown: p.ContentMarkdown, CategoryID: p.CategoryID, UpdatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := tx.Save(&revision).Error; err != nil {
		return err
	}
	if err := tx.Exec("DELETE FROM post_revision_tags WHERE post_id=?", p.ID).Error; err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, id := range p.TagIDs {
		if id == "" || seen[id] {
			return ErrInvalid
		}
		seen[id] = true
		if err := tx.Exec("INSERT INTO post_revision_tags(post_id,tag_id) VALUES (?,?)", p.ID, id).Error; err != nil {
			return err
		}
	}
	return nil
}

func clearRevision(tx *gorm.DB, id string) error {
	if err := tx.Exec("DELETE FROM post_revision_tags WHERE post_id=?", id).Error; err != nil {
		return err
	}
	return tx.Where("post_id=?", id).Delete(&Revision{}).Error
}
