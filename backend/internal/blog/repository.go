package blog

import (
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalid = errors.New("invalid article")
var ErrConflict = errors.New("version conflict")
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type Record struct {
	ID              string     `json:"id"`
	Slug            string     `json:"slug"`
	Title           string     `json:"title"`
	Summary         string     `json:"summary"`
	ContentMarkdown string     `json:"contentMarkdown"`
	CategoryID      *string    `json:"categoryId"`
	Status          string     `json:"status"`
	Version         int64      `json:"version"`
	PublishedAt     *time.Time `json:"publishedAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	TagIDs          []string   `json:"tagIds" gorm:"-"`
}

func (Record) TableName() string { return "posts" }

type Repository struct{ DB *gorm.DB }

func ValidRecord(p Record) bool {
	return len(p.Slug) <= 100 && slugPattern.MatchString(p.Slug) && validText(p.Title, 160) && strings.TrimSpace(p.Title) != "" && validText(p.Summary, 500) && utf8.ValidString(p.ContentMarkdown) && !strings.ContainsRune(p.ContentMarkdown, 0) && len(p.ContentMarkdown) <= 200*1024 && len(p.TagIDs) <= 100
}
func validText(s string, n int) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0) && utf8.RuneCountInString(s) <= n
}

func (r Repository) Terms(ctx context.Context, table string) ([]Term, error) {
	if table != "categories" && table != "tags" {
		return nil, ErrInvalid
	}
	out := []Term{}
	err := r.DB.WithContext(ctx).Table(table).Order("name,id").Find(&out).Error
	return out, err
}
func (r Repository) tags(ctx context.Context, id string) ([]Term, error) {
	out := []Term{}
	err := r.DB.WithContext(ctx).Table("tags t").Select("t.*").Joins("JOIN post_tags pt ON pt.tag_id=t.id").Where("pt.post_id=?", id).Order("t.name,t.id").Scan(&out).Error
	return out, err
}
func (r Repository) Public(ctx context.Context, p Record) (Summary, error) {
	v := Summary{ID: p.ID, Slug: p.Slug, Title: p.Title, Summary: p.Summary, UpdatedAt: p.UpdatedAt.UTC(), Tags: []Term{}}
	if p.PublishedAt != nil {
		v.PublishedAt = p.PublishedAt.UTC()
	}
	if p.CategoryID != nil {
		var term Term
		if err := r.DB.WithContext(ctx).Table("categories").Where("id=?", *p.CategoryID).Take(&term).Error; err != nil {
			return v, err
		}
		v.Category = &term
	}
	var err error
	v.Tags, err = r.tags(ctx, p.ID)
	return v, err
}
func (r Repository) FindPublic(ctx context.Context, slug string) (Detail, error) {
	var p Record
	err := r.DB.WithContext(ctx).Where("slug=? AND status='published' AND published_at IS NOT NULL", slug).Take(&p).Error
	if err != nil {
		return Detail{}, err
	}
	v, err := r.Public(ctx, p)
	return Detail{Summary: v, ContentMarkdown: p.ContentMarkdown}, err
}
func escapedSearch(s string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(s) + "%"
}
func (r Repository) ListPublic(ctx context.Context, f Filter) ([]Summary, int64, error) {
	out := []Summary{}
	var total int64
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&Record{}).Where("status='published' AND published_at IS NOT NULL")
		if f.Q != "" {
			pattern := escapedSearch(f.Q)
			q = q.Where("(title ILIKE ? OR summary ILIKE ?)", pattern, pattern)
		}
		if f.Category != "" {
			q = q.Where("category_id IN (SELECT id FROM categories WHERE slug=?)", f.Category)
		}
		if f.Tag != "" {
			q = q.Where("id IN (SELECT post_id FROM post_tags JOIN tags ON tags.id=post_tags.tag_id WHERE tags.slug=?)", f.Tag)
		}
		if err := q.Count(&total).Error; err != nil {
			return err
		}
		if total == 0 || int64(f.Page-1) > (total-1)/int64(f.PageSize) {
			return nil
		}
		var records []Record
		if err := q.Order("published_at DESC,id DESC").Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Find(&records).Error; err != nil {
			return err
		}
		repo := Repository{tx}
		for _, p := range records {
			v, err := repo.Public(ctx, p)
			if err != nil {
				return err
			}
			out = append(out, v)
		}
		return nil
	})
	return out, total, err
}
func (r Repository) Get(ctx context.Context, id string) (Record, error) {
	var p Record
	err := r.DB.WithContext(ctx).Where("id=?", id).Take(&p).Error
	if err != nil {
		return p, err
	}
	p.TagIDs = []string{}
	err = r.DB.WithContext(ctx).Table("post_tags").Where("post_id=?", id).Order("tag_id").Pluck("tag_id", &p.TagIDs).Error
	return p, err
}
func (r Repository) ListAdmin(ctx context.Context, page, size int, status string) ([]Record, int64, error) {
	out := []Record{}
	var total int64
	q := r.DB.WithContext(ctx).Model(&Record{})
	if status != "" {
		q = q.Where("status=?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 || int64(page-1) > (total-1)/int64(size) {
		return out, total, nil
	}
	if err := q.Order("updated_at DESC,id DESC").Limit(size).Offset((page - 1) * size).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	for i := range out {
		p, err := r.Get(ctx, out[i].ID)
		if err != nil {
			return nil, 0, err
		}
		out[i] = p
	}
	return out, total, nil
}
func setTags(tx *gorm.DB, p Record) error {
	seen := map[string]bool{}
	for _, id := range p.TagIDs {
		if id == "" || seen[id] {
			return ErrInvalid
		}
		seen[id] = true
	}
	if err := tx.Exec("DELETE FROM post_tags WHERE post_id=?", p.ID).Error; err != nil {
		return err
	}
	for _, id := range p.TagIDs {
		if err := tx.Exec("INSERT INTO post_tags(post_id,tag_id) VALUES (?,?)", p.ID, id).Error; err != nil {
			return err
		}
	}
	return nil
}
func (r Repository) Create(ctx context.Context, p Record) (Record, error) {
	if !ValidRecord(p) {
		return Record{}, ErrInvalid
	}
	p.Status = "draft"
	p.Version = 1
	p.PublishedAt = nil
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&p).Error; err != nil {
			return err
		}
		return setTags(tx, p)
	})
	return p, err
}

// ApplyPatch preserves omitted fields; only categoryId accepts null.
func ApplyPatch(p *Record, fields map[string]json.RawMessage) error {
	for k, v := range fields {
		if string(v) == "null" && k != "categoryId" {
			return ErrInvalid
		}
		var target any
		switch k {
		case "version":
			continue
		case "title":
			target = &p.Title
		case "summary":
			target = &p.Summary
		case "contentMarkdown":
			target = &p.ContentMarkdown
		case "categoryId":
			target = &p.CategoryID
		case "tagIds":
			target = &p.TagIDs
		default:
			return ErrInvalid
		}
		if err := json.Unmarshal(v, target); err != nil {
			return ErrInvalid
		}
	}
	if !ValidRecord(*p) {
		return ErrInvalid
	}
	return nil
}
func (r Repository) Change(ctx context.Context, id string, version int64, action string, fields map[string]json.RawMessage) (Record, error) {
	var result Record
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p Record
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", id).Take(&p).Error; err != nil {
			return err
		}
		if version < 1 || p.Version != version {
			return ErrConflict
		}
		repo := Repository{tx}
		var err error
		p, err = repo.Get(ctx, id)
		if err != nil {
			return err
		}
		switch action {
		case "patch":
			if err := ApplyPatch(&p, fields); err != nil {
				return err
			}
		case "publish":
			if !ValidRecord(p) || strings.TrimSpace(p.ContentMarkdown) == "" {
				return ErrInvalid
			}
			p.Status = "published"
			if p.PublishedAt == nil {
				now := time.Now().UTC()
				p.PublishedAt = &now
			}
		case "archive":
			p.Status = "archived"
		default:
			return ErrInvalid
		}
		p.Version++
		p.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&p).Error; err != nil {
			return err
		}
		if action == "patch" {
			if err := setTags(tx, p); err != nil {
				return err
			}
		}
		result = p
		return nil
	})
	return result, err
}
