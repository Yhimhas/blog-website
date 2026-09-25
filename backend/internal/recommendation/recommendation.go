package recommendation

import (
	"blog-website/backend/internal/music"
	"blog-website/backend/internal/music/provider"
	"context"
	"encoding/json"
	"gorm.io/gorm"
	"math/rand/v2"
	"sort"
	"time"
	_ "time/tzdata"
)

var Shanghai = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}()

func Date(now time.Time) string { return now.In(Shanghai).Format("2006-01-02") }

type Result struct {
	Date     string           `json:"date"`
	Timezone string           `json:"timezone"`
	Status   string           `json:"status"`
	Items    []provider.Track `json:"items"`
}
type Service struct{ DB *gorm.DB }

// Select favors never-recently-played tracks, then progressively relaxes the window.
// Randomness is injected; candidates are sorted so input order cannot bias selection.
func Select(candidates []provider.Track, recent map[string]int, count int, rng *rand.Rand) []provider.Track {
	pool := append([]provider.Track{}, candidates...)
	sort.Slice(pool, func(i, j int) bool { return pool[i].ID < pool[j].ID })
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	out := []provider.Track{}
	used := map[string]bool{}
	lastAuthor := ""
	for window := 7; window >= 0 && len(out) < count; window-- {
		for len(out) < count {
			best := -1
			for i, t := range pool {
				if used[t.ID] || t.Availability != "available" {
					continue
				}
				if days, ok := recent[t.ID]; ok && days <= window {
					continue
				}
				if best < 0 {
					best = i
				}
				if t.Author == nil || *t.Author != lastAuthor {
					best = i
					break
				}
			}
			if best < 0 {
				break
			}
			v := pool[best]
			used[v.ID] = true
			out = append(out, v)
			lastAuthor = ""
			if v.Author != nil {
				lastAuthor = *v.Author
			}
		}
	}
	return out
}
func (s Service) Get(ctx context.Context, date string) (Result, error) {
	result := Result{Date: date, Timezone: "Asia/Shanghai", Status: "ready", Items: []provider.Track{}}
	var record struct{ Date string }
	if err := s.DB.WithContext(ctx).Table("daily_recommendations").Where("date=?", date).Take(&record).Error; err != nil {
		return result, err
	}
	var rows []struct{ Snapshot []byte }
	if err := s.DB.WithContext(ctx).Table("daily_recommendation_items").Where("date=?", date).Order("position").Find(&rows).Error; err != nil {
		return result, err
	}
	for _, row := range rows {
		var t provider.Track
		if err := json.Unmarshal(row.Snapshot, &t); err != nil {
			return result, err
		}
		if err := provider.Validate([]provider.Track{t}, t.Provider); err != nil {
			return result, err
		}
		result.Items = append(result.Items, t)
	}
	return result, nil
}
func (s Service) Generate(ctx context.Context, now time.Time) error {
	date := Date(now)
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "recommendation:"+date).Error; err != nil {
			return err
		}
		var n int64
		if err := tx.Table("daily_recommendations").Where("date=?", date).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		tracks, err := music.ReadTracks(tx.Table("music_items m").Select("DISTINCT ON (m.id) m.*,si.source_id AS playlist_id").Joins("JOIN source_items si ON si.item_id=m.id AND si.active JOIN music_sources s ON s.id=si.source_id AND s.enabled").Where("m.availability='available' AND s.synced_at IS NOT NULL").Order("m.id,si.source_id"))
		if err != nil {
			return err
		}
		var history []struct {
			ItemID string
			Days   int
		}
		if err := tx.Raw("SELECT item_id, min(?::date-date) AS days FROM daily_recommendation_items WHERE date >= ?::date - 7 AND date < ?::date GROUP BY item_id", date, date, date).Scan(&history).Error; err != nil {
			return err
		}
		recent := map[string]int{}
		for _, v := range history {
			recent[v.ItemID] = v.Days
		}
		picked := Select(tracks, recent, 3, rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
		if err := tx.Exec("INSERT INTO daily_recommendations(date,timezone,algorithm) VALUES (?,'Asia/Shanghai','recent7-author-v1')", date).Error; err != nil {
			return err
		}
		for i, t := range picked {
			snapshot, err := json.Marshal(t)
			if err != nil {
				return err
			}
			if err := tx.Exec("INSERT INTO daily_recommendation_items(date,position,item_id,snapshot) VALUES (?,?,?,?::jsonb)", date, i, t.ID, string(snapshot)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
