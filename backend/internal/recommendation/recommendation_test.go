package recommendation

import (
	"blog-website/backend/internal/music/provider"
	"math/rand/v2"
	"testing"
	"time"
)

func TestSelectionFallbackAndDate(t *testing.T) {
	rng := func() *rand.Rand { return rand.New(rand.NewPCG(1, 2)) }
	a, b := "author-a", "author-b"
	pool := []provider.Track{{ID: "1", Availability: "available", Author: &a}, {ID: "2", Availability: "available", Author: &a}, {ID: "3", Availability: "available", Author: &b}, {ID: "4", Availability: "unknown"}, {ID: "1", Availability: "available", Author: &a}}
	got := Select(pool, map[string]int{"1": 1, "2": 7}, 3, rng())
	if len(got) != 3 || got[0].ID != "3" {
		t.Fatalf("recent-first selection %+v", got)
	}
	seen := map[string]bool{}
	for _, v := range got {
		if seen[v.ID] || v.ID == "4" {
			t.Fatal("invalid candidate")
		}
		seen[v.ID] = true
	}
	if len(Select(nil, nil, 3, rng())) != 0 || len(Select(pool, nil, 0, rng())) != 0 {
		t.Fatal("empty selection")
	}
	now := time.Date(2026, 9, 24, 16, 0, 0, 0, time.UTC)
	if Date(now) != "2026-09-25" || Date(now.Add(-time.Second)) != "2026-09-24" {
		t.Fatal("Shanghai date boundary")
	}
}
