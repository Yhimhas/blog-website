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
	NotFound    Failure = "TRACK_NOT_FOUND"
	Unsupported Failure = "PLAYBACK_UNSUPPORTED"
	Unavailable Failure = "UPSTREAM_UNAVAILABLE"
	RateLimited Failure = "UPSTREAM_RATE_LIMITED"
	Timeout     Failure = "PLAYBACK_TIMEOUT"
	Busy        Failure = "PLAYBACK_BUSY"
	Conflict    Failure = "STREAM_CONFLICT"
	Expired     Failure = "SESSION_EXPIRED"
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
	Headers   map[string]string
	Duration  *int
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
}
type session struct {
	View
	owner     string
	audio     Audio
	ctx       context.Context
	cancel    context.CancelFunc
	connected bool
	retain    time.Time
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
	resolving, streaming int
	pending              map[string]bool
	client               *http.Client
}

func New(ctx context.Context, r Resolver, db Store) *Service {
	ctx, cancel := context.WithCancel(ctx)
	s := &Service{ctx: ctx, cancel: cancel, resolver: r, store: db, sessions: map[string]*session{}, limits: map[string]counter{}, pending: map[string]bool{}, client: MediaClient()}
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
	v.retain = time.Now().Add(5 * time.Minute)
	v.cancel()
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
func (s *Service) Create(ctx context.Context, owner, ip, id string) (View, error) {
	s.mu.Lock()
	s.sweep(time.Now())
	if s.ctx.Err() != nil || len(s.sessions)+s.resolving >= 128 || s.pending[owner] || s.resolving >= 2 {
		s.mu.Unlock()
		return View{}, Busy
	}
	for _, key := range []string{"o:" + owner, "i:" + ip} {
		v := s.limits[key]
		if v.n >= 10 {
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
	if err != nil || track.Provider != "bilibili" || track.PartID == nil || *track.PartID != "1" {
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
	v := &session{owner: owner, ctx: c, cancel: cancel, View: View{ID: auth.Token(), Status: "preparing", Seek: "none", Attribution: track, Expires: time.Now().Add(25 * time.Second)}}
	s.sessions[v.ID] = v
	out := v.View
	s.mu.Unlock()
	go func() {
		defer s.workers.Done()
		rc, done := context.WithTimeout(c, 20*time.Second)
		a, err := s.resolver.Resolve(rc, track)
		done()
		s.mu.Lock()
		shouldRecord := !terminal(v.Status) && err != nil
		s.resolving--
		if !terminal(v.Status) {
			if err != nil {
				s.finish(v, "failed", Code(err))
			} else {
				v.audio = a
				v.MIME = a.MIME
				v.Duration = a.Duration
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
			s.store.Checked(checkCtx, track.ID, Code(err))
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
	return v.View, nil
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
	if s.streaming >= 4 {
		return nil, Busy
	}
	v.connected = true
	v.Status = "streaming"
	v.Expires = time.Now().Add(6 * time.Hour)
	s.streaming++
	s.workers.Add(1)
	return v, nil
}
func (s *Service) release(v *session, err error) {
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
		s.store.Checked(ctx, v.Attribution.ID, Code(err))
		cancel()
	}
	s.workers.Done()
}
