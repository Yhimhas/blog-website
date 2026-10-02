package storage_test

import (
	"blog-website/backend/internal/blog"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestBlogManagement(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	repo := blog.Repository{DB: db}
	category, err := repo.SaveTerm(ctx, "categories", blog.Term{ID: "cat", Slug: "dev", Name: "开发"}, true)
	if err != nil {
		t.Fatal(err)
	}
	tag, err := repo.SaveTerm(ctx, "tags", blog.Term{ID: "tag", Slug: "go", Name: "Go"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SaveTerm(ctx, "tags", blog.Term{ID: "duplicate", Slug: "go", Name: "重复"}, true); !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Fatal("duplicate slug accepted", err)
	}
	category.Name = "开发笔记"
	if _, err := repo.SaveTerm(ctx, "categories", category, false); err != nil {
		t.Fatal(err)
	}
	p, err := repo.Create(ctx, blog.Record{ID: "post", Slug: "article", Title: "公开标题", Summary: "公开摘要", ContentMarkdown: "公开正文", TagIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	p, err = repo.Change(ctx, p.ID, p.Version, "publish", nil)
	if err != nil {
		t.Fatal(err)
	}
	public, err := repo.FindPublic(ctx, p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	patch := map[string]json.RawMessage{"title": json.RawMessage(`"修订标题"`), "summary": json.RawMessage(`"修订摘要"`), "contentMarkdown": json.RawMessage(`"修订正文"`), "categoryId": json.RawMessage(`"cat"`), "tagIds": json.RawMessage(`["tag"]`)}
	p, err = repo.Change(ctx, p.ID, p.Version, "patch", patch)
	if err != nil || p.Revision == nil || p.Revision.Title != "修订标题" || p.Title != public.Title {
		t.Fatal("revision not isolated", p, err)
	}
	visible, err := repo.FindPublic(ctx, p.Slug)
	if err != nil || visible.Title != public.Title || visible.Summary.Summary != public.Summary.Summary || visible.ContentMarkdown != public.ContentMarkdown || visible.Category != nil || len(visible.Tags) != 0 || !visible.UpdatedAt.Equal(public.UpdatedAt) {
		t.Fatal("save changed public content", visible, err)
	}
	for _, table := range []string{"categories", "tags"} {
		id := category.ID
		if table == "tags" {
			id = tag.ID
		}
		if err := repo.DeleteTerm(ctx, table, id); !errors.Is(err, blog.ErrTermInUse) {
			t.Fatal("revision reference deleted", err)
		}
	}
	if _, err := repo.Change(ctx, p.ID, p.Version-1, "publish", nil); !errors.Is(err, blog.ErrConflict) {
		t.Fatal("stale publish accepted", err)
	}
	if _, err := repo.Change(ctx, p.ID, p.Version, "patch", map[string]json.RawMessage{"title": json.RawMessage(`"must rollback"`), "tagIds": json.RawMessage(`["missing"]`)}); err == nil {
		t.Fatal("missing revision tag accepted")
	}
	loaded, err := repo.Get(ctx, p.ID)
	if err != nil || loaded.Version != p.Version || loaded.Revision.Title != "修订标题" || len(loaded.Revision.TagIDs) != 1 {
		t.Fatal("revision rollback failed", loaded, err)
	}
	p, err = repo.Change(ctx, p.ID, p.Version, "patch", map[string]json.RawMessage{"contentMarkdown": json.RawMessage(`""`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Change(ctx, p.ID, p.Version, "publish", nil); !errors.Is(err, blog.ErrInvalid) {
		t.Fatal("empty revision published", err)
	}
	visible, err = repo.FindPublic(ctx, p.Slug)
	if err != nil || visible.ContentMarkdown != public.ContentMarkdown {
		t.Fatal("failed publication changed public article", err)
	}
	p, err = repo.Change(ctx, p.ID, p.Version, "patch", map[string]json.RawMessage{"contentMarkdown": json.RawMessage(`"修订正文"`)})
	if err != nil || p.Revision.Title != "修订标题" {
		t.Fatal("partial patch lost revision fields", err)
	}
	p, err = repo.Change(ctx, p.ID, p.Version, "archive", nil)
	if err != nil || p.Revision == nil {
		t.Fatal("archive lost revision", err)
	}
	if _, err := repo.FindPublic(ctx, p.Slug); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("archive exposed", err)
	}
	items, total, err := repo.ListAdmin(ctx, 1, 20, "archived")
	if err != nil || total != 1 || len(items) != 1 || items[0].Revision == nil {
		t.Fatal("archive filter", err)
	}
	p, err = repo.Change(ctx, p.ID, p.Version, "publish", nil)
	if err != nil || p.Revision != nil {
		t.Fatal("revision not consumed", err)
	}
	visible, err = repo.FindPublic(ctx, p.Slug)
	if err != nil || visible.Title != "修订标题" || visible.ContentMarkdown != "修订正文" || visible.Summary.Summary != "修订摘要" || visible.Category == nil || visible.Category.Name != category.Name || len(visible.Tags) != 1 || visible.Tags[0].ID != tag.ID || !visible.PublishedAt.Equal(public.PublishedAt) {
		t.Fatal("publication incomplete", visible, err)
	}
	if err := repo.DeleteTerm(ctx, "tags", tag.ID); !errors.Is(err, blog.ErrTermInUse) {
		t.Fatal("published reference deleted", err)
	}
	p, err = repo.Change(ctx, p.ID, p.Version, "patch", map[string]json.RawMessage{"categoryId": json.RawMessage(`null`), "tagIds": json.RawMessage(`[]`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Change(ctx, p.ID, p.Version, "publish", nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteTerm(ctx, "categories", category.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteTerm(ctx, "tags", tag.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteTerm(ctx, "tags", tag.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("missing term delete", err)
	}
}
