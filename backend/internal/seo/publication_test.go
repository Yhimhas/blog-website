package seo

import (
	"blog-website/backend/internal/blog"
	"blog-website/backend/internal/storage"
	"blog-website/backend/migrations"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func publicationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not configured; PostgreSQL SEO integration not executed")
	}
	if os.Getenv("ALLOW_TEST_SCHEMA_CREATE") != "true" {
		t.Fatal("set ALLOW_TEST_SCHEMA_CREATE=true for an isolated test database")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme != "postgres" && u.Scheme != "postgresql" {
		t.Fatal("TEST_DATABASE_URL must be a postgres URL")
	}
	db, err := storage.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	defer pool.Close()
	id, err := uniqueID()
	if err != nil {
		t.Fatal(err)
	}
	schema := "seo_test_" + id
	if err := db.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Log("Retained SEO test schema:", schema)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err = storage.Open(u.String())
	if err != nil {
		t.Fatal(err)
	}
	isolated, _ := db.DB()
	t.Cleanup(func() { isolated.Close() })
	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		content, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(string(content)).Error; err != nil {
			t.Fatal(entry.Name(), err)
		}
	}
	return db
}

func TestPublicationEventsAreTransactionalAndExcludeDraftRevisions(t *testing.T) {
	db := publicationDB(t)
	ctx := context.Background()
	repo := blog.Repository{DB: db}
	check := func(want int64) {
		t.Helper()
		p, err := Read(ctx, db)
		if err != nil || p.Revision != want || p.AppliedRevision != 0 {
			t.Fatal("publication revision", p, err, "want", want)
		}
	}
	check(1)
	p, err := repo.Create(ctx, blog.Record{ID: "article", Slug: "article", Title: "Original", ContentMarkdown: "body", TagIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	check(1)
	p, err = repo.Change(ctx, p.ID, p.Version, "publish", nil)
	if err != nil {
		t.Fatal(err)
	}
	check(2)
	p, err = repo.Change(ctx, p.ID, p.Version, "patch", map[string]json.RawMessage{"title": json.RawMessage(`"Revision"`)})
	if err != nil {
		t.Fatal(err)
	}
	check(2)
	if _, err := repo.Change(ctx, p.ID, p.Version-1, "publish", nil); !errors.Is(err, blog.ErrConflict) {
		t.Fatal(err)
	}
	check(2)
	// Even an event emitted by the database trigger disappears on rollback.
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&blog.Record{}).Where("id=?", p.ID).Update("title", "rolled back").Error; err != nil {
			return err
		}
		return errors.New("rollback")
	})
	if err == nil {
		t.Fatal("rollback failed")
	}
	check(2)
	for i, action := range []string{"publish", "archive", "publish"} {
		p, err = repo.Change(ctx, p.ID, p.Version, action, nil)
		if err != nil {
			t.Fatal(err)
		}
		check(int64(3 + i))
	}
}

func TestActivationRejectsStaleAndBlocksPublicCommitUntilSwitch(t *testing.T) {
	db := publicationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, err := (blog.Repository{DB: db}).Create(ctx, blog.Record{ID: "article", Slug: "article", Title: "title", ContentMarkdown: "body", TagIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := activate(ctx, db, "0", func() error { t.Fatal("stale switched"); return nil }); !errors.Is(err, ErrStale) {
		t.Fatal(err)
	}
	switchStarted, allowSwitch := make(chan struct{}), make(chan struct{})
	activation := make(chan error, 1)
	go func() {
		activation <- activate(ctx, db, "1", func() error {
			close(switchStarted)
			select {
			case <-allowSwitch:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	<-switchStarted
	writeDone := make(chan error, 1)
	go func() {
		_, err := (blog.Repository{DB: db}).Change(ctx, p.ID, p.Version, "publish", nil)
		writeDone <- err
	}()
	select {
	case err := <-writeDone:
		t.Fatal("public commit crossed activation lock", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(allowSwitch)
	if err := <-activation; err != nil {
		t.Fatal(err)
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	state, err := Read(ctx, db)
	if err != nil || state.Revision != 2 || state.AppliedRevision != 1 {
		t.Fatal("new public event was lost", state, err)
	}
	if err := activate(ctx, db, "2", func() error { return errors.New("switch failed") }); err == nil {
		t.Fatal("switch failure ignored")
	}
	state, _ = Read(ctx, db)
	if state.AppliedRevision != 1 {
		t.Fatal("failed switch was acknowledged")
	}
}

func TestPublisherLeaseSingleFlightAndReleaseOnFailure(t *testing.T) {
	db := publicationDB(t)
	ctx := context.Background()
	if err := withLease(ctx, db, func(conn *gorm.DB) error {
		if err := withLease(ctx, db, func(*gorm.DB) error { t.Fatal("second publisher entered"); return nil }); !errors.Is(err, ErrBusy) {
			t.Fatal(err)
		}
		return errors.New("simulated build failure")
	}); err == nil {
		t.Fatal("work failure ignored")
	}
	if err := withLease(ctx, db, func(*gorm.DB) error { return nil }); err != nil {
		t.Fatal("lease leaked after failure", err)
	}
}

func TestWorkerRetriesCoalescesAndRecoversUnacknowledgedSwitch(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("worker activation requires Linux")
	}
	db := publicationDB(t)
	base := retainedDir(t)
	ctx := context.Background()
	w := Worker{DB: db, Origin: testOrigin, Layout: Layout{Releases: filepath.Join(base, "releases"), Current: filepath.Join(base, "current"), Assets: filepath.Join(base, "assets")}}
	builds := 0
	w.Build = func(ctx context.Context, release string) error {
		builds++
		return errors.New("simulated build failure")
	}
	if err := w.Run(ctx); err == nil {
		t.Fatal("build failure ignored")
	}
	state, _ := Read(ctx, db)
	if state.AppliedRevision != 0 || state.LastError == "" {
		t.Fatal("failure lost pending revision", state)
	}
	w.Build = func(ctx context.Context, release string) error {
		builds++
		state, err := Read(ctx, db)
		if err != nil {
			return err
		}
		fixtureRelease(t, release, strconv.FormatInt(state.Revision, 10), state.SourceID)
		return nil
	}
	if err := w.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if err := w.Run(ctx); err != nil || builds != 2 {
		t.Fatal("unchanged content rebuilt", builds, err)
	}
	// Lost acknowledgement after a successful filesystem switch is recovered
	// without building or removing the current release.
	if err := db.Model(&Publication{}).Where("singleton=true").Update("applied_revision", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := w.Run(ctx); err != nil || builds != 2 {
		t.Fatal("crash recovery rebuilt", builds, err)
	}
	state, _ = Read(ctx, db)
	if state.AppliedRevision != state.Revision || state.LastError != "" {
		t.Fatal("crash recovery did not acknowledge", state)
	}
	// Simulate a new publication after the frontend produced its receipt.
	w.Force = true
	originalBuild := w.Build
	w.Build = func(ctx context.Context, release string) error {
		if err := originalBuild(ctx, release); err != nil {
			return err
		}
		return db.Exec("UPDATE seo_publication SET revision=revision+1 WHERE singleton=true").Error
	}
	if err := w.Run(ctx); !errors.Is(err, ErrStale) {
		t.Fatal("mid-build update activated", err)
	}
	current, err := ReadManifest(w.Layout.Current, testOrigin)
	if err != nil || current.Revision != "1" {
		t.Fatal("stale build changed current", current, err)
	}
	// Additional events coalesce into one latest build on the next timer run.
	if err := db.Exec("UPDATE seo_publication SET revision=revision+1 WHERE singleton=true").Error; err != nil {
		t.Fatal(err)
	}
	w.Force = false
	w.Build = originalBuild
	if err := w.Run(ctx); err != nil {
		t.Fatal("pending updates did not recover", err)
	}
	current, err = ReadManifest(w.Layout.Current, testOrigin)
	if err != nil || current.Revision != "3" || builds != 4 {
		t.Fatal("updates did not coalesce", current, builds, err)
	}
}
