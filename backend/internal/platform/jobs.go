package platform

import (
	"blog-website/backend/internal/music"
	"blog-website/backend/internal/recommendation"
	"context"
	"log/slog"
	"time"
)

// Startup only compensates today; subsequent dates run after 00:10 Shanghai.
// Each generation has three attempts, separated by 1m and 2m. No unbounded retry.
func RunJobs(ctx context.Context, m *music.Service, r recommendation.Service, logger *slog.Logger, autoSync bool) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	date := ""
	attempt := 0
	var next time.Time
	startup := true
	for {
		now := time.Now().In(recommendation.Shanghai)
		today := recommendation.Date(now)
		if today != date {
			date = today
			attempt = 0
			next = time.Time{}
		}
		jobCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		if err := m.Recover(jobCtx); err != nil && ctx.Err() == nil {
			logger.Warn("sync recovery unavailable")
		}
		if attempt < 3 && !now.Before(next) && (startup || now.Hour() > 0 || now.Minute() >= 10) {
			attempt++
			if autoSync {
				syncCtx, done := context.WithTimeout(ctx, 45*time.Second)
				if err := m.SyncDaily(syncCtx, now); err != nil {
					logger.Warn("daily sync unavailable", "date", date)
				}
				done()
				cancel()
				jobCtx, cancel = context.WithTimeout(ctx, 20*time.Second)
			}
			if err := r.Generate(jobCtx, now); err != nil {
				logger.Warn("recommendation generation failed", "date", date, "attempt", attempt)
				next = now.Add(time.Duration(attempt) * time.Minute)
			} else {
				attempt = 3
				logger.Info("recommendation ready", "date", date)
			}
		}
		cancel()
		startup = false
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
