package playback

import (
	"blog-website/backend/internal/auth"
	"blog-website/backend/internal/music/provider"
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

type Failure string

func (e Failure) Error() string { return string(e) }

const (
	NotFound             Failure = "TRACK_NOT_FOUND"
	Unsupported          Failure = "PLAYBACK_UNSUPPORTED"
	Unavailable          Failure = "UPSTREAM_UNAVAILABLE"
	RateLimited          Failure = "UPSTREAM_RATE_LIMITED"
	Timeout              Failure = "PLAYBACK_TIMEOUT"
	Busy                 Failure = "PLAYBACK_BUSY"
	Conflict             Failure = "STREAM_CONFLICT"
	Expired              Failure = "SESSION_EXPIRED"
	SourceUnavailable    Failure = "AUDIO_SOURCE_UNAVAILABLE"
	TranscodeUnavailable Failure = "TRANSCODE_UNAVAILABLE"
	TranscodeFailed      Failure = "TRANSCODE_FAILED"
	EntitlementRequired  Failure = "ENTITLEMENT_REQUIRED"
	SourceAuthExpired    Failure = "SOURCE_AUTH_EXPIRED"
	PublicNotAllowed     Failure = "PUBLIC_PLAYBACK_NOT_ALLOWED"
	AccessRestricted     Failure = "UPSTREAM_ACCESS_RESTRICTED"
	BudgetExceeded       Failure = "PLAYBACK_BUDGET_EXCEEDED"
)

func Code(err error) Failure {
	var f Failure
	if errors.As(err, &f) {
		return f
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return Timeout
	}
	return Unavailable
}

type Audio struct {
	URL, MIME string
	// InputFormat is a fixed FFmpeg demuxer name, never supplied by an API caller.
	InputFormat                                                   string
	Transcode                                                     bool
	RedirectPolicy                                                func(*http.Request, []*http.Request) error
	Headers                                                       map[string]string
	Duration                                                      *int
	Capability                                                    Capability
	SourceMode, Audience, CompletenessEvidence, CredentialVersion string
	Valid                                                         func() bool
}
type Resolvers map[string]Resolver

func (r Resolvers) Resolve(ctx context.Context, t provider.Track) (Audio, error) {
	resolver := r[t.Provider]
	if resolver == nil {
		return Audio{}, Unsupported
	}
	return resolver.Resolve(ctx, t)
}

type Options struct {
	FFmpeg                                                                             string
	ForceTranscode                                                                     bool
	Policy                                                                             Policy
	OwnerPerMinute, IPPerMinute, MaxSessions, MaxResolving, MaxStreams, MaxTranscoders int
	MaxStreamBytes, HourlyBytes                                                        int64
	MaxStreamDuration                                                                  time.Duration
	Control                                                                            Control
}
type Resolver interface {
	Resolve(context.Context, provider.Track) (Audio, error)
}
type Store interface {
	Find(context.Context, string) (provider.Track, error)
	Checked(context.Context, string, Failure)
}
type View struct {
	ID          string         `json:"sessionId"`
	Status      string         `json:"status"`
	StreamURL   string         `json:"streamUrl,omitempty"`
	MIME        string         `json:"mimeType,omitempty"`
	Duration    *int           `json:"durationSeconds"`
	Seek        string         `json:"seekMode"`
	Expires     time.Time      `json:"expiresAt"`
	Attribution provider.Track `json:"attribution"`
	Error       Failure        `json:"errorCode,omitempty"`
	Capability  Capability     `json:"capability"`
}
type session struct {
	View
	owner       string
	audio       Audio
	ctx         context.Context
	cancel      context.CancelFunc
	connected   bool
	retain      time.Time
	created     time.Time
	outputBytes int64
}
type counter struct {
	n     int
	until time.Time
}
type Service struct {
	mu                   sync.Mutex
	sessions             map[string]*session
	limits               map[string]counter
	resolver             Resolver
	store                Store
	ctx                  context.Context
	cancel               context.CancelFunc
	workers              sync.WaitGroup
	cleanupWorkers       sync.WaitGroup
	resolving, streaming int
	pending              map[string]bool
	client               *http.Client
	options              Options
	transcoders          chan struct{}
	metrics              Metrics
	budgetWindow         time.Time
	budgetUsed           int64
}

func New(ctx context.Context, r Resolver, db Store, options ...Options) *Service {
	ctx, cancel := context.WithCancel(ctx)
	s := &Service{ctx: ctx, cancel: cancel, resolver: r, store: db, sessions: map[string]*session{}, limits: map[string]counter{}, pending: map[string]bool{}, client: MediaClient()}
	if len(options) > 0 {
		s.options = options[0]
	}
	s.options.defaults()
	s.transcoders = make(chan struct{}, s.options.MaxTranscoders)
	s.metrics.Failures = map[Failure]int64{}
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-tick.C:
				s.mu.Lock()
				s.sweep(now)
				s.mu.Unlock()
			}
		}
	}()
	return s
}
func terminal(status string) bool {
	return status == "failed" || status == "stopped" || status == "ended" || status == "expired"
}
func (s *Service) finish(v *session, status string, code Failure) {
	if terminal(v.Status) {
		return
	}
	v.Status = status
	v.Error = code
	if code != "" {
		s.metrics.Failures[code]++
	}
	v.retain = time.Now().Add(5 * time.Minute)
	v.cancel()
	s.releaseLease("session", v.ID)
}
func (s *Service) sweep(now time.Time) {
	for id, v := range s.sessions {
		if !terminal(v.Status) && now.After(v.Expires) {
			s.finish(v, "expired", Expired)
		}
		if terminal(v.Status) && now.After(v.retain) {
			delete(s.sessions, id)
		}
	}
	for k, v := range s.limits {
		if now.After(v.until) {
			delete(s.limits, k)
		}
	}
}
func (s *Service) Create(ctx context.Context, owner, ip, id string) (out View, err error) {
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.metrics.Requests++
		if err != nil {
			s.metrics.RejectedCreates++
			s.metrics.Failures[Code(err)]++
		}
	}()
	sessionID := auth.Token()
	transferred := false
	if s.options.Control != nil {
		for _, spec := range []struct {
			key   string
			limit int
		}{{"owner:" + owner, s.options.OwnerPerMinute}, {"ip:" + ip, s.options.IPPerMinute}} {
			if err := s.options.Control.Rate(ctx, spec.key, spec.limit, time.Minute); err != nil {
				return View{}, Busy
			}
		}
		if err := s.options.Control.Lease(ctx, "session", sessionID, owner, s.options.MaxSessions, 30*time.Second); err != nil {
			return View{}, Busy
		}
		defer func() {
			if !transferred {
				s.releaseLease("session", sessionID)
				s.releaseLease("resolving", sessionID)
			}
		}()
		if err := s.options.Control.Lease(ctx, "resolving", sessionID, "", s.options.MaxResolving, 30*time.Second); err != nil {
			return View{}, Busy
		}
	}
	s.mu.Lock()
	s.sweep(time.Now())
	if s.ctx.Err() != nil || len(s.sessions)+s.resolving >= s.options.MaxSessions || s.pending[owner] || s.resolving >= s.options.MaxResolving {
		s.mu.Unlock()
		return View{}, Busy
	}
	for _, key := range []string{"o:" + owner, "i:" + ip} {
		v := s.limits[key]
		limit := s.options.IPPerMinute
		if key == "o:"+owner {
			limit = s.options.OwnerPerMinute
		}
		if v.n >= limit {
			s.mu.Unlock()
			return View{}, Busy
		}
	}
	if len(s.limits) > 4096 {
		s.mu.Unlock()
		return View{}, Busy
	}
	for _, v := range s.sessions {
		if v.owner == owner && !terminal(v.Status) {
			s.mu.Unlock()
			return View{}, Busy
		}
	}
	for _, key := range []string{"o:" + owner, "i:" + ip} {
		v := s.limits[key]
		if v.n == 0 {
			v.until = time.Now().Add(time.Minute)
		}
		v.n++
		s.limits[key] = v
	}
	s.pending[owner] = true
	s.resolving++
	s.workers.Add(1)
	s.mu.Unlock()
	track, err := s.store.Find(ctx, id)
	supported := (track.Provider == "bilibili" && track.PartID != nil && *track.PartID == "1") || (track.Provider == "netease" && track.PartID == nil)
	if err != nil || !supported {
		s.mu.Lock()
		delete(s.pending, owner)
		s.resolving--
		s.mu.Unlock()
		s.workers.Done()
		if err != nil {
			return View{}, err
		}
		return View{}, Unsupported
	}
	s.mu.Lock()
	delete(s.pending, owner)
	if s.ctx.Err() != nil {
		s.resolving--
		s.mu.Unlock()
		s.workers.Done()
		return View{}, Busy
	}
	c, cancel := context.WithCancel(s.ctx)
	v := &session{owner: owner, ctx: c, cancel: cancel, created: time.Now(), View: View{ID: sessionID, Status: "preparing", Capability: Capability{MediaKind: "unknown", TrackDuration: track.DurationSeconds}, Seek: "none", Attribution: track, Expires: time.Now().Add(25 * time.Second)}}
	s.sessions[v.ID] = v
	out = v.View
	s.mu.Unlock()
	transferred = true
	go func() {
		defer s.workers.Done()
		defer s.releaseLease("resolving", sessionID)
		rc, done := context.WithTimeout(c, 20*time.Second)
		a, err := s.resolver.Resolve(rc, track)
		done()
		a = normalizeAudio(a, track)
		if err == nil {
			err = s.options.Policy.Check(track.ID, a)
		}
		if err == nil && s.options.Control != nil {
			err = s.options.Control.Lease(c, "session", sessionID, owner, s.options.MaxSessions, 2*time.Minute)
		}
		if err == nil && (a.Transcode || s.options.ForceTranscode) {
			a.Transcode = true
			if s.options.FFmpeg == "" {
				err = TranscodeUnavailable
			}
		}
		s.mu.Lock()
		shouldRecord := !terminal(v.Status) && err != nil
		s.resolving--
		s.metrics.Preparations++
		s.metrics.PreparationMillis += time.Since(v.created).Milliseconds()
		if !terminal(v.Status) {
			if err != nil {
				s.finish(v, "failed", Code(err))
			} else {
				v.audio = a
				v.MIME = a.MIME
				if a.Transcode {
					v.MIME = "audio/mpeg"
				}
				v.Duration = a.Duration
				v.Capability = a.Capability
				v.StreamURL = "/api/v1/music/streams/" + v.ID
				v.Status = "ready"
				v.Expires = time.Now().Add(time.Minute)
			}
		}
		s.mu.Unlock()
		// Availability becomes available only after validated media bytes, not metadata resolution.
		if shouldRecord {
			checkCtx, cancelCheck := context.WithTimeout(s.ctx, 3*time.Second)
			defer cancelCheck()
			s.record(checkCtx, track.ID, a, Code(err))
		}
	}()
	return out, nil
}
func (s *Service) Get(id, owner string) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep(time.Now())
	v := s.sessions[id]
	if v == nil || v.owner != owner {
		return View{}, Expired
	}
	if v.Status == "expired" {
		return v.View, Expired
	}
	out := v.View
	out.Error = PublicCode(out.Error)
	return out, nil
}
func (s *Service) Stop(id, owner string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.sessions[id]
	if v == nil {
		return nil
	}
	if v.owner != owner {
		return Expired
	}
	s.finish(v, "stopped", "")
	return nil
}
func (s *Service) Close() {
	s.mu.Lock()
	s.cancel()
	for _, v := range s.sessions {
		s.finish(v, "stopped", "")
	}
	s.mu.Unlock()
	s.workers.Wait()
	s.cleanupWorkers.Wait()
}
func (s *Service) acquire(id, owner string) (*session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep(time.Now())
	if s.ctx.Err() != nil {
		return nil, Expired
	}
	v := s.sessions[id]
	if v == nil || v.owner != owner || terminal(v.Status) {
		return nil, Expired
	}
	if v.connected || v.Status != "ready" {
		return nil, Conflict
	}
	if s.streaming >= s.options.MaxStreams {
		return nil, Busy
	}
	if err := s.options.Policy.Check(v.Attribution.ID, v.audio); err != nil {
		s.finish(v, "failed", Code(err))
		return nil, Code(err)
	}
	if s.options.Control != nil {
		leaseCtx, done := context.WithTimeout(v.ctx, 3*time.Second)
		defer done()
		if err := s.options.Control.Lease(leaseCtx, "streaming", v.ID, "", s.options.MaxStreams, s.options.MaxStreamDuration+time.Minute); err != nil {
			return nil, Busy
		}
		if err := s.options.Control.Lease(leaseCtx, "session", v.ID, v.owner, s.options.MaxSessions, s.options.MaxStreamDuration+time.Minute); err != nil {
			s.releaseLease("streaming", v.ID)
			return nil, Busy
		}
	}
	v.connected = true
	v.Status = "streaming"
	v.Expires = time.Now().Add(s.options.MaxStreamDuration)
	s.streaming++
	s.workers.Add(1)
	return v, nil
}
func (s *Service) release(v *session, err error) {
	s.releaseLease("streaming", v.ID)
	s.mu.Lock()
	s.streaming--
	record := !terminal(v.Status) && err != nil
	if err != nil {
		s.finish(v, "failed", Code(err))
	} else {
		s.finish(v, "ended", "")
	}
	s.mu.Unlock()
	if record {
		ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
		s.record(ctx, v.Attribution.ID, v.audio, Code(err))
		cancel()
	}
	s.workers.Done()
}
