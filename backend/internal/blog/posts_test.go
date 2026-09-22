package blog

import (
	"testing"
	"time"
)

func TestPublicationAndDateOrder(t *testing.T) {
	s := NewDemoService()
	newer := s.posts[0]
	newer.detail.ID = "post-000"
	newer.detail.Slug = "newer"
	newer.detail.PublishedAt = newer.detail.PublishedAt.Add(time.Hour)
	archived := newer
	archived.detail.Slug = "archived"
	archived.status = "archived"
	missingDate := newer
	missingDate.detail.Slug = "missing-date"
	missingDate.detail.PublishedAt = time.Time{}
	s.posts = append(s.posts, newer, archived, missingDate)
	items, total := s.List(Filter{Page: 1, PageSize: 20})
	if total != 3 || items[0].Slug != "newer" || items[1].Slug != "hello-go" {
		t.Fatalf("order/isolation: %+v", items)
	}
	for _, slug := range []string{"draft-note", "archived", "missing-date"} {
		if _, ok := s.Find(slug); ok {
			t.Fatalf("public detail leaked %s", slug)
		}
	}
}
