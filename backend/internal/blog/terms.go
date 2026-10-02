package blog

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

var ErrTermInUse = errors.New("term is referenced by articles")

func validTermTable(table string) bool { return table == "categories" || table == "tags" }

func ValidTerm(term Term) bool {
	return len(term.Slug) <= 100 && slugPattern.MatchString(term.Slug) &&
		validText(term.Name, 160) && strings.TrimSpace(term.Name) != ""
}

func (r Repository) SaveTerm(ctx context.Context, table string, term Term, create bool) (Term, error) {
	if !validTermTable(table) || !ValidTerm(term) || term.ID == "" {
		return Term{}, ErrInvalid
	}
	term.Name = strings.TrimSpace(term.Name)
	q := r.DB.WithContext(ctx).Table(table)
	if create {
		return term, q.Create(&term).Error
	}
	result := q.Where("id=?", term.ID).Updates(map[string]any{"name": term.Name, "slug": term.Slug})
	if result.Error != nil {
		return Term{}, result.Error
	}
	if result.RowsAffected == 0 {
		return Term{}, gorm.ErrRecordNotFound
	}
	return term, nil
}

func (r Repository) DeleteTerm(ctx context.Context, table, id string) error {
	if !validTermTable(table) || id == "" {
		return ErrInvalid
	}
	// Foreign keys also protect references created concurrently with this delete.
	result := r.DB.WithContext(ctx).Table(table).Where("id=?", id).Delete(&Term{})
	if errors.Is(result.Error, gorm.ErrForeignKeyViolated) {
		return ErrTermInUse
	}
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
