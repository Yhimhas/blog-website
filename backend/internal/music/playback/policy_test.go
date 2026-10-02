package playback

import (
	"blog-website/backend/internal/music/provider"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type audioResolver struct {
	audio Audio
	err   error
	calls *int
}

func (r audioResolver) Resolve(context.Context, provider.Track) (Audio, error) {
	if r.calls != nil {
		*r.calls++
	}
	return r.audio, r.err
}
func intPointer(n int) *int { return &n }

func TestPolicyCompletenessAndAudience(t *testing.T) {
	target := provider.Track{ID: "netease:123", DurationSeconds: intPointer(200)}
	a := normalizeAudio(Audio{Duration: intPointer(30)}, target)
	if a.Capability.MediaKind != "unknown" || a.Capability.TrackDuration == nil || *a.Capability.TrackDuration != 200 {
		t.Fatal(a)
	}
	if err := (Policy{}).Check(target.ID, a); err != nil {
		t.Fatal(err)
	}
	a.Capability.MediaKind = "full"
	if err := (Policy{}).Check(target.ID, a); err != Unsupported {
		t.Fatal("unproven full accepted", err)
	}
	a.CompletenessEvidence = "verified upstream full flag"
	if err := (Policy{RequireFull: true}).Check(target.ID, a); err != nil {
		t.Fatal(err)
	}
	a.Capability.MediaKind = "preview"
	a.Capability.PreviewStart = intPointer(30)
	a.Capability.PreviewEnd = intPointer(60)
	if err := (Policy{}).Check(target.ID, a); err != nil {
		t.Fatal(err)
	}
	if err := (Policy{RequireFull: true}).Check(target.ID, a); err != PublicNotAllowed {
		t.Fatal(err)
	}
	a.SourceMode = "account"
	a.Audience = "private"
	if err := (Policy{}).Check(target.ID, a); err != PublicNotAllowed {
		t.Fatal(err)
	}
	a.Audience = "public"
	if err := (Policy{}).Check(target.ID, a); err != PublicNotAllowed {
		t.Fatal("cookie entitlement became a public grant", err)
	}
	p := Policy{Mode: "licensed", Grants: []Grant{{TrackID: target.ID, SourceMode: "account", Audience: "public", Evidence: "operator authorization reference", Expires: time.Now().Add(time.Minute)}}}
	if err := p.Check(target.ID, a); err != nil {
		t.Fatal(err)
	}
	p.Grants[0].Expires = time.Now().Add(-time.Second)
	if err := p.Check(target.ID, a); err != PublicNotAllowed {
		t.Fatal("expired grant accepted", err)
	}
}

func TestFallbackDoesNotUseUnlicensedAccountAndScopesFailures(t *testing.T) {
	count := 0
	r := FallbackResolver{Public: audioResolver{err: Unavailable}, Account: audioResolver{err: SourceAuthExpired, calls: &count}}
	target := provider.Track{ID: "netease:123"}
	if _, err := r.Resolve(context.Background(), target); err != Unavailable || count != 0 {
		t.Fatal(err, count)
	}
	r.Policy = Policy{Mode: "licensed", Grants: []Grant{{TrackID: target.ID, SourceMode: "account", Audience: "public", Evidence: "test grant", Expires: time.Now().Add(time.Minute)}}}
	a, err := r.Resolve(context.Background(), target)
	if err != SourceAuthExpired || count != 1 || a.SourceMode != "account" || a.Audience != "blocked" {
		t.Fatal(a, err, count)
	}
}

func TestAccountDeniedBeforeReadyAndCredentialsStayPrivate(t *testing.T) {
	a := Audio{URL: "https://cdn.music.126.net/signed?secret=URL_SENTINEL", MIME: "audio/mpeg", SourceMode: "account", Audience: "private", CredentialVersion: "PRIVATE_VERSION", Headers: map[string]string{"Cookie": "COOKIE_SENTINEL", "Authorization": "AUTH_SENTINEL"}}
	s := New(context.Background(), audioResolver{audio: a}, neteaseStore{})
	defer s.Close()
	v, err := s.Create(context.Background(), "o", "ip", "netease:123")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		out, _ := s.Get(v.ID, "o")
		if out.Status == "failed" {
			if out.Error != PublicNotAllowed || out.StreamURL != "" {
				t.Fatal(out)
			}
			raw, _ := json.Marshal(out)
			for _, secret := range []string{"URL_SENTINEL", "COOKIE_SENTINEL", "AUTH_SENTINEL", "PRIVATE_VERSION"} {
				if strings.Contains(string(raw), secret) {
					t.Fatal("DTO credential leak")
				}
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("unlicensed account session did not fail")
}

func TestAuthExpiredPublicMappingAndBudget(t *testing.T) {
	s := New(context.Background(), audioResolver{err: SourceAuthExpired}, neteaseStore{}, Options{MaxStreamBytes: 20, HourlyBytes: 30})
	defer s.Close()
	v, _ := s.Create(context.Background(), "o", "ip", "netease:123")
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		out, _ := s.Get(v.ID, "o")
		if out.Status == "failed" {
			if out.Error != Unavailable {
				t.Fatal("internal credential status exposed", out)
			}
			break
		}
		time.Sleep(time.Millisecond)
	}
	a := normalizeAudio(Audio{}, provider.Track{})
	active := &session{audio: a}
	if err := s.charge(context.Background(), active, 15); err != nil {
		t.Fatal(err)
	}
	if err := s.charge(context.Background(), active, 10); err != BudgetExceeded {
		t.Fatal(err)
	}
	other := &session{audio: a}
	if err := s.charge(context.Background(), other, 20); err != BudgetExceeded {
		t.Fatal(err)
	}
}

func TestRefreshHonorsResolverCapacity(t *testing.T) {
	count := 0
	s := New(context.Background(), audioResolver{calls: &count}, fakeStore{}, Options{MaxResolving: 1})
	defer s.Close()
	s.mu.Lock()
	s.resolving = 1
	s.mu.Unlock()
	if _, err := s.resolveAgain(context.Background(), provider.Track{}); err != Busy || count != 0 {
		t.Fatal("refresh bypassed resolver capacity", err, count)
	}
	s.mu.Lock()
	s.resolving = 0
	s.mu.Unlock()
}
