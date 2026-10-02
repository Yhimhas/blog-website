package playback

import (
	"blog-website/backend/internal/auth"
	"blog-website/backend/internal/music/provider"
	"context"
	"time"
)

func (s *Service) resolveAgain(ctx context.Context, track provider.Track) (Audio, error) {
	s.mu.Lock()
	if s.ctx.Err() != nil || s.resolving >= s.options.MaxResolving {
		s.mu.Unlock()
		return Audio{}, Busy
	}
	s.resolving++
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.resolving--; s.mu.Unlock() }()
	id := auth.Token()
	if s.options.Control != nil {
		if err := s.options.Control.Lease(ctx, "resolving", id, "", s.options.MaxResolving, 30*time.Second); err != nil {
			return Audio{}, Busy
		}
		defer s.releaseLease("resolving", id)
	}
	return s.resolver.Resolve(ctx, track)
}

// Production stores implement this interface with shared PostgreSQL controls.
// Session contents remain local: deployments must use sticky routing.
type Control interface {
	Lease(context.Context, string, string, string, int, time.Duration) error
	ReleaseLease(context.Context, string, string)
	Rate(context.Context, string, int, time.Duration) error
	Spend(context.Context, string, int64, int64, time.Duration) error
}

func (o *Options) defaults() {
	if o.OwnerPerMinute <= 0 {
		o.OwnerPerMinute = 10
	}
	if o.IPPerMinute <= 0 {
		o.IPPerMinute = 10
	}
	if o.MaxSessions <= 0 {
		o.MaxSessions = 128
	}
	if o.MaxResolving <= 0 {
		o.MaxResolving = 2
	}
	if o.MaxStreams <= 0 {
		o.MaxStreams = 4
	}
	if o.MaxTranscoders <= 0 {
		o.MaxTranscoders = 2
	}
	if o.MaxStreamBytes <= 0 {
		o.MaxStreamBytes = 256 * 1024 * 1024
	}
	if o.HourlyBytes <= 0 {
		o.HourlyBytes = 1024 * 1024 * 1024
	}
	if o.MaxStreamDuration <= 0 {
		o.MaxStreamDuration = 6 * time.Hour
	}
	if o.Policy.Mode == "" {
		o.Policy.Mode = "public_only"
	}
}

type Metrics struct {
	Requests          int64             `json:"requests"`
	RejectedCreates   int64             `json:"rejectedCreates"`
	ReResolutions     int64             `json:"reResolutions"`
	Preparations      int64             `json:"preparations"`
	PreparationMillis int64             `json:"preparationMillisTotal"`
	FirstAudioSamples int64             `json:"firstAudioSamples"`
	FirstAudioMillis  int64             `json:"firstAudioMillisTotal"`
	OutputBytes       int64             `json:"outputBytes"`
	ActiveStreams     int               `json:"activeStreams"`
	ActiveResolvers   int               `json:"activeResolvers"`
	ActiveTranscoders int               `json:"activeTranscoders"`
	Failures          map[Failure]int64 `json:"failures"`
}

func (s *Service) Metrics() Metrics {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.metrics
	out.Failures = map[Failure]int64{}
	for k, n := range s.metrics.Failures {
		out.Failures[k] = n
	}
	out.ActiveStreams = s.streaming
	out.ActiveResolvers = s.resolving
	out.ActiveTranscoders = len(s.transcoders)
	return out
}
func (s *Service) record(ctx context.Context, id string, a Audio, code Failure) {
	if db, ok := s.store.(ScopedStore); ok {
		db.RecordPlayback(ctx, id, PlaybackCheck{SourceMode: a.SourceMode, Audience: a.Audience, MediaKind: a.Capability.MediaKind, CredentialVersion: a.CredentialVersion, Error: code})
	} else if a.SourceMode != "account" && a.Audience != "private" && code != SourceAuthExpired && code != PublicNotAllowed {
		s.store.Checked(ctx, id, code)
	}
}
func (s *Service) charge(ctx context.Context, v *session, n int) error {
	if err := s.options.Policy.Check(v.Attribution.ID, v.audio); err != nil {
		return err
	}
	s.mu.Lock()
	window := time.Now().Truncate(time.Hour)
	if !window.Equal(s.budgetWindow) {
		s.budgetWindow = window
		s.budgetUsed = 0
	}
	if v.outputBytes+int64(n) > s.options.MaxStreamBytes || s.budgetUsed+int64(n) > s.options.HourlyBytes {
		s.mu.Unlock()
		return BudgetExceeded
	}
	v.outputBytes += int64(n)
	s.budgetUsed += int64(n)
	s.mu.Unlock()
	if s.options.Control != nil {
		return s.options.Control.Spend(ctx, "public-egress", int64(n), s.options.HourlyBytes, time.Hour)
	}
	return nil
}

func (s *Service) releaseLease(kind, id string) {
	if s.options.Control == nil {
		return
	}
	s.cleanupWorkers.Add(1)
	go func() {
		defer s.cleanupWorkers.Done()
		ctx, done := context.WithTimeout(context.Background(), 3*time.Second)
		defer done()
		s.options.Control.ReleaseLease(ctx, kind, id)
	}()
}
