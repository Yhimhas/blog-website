package blog

import (
	"sort"
	"strings"
	"time"
)

type Term struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// Summary is the public list DTO; storage-only fields never reach JSON.
type Summary struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	Category    *Term     `json:"category"`
	Tags        []Term    `json:"tags"`
	PublishedAt time.Time `json:"publishedAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Detail struct {
	Summary
	ContentMarkdown string `json:"contentMarkdown"`
}

// post is deliberately separate from the public DTOs.
type post struct {
	detail Detail
	status string
}

type Service struct{ posts []post }

type Filter struct {
	Page, PageSize   int
	Q, Category, Tag string
}

// NewDemoService provides immutable, in-memory examples, not persisted content.
func NewDemoService() *Service {
	timestamp := time.Date(2026, 9, 21, 2, 0, 0, 0, time.UTC)
	return &Service{posts: []post{
		{detail: Detail{Summary: Summary{
			ID: "post-001", Slug: "building", Title: "从一个组件开始", Summary: "记录网站开发过程。",
			Category:    &Term{ID: "cat-001", Slug: "development", Name: "开发笔记"},
			Tags:        []Term{{ID: "tag-001", Slug: "go", Name: "Go"}},
			PublishedAt: timestamp, UpdatedAt: timestamp,
		}, ContentMarkdown: "# 从一个组件开始\n\n这是内存示例文章，记录网站开发过程。"}, status: "published"},
		{detail: Detail{Summary: Summary{
			ID: "post-002", Slug: "hello-go", Title: "Hello Go", Summary: "用标准库建立只读 API。",
			Tags: []Term{}, PublishedAt: timestamp, UpdatedAt: timestamp,
		}, ContentMarkdown: "# Hello Go\n\n使用 net/http 和 encoding/json 返回文章。"}, status: "published"},
		{detail: Detail{Summary: Summary{ID: "post-003", Slug: "draft-note", Title: "未发布笔记"}, ContentMarkdown: "草稿正文"}, status: "draft"},
	}}
}

func public(p post) bool { return p.status == "published" && !p.detail.PublishedAt.IsZero() }

// List applies literal, case-insensitive search and AND filters before pagination.
// The HTTP layer validates Page and PageSize before calling List.
func (s *Service) List(f Filter) ([]Summary, int) {
	items := make([]Summary, 0)
	q := strings.ToLower(f.Q)
	for _, p := range s.posts {
		v := p.detail.Summary
		if !public(p) || (q != "" && !strings.Contains(strings.ToLower(v.Title), q) && !strings.Contains(strings.ToLower(v.Summary), q)) {
			continue
		}
		if f.Category != "" && (v.Category == nil || v.Category.Slug != f.Category) {
			continue
		}
		if f.Tag != "" {
			found := false
			for _, tag := range v.Tags {
				if tag.Slug == f.Tag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		items = append(items, v)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].PublishedAt.Equal(items[j].PublishedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].PublishedAt.After(items[j].PublishedAt)
	})
	total := len(items)
	// Check before multiplication so even the largest valid page cannot overflow.
	if total == 0 || f.Page > (total-1)/f.PageSize+1 {
		return []Summary{}, total
	}
	start := (f.Page - 1) * f.PageSize
	end := min(start+f.PageSize, total)
	return items[start:end], total
}

func (s *Service) Find(slug string) (Detail, bool) {
	for _, p := range s.posts {
		if public(p) && p.detail.Slug == slug {
			return p.detail, true
		}
	}
	return Detail{}, false
}
