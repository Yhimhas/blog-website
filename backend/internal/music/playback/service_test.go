package playback

import (
	"blog-website/backend/internal/music/provider"
	"context"
	"net"
	"testing"
	"time"
)

type fakeStore struct{}

func (fakeStore) Find(context.Context, string) (provider.Track, error) {
	p := "1"
	return provider.Track{ID: "bilibili:BV1a4MS67Eey:1", Provider: "bilibili", PartID: &p}, nil
}
func (fakeStore) Checked(context.Context, string, Failure) {}

type fakeResolver struct{ wait bool }

func (r fakeResolver) Resolve(ctx context.Context, _ provider.Track) (Audio, error) {
	if r.wait {
		<-ctx.Done()
		return Audio{}, ctx.Err()
	}
	return Audio{URL: "https://example.com/audio", MIME: "audio/mp4"}, nil
}
func ready(t *testing.T, s *Service, id, owner string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		v, e := s.Get(id, owner)
		if e == nil && v.Status == "ready" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("not ready")
}
func TestLifecycle(t *testing.T) {
	s := New(context.Background(), fakeResolver{}, fakeStore{})
	defer s.Close()
	v, e := s.Create(context.Background(), "owner", "ip", "track")
	if e != nil {
		t.Fatal(e)
	}
	ready(t, s, v.ID, "owner")
	if _, e = s.Get(v.ID, "other"); e != Expired {
		t.Fatal("cookie bypass")
	}
	if _, e = s.Create(context.Background(), "owner", "ip", "track"); e != Busy {
		t.Fatal("duplicate active accepted")
	}
	a, e := s.acquire(v.ID, "owner")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.acquire(v.ID, "owner"); e != Conflict {
		t.Fatal("duplicate stream")
	}
	s.Stop(v.ID, "owner")
	s.Stop(v.ID, "owner")
	select {
	case <-a.ctx.Done():
	default:
		t.Fatal("not cancelled")
	}
	s.release(a, context.Canceled)
	out, _ := s.Get(v.ID, "owner")
	if out.Status != "stopped" {
		t.Fatal(out.Status)
	}
}
func TestResolverLimitAndClose(t *testing.T) {
	s := New(context.Background(), fakeResolver{true}, fakeStore{})
	for _, o := range []string{"a", "b"} {
		if _, e := s.Create(context.Background(), o, o, "t"); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := s.Create(context.Background(), "c", "c", "t"); e != Busy {
		t.Fatal("limit ignored")
	}
	done := make(chan bool)
	go func() { s.Close(); done <- true }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("workers not cancelled")
	}
}
func TestExpiryAndCapacity(t *testing.T) {
	s := New(context.Background(), fakeResolver{}, fakeStore{})
	defer s.Close()
	v, _ := s.Create(context.Background(), "a", "a", "t")
	ready(t, s, v.ID, "a")
	s.mu.Lock()
	s.sessions[v.ID].Expires = time.Now().Add(-time.Second)
	s.mu.Unlock()
	if _, e := s.Get(v.ID, "a"); e != Expired {
		t.Fatal("not expired")
	}
	s.mu.Lock()
	s.sweep(time.Now().Add(6 * time.Minute))
	n := len(s.sessions)
	s.mu.Unlock()
	if n != 0 {
		t.Fatal("terminal entry leaked")
	}
}
func TestAddressPolicy(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "100.96.172.0", "169.254.169.254", "::1", "::ffff:127.0.0.1", "fc00::1", "192.0.2.1"} {
		if PublicIP(net.ParseIP(ip)) {
			t.Fatal(ip)
		}
	}
	if !PublicIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public rejected")
	}
	for _, u := range []string{"http://example.com/a", "https://u:p@example.com/a", "https://example.com:8080/a", "file:///etc/passwd"} {
		if MediaURL(u) {
			t.Fatal(u)
		}
	}
	if _, err := MediaClient().Get("https://127.0.0.1/test"); err == nil {
		t.Fatal("private connection allowed")
	}
}

func TestOfficialCDNPorts(t *testing.T) {
	for _, u := range []string{"https://cdn.bilivideo.cn:8082/a", "https://cdn.bilivideo.com:4483/a"} {
		if !MediaURL(u) {
			t.Fatal(u)
		}
	}
	for _, u := range []string{"https://bilivideo.cn.evil.test:8082/a", "https://evil.test:4483/a"} {
		if MediaURL(u) {
			t.Fatal(u)
		}
	}
}

type neteaseStore struct{ fakeStore }

func (neteaseStore) Find(context.Context, string) (provider.Track, error) {
	return provider.Track{ID: "netease:123", ExternalID: "123", Provider: "netease"}, nil
}
func TestNetEaseRoutingAndForceTranscode(t *testing.T) {
	s := New(context.Background(), Resolvers{"netease": fakeResolver{}}, neteaseStore{}, Options{FFmpeg: "configured-ffmpeg", ForceTranscode: true})
	defer s.Close()
	v, e := s.Create(context.Background(), "n", "n", "netease:123")
	if e != nil {
		t.Fatal(e)
	}
	ready(t, s, v.ID, "n")
	v, e = s.Get(v.ID, "n")
	if e != nil || v.MIME != "audio/mpeg" || v.Attribution.Provider != "netease" {
		t.Fatal(v, e)
	}
	if _, e := (Resolvers{}).Resolve(context.Background(), provider.Track{Provider: "other"}); e != Unsupported {
		t.Fatal(e)
	}
}
