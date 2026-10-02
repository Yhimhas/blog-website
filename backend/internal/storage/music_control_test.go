package storage_test

import (
	"blog-website/backend/internal/music"
	"blog-website/backend/internal/music/playback"
	"blog-website/backend/internal/music/provider"
	"blog-website/backend/internal/recommendation"
	"context"
	"testing"
	"time"
)

func TestSharedPlaybackControlsAndAvailabilityScopes(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	a := music.New(db, nil, ctx)
	b := music.New(db, nil, ctx)
	if err := a.Lease(ctx, "streaming", "one", "", 1, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := b.Lease(ctx, "streaming", "two", "", 1, time.Minute); err != playback.Busy {
		t.Fatal("global stream cap bypassed", err)
	}
	a.ReleaseLease(ctx, "streaming", "one")
	if err := b.Lease(ctx, "streaming", "two", "", 1, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := a.Spend(ctx, "fixture", 6, 10, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := b.Spend(ctx, "fixture", 5, 10, time.Hour); err != playback.BudgetExceeded {
		t.Fatal("global byte budget bypassed", err)
	}
	if err := a.Rate(ctx, "shared-ip", 1, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := b.Rate(ctx, "shared-ip", 1, time.Minute); err != playback.BudgetExceeded {
		t.Fatal("shared NAT quota bypassed", err)
	}
	if err := b.Rate(ctx, "different-ip", 1, time.Minute); err != nil {
		t.Fatal("different visitor shares proxy budget", err)
	}
	source := music.Source{ID: "netease:123", ExternalID: "123", Provider: "netease", Title: "scope fixture", SourceURL: "https://music.163.com/playlist?id=123"}
	track := provider.Track{ID: "netease:123", ExternalID: "123", Provider: "netease", Title: "scope fixture", SourceURL: "https://music.163.com/song?id=123", Availability: "unknown"}
	if err := music.Import(ctx, db, source, []provider.Track{track}); err != nil {
		t.Fatal(err)
	}
	a.RecordPlayback(ctx, track.ID, playback.PlaybackCheck{SourceMode: "account", Audience: "private", MediaKind: "full", CredentialVersion: "FIXTURE_VERSION"})
	got, _, err := a.Tracks(ctx, source.ID, 1, 20)
	if err != nil || got[0].Availability != "unknown" {
		t.Fatal("private account availability polluted public track", got, err)
	}
	a.RecordPlayback(ctx, track.ID, playback.PlaybackCheck{SourceMode: "public", Audience: "public", MediaKind: "preview"})
	rec := recommendation.Service{DB: db}
	day := time.Date(2026, 10, 2, 2, 0, 0, 0, time.UTC)
	if err := rec.Generate(ctx, day); err != nil {
		t.Fatal(err)
	}
	result, err := rec.Get(ctx, recommendation.Date(day))
	if err != nil || len(result.Items) != 0 {
		t.Fatal("preview/private audio became a public recommendation", result, err)
	}
}
