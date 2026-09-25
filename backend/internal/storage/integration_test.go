package storage_test

import (
	"blog-website/backend/internal/auth"
	"blog-website/backend/internal/blog"
	"blog-website/backend/internal/music"
	"blog-website/backend/internal/music/provider"
	"blog-website/backend/internal/recommendation"
	"blog-website/backend/internal/storage"
	"blog-website/backend/migrations"
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

// Opt-in only. Every run creates a unique schema in an explicitly designated test
// database, and retains it for inspection (no DROP/cleanup under this user's policy).
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not configured; PostgreSQL integration not executed")
	}
	if os.Getenv("ALLOW_TEST_SCHEMA_CREATE") != "true" {
		t.Fatal("set ALLOW_TEST_SCHEMA_CREATE=true for an isolated test database")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		t.Fatal("TEST_DATABASE_URL must be a postgres URL")
	}
	db, err := storage.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	defer pool.Close()
	schema := "backend_test_" + auth.Token()[:16]
	if err := db.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Log("Retained test schema:", schema)
	q := parsed.Query()
	q.Set("search_path", schema)
	parsed.RawQuery = q.Encode()
	isolated, err := storage.Open(parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	isolatedPool, _ := isolated.DB()
	t.Cleanup(func() { isolatedPool.Close() })
	sql, err := migrations.Files.ReadFile("000001_initial.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err = isolated.Exec(string(sql)).Error; err != nil {
		t.Fatal(err)
	}
	return isolated
}
func TestPostgresContracts(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	repo := blog.Repository{DB: db}
	if err := storage.Ready(ctx, db); err != nil {
		t.Fatal(err)
	}
	p, err := repo.Create(ctx, blog.Record{ID: auth.Token(), Slug: "integration", Title: "100% Go", Summary: "literal_underscore", ContentMarkdown: "# body", TagIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindPublic(ctx, p.Slug); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("draft exposed")
	}
	published, err := repo.Change(ctx, p.ID, 1, "publish", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Change(ctx, p.ID, 1, "archive", nil); !errors.Is(err, blog.ErrConflict) {
		t.Fatal("stale version accepted")
	}
	if _, err := repo.Change(ctx, p.ID, 2, "patch", map[string]json.RawMessage{"title": json.RawMessage(`"should roll back"`), "tagIds": json.RawMessage(`["missing-tag"]`)}); err == nil {
		t.Fatal("foreign key accepted")
	}
	unchanged, err := repo.Get(ctx, p.ID)
	if err != nil || unchanged.Title != p.Title || unchanged.Version != 2 {
		t.Fatal("transaction failed to roll back", err)
	}
	items, total, err := repo.ListPublic(ctx, blog.Filter{Page: 1, PageSize: 20, Q: "%"})
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatal("literal search", err)
	}
	archived, err := repo.Change(ctx, p.ID, 2, "archive", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindPublic(ctx, p.Slug); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("archive exposed")
	}
	again, err := repo.Change(ctx, p.ID, archived.Version, "publish", nil)
	if err != nil || !again.PublishedAt.Equal(*published.PublishedAt) {
		t.Fatal("first publication not retained")
	}
	// Two writers using the same version: exactly one can succeed.
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Change(ctx, p.ID, again.Version, "archive", nil)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, blog.ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("optimistic locking")
	}
	if err := auth.CreateUser(ctx, db, "owner", "integration-only-password"); err != nil {
		t.Fatal(err)
	}
	sessions := auth.Service{DB: db, TTL: time.Hour}
	_, token, err := sessions.Login(ctx, "owner", "integration-only-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sessions.Current(ctx, token); err != nil {
		t.Fatal(err)
	}
	if err = sessions.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err = sessions.Current(ctx, token); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatal("expired session accepted")
	}

	source := music.Source{ID: "netease:595975585", Provider: "netease", ExternalID: "595975585", Title: "test", SourceURL: "https://music.163.com/playlist?id=595975585"}
	tracks := []provider.Track{{ID: "netease:123", Provider: "netease", ExternalID: "123", Title: "original", SourceURL: "https://music.163.com/#/song?id=123", Availability: "available"}}
	if err := music.Import(ctx, db, source, tracks); err != nil {
		t.Fatal(err)
	}
	blocking := &blockingAdapter{entered: make(chan struct{}), release: make(chan struct{})}
	serviceCtx, cancel := context.WithCancel(ctx)
	ms := music.New(db, blocking, serviceCtx)
	defer func() { cancel(); ms.Stop() }()
	first, err := ms.StartSync(ctx, source.ID, "test")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-blocking.entered:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	if _, err := ms.StartSync(ctx, source.ID, "second"); !errors.Is(err, music.ErrBusy) {
		t.Fatal("duplicate sync accepted", err)
	}
	close(blocking.release)
	deadline := time.Now().Add(5 * time.Second)
	for {
		run, err := ms.GetRun(ctx, first.ID)
		if err != nil {
			t.Fatal(err)
		}
		if run.Status == "failed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, count, err := ms.Tracks(ctx, source.ID, 1, 20)
	if err != nil || count != 1 || got[0].Title != "original" {
		t.Fatal("upstream failure lost snapshot", err)
	}
	if err := db.Exec("ALTER TABLE music_items ADD CONSTRAINT reject_test_title CHECK (title <> 'reject')").Error; err != nil {
		t.Fatal(err)
	}
	changed := append([]provider.Track{}, tracks...)
	changed[0].Title = "changed"
	changed = append(changed, provider.Track{ID: "netease:456", Provider: "netease", ExternalID: "456", Title: "reject", SourceURL: "https://music.163.com/#/song?id=456", Availability: "available"})
	if err := music.Import(ctx, db, source, changed); err == nil {
		t.Fatal("expected mid-snapshot failure")
	}
	got, count, err = ms.Tracks(ctx, source.ID, 1, 20)
	if err != nil || count != 1 || got[0].Title != "original" {
		t.Fatal("partial snapshot committed")
	}
	rec := recommendation.Service{DB: db}
	now := time.Date(2026, 9, 25, 1, 0, 0, 0, time.UTC)
	errorsOut := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); errorsOut <- rec.Generate(ctx, now) }()
	}
	wg.Wait()
	close(errorsOut)
	for err := range errorsOut {
		if err != nil {
			t.Fatal(err)
		}
	}
	result, err := rec.Get(ctx, recommendation.Date(now))
	if err != nil || len(result.Items) != 1 || result.Items[0].ID != "netease:123" {
		t.Fatal("recommendation incomplete", err)
	}
	var rows int64
	db.Table("daily_recommendations").Count(&rows)
	if rows != 1 {
		t.Fatal("duplicate recommendation")
	}
	if err := music.Import(ctx, db, source, []provider.Track{}); err != nil {
		t.Fatal(err)
	}
	if err := rec.Generate(ctx, now); err != nil {
		t.Fatal(err)
	}
	result, err = rec.Get(ctx, recommendation.Date(now))
	if err != nil || len(result.Items) != 1 {
		t.Fatal("saved recommendation changed")
	}
}

type blockingAdapter struct{ entered, release chan struct{} }

func (b *blockingAdapter) FetchPlaylist(ctx context.Context, _ provider.Ref) ([]provider.Track, error) {
	close(b.entered)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.release:
		return nil, provider.Failure("UPSTREAM_TIMEOUT")
	}
}
