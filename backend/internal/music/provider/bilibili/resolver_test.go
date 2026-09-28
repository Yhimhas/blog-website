package bilibili

import (
	"blog-website/backend/internal/music/playback"
	"blog-website/backend/internal/music/provider"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

type sampleStore struct{}

func (sampleStore) Find(context.Context, string) (provider.Track, error) {
	p := "1"
	return provider.Track{ID: "bilibili:BV1a4MS67Eey:1", Provider: "bilibili", ExternalID: "BV1a4MS67Eey", PartID: &p}, nil
}
func (sampleStore) Checked(context.Context, string, playback.Failure) {}
func TestRealAudio(t *testing.T) {
	binary := os.Getenv("MUSIC_TEST_YTDLP")
	if binary == "" {
		t.Skip("opt-in real platform test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	tr, _ := (sampleStore{}).Find(ctx, "")
	a, e := (Resolver{binary}).Resolve(ctx, tr)
	if e != nil {
		t.Fatal(e)
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", a.URL, nil)
	for k, v := range a.Headers {
		req.Header.Set(k, v)
	}
	rr, e := playback.MediaClient().Do(req)
	if e != nil {
		if ue, ok := e.(*url.Error); ok {
			t.Fatal("safe transport:", ue.Err)
		}
		t.Fatal("transport failed")
	}
	rr.Body.Close()
	s := playback.New(ctx, Resolver{binary}, sampleStore{})
	defer s.Close()
	v, err := s.Create(ctx, "owner", "ip", "sample")
	if err != nil {
		t.Fatal(err)
	}
	for v.Status == "preparing" {
		time.Sleep(100 * time.Millisecond)
		v, err = s.Get(v.ID, "owner")
		if err != nil {
			t.Fatal(err)
		}
	}
	if v.Status != "ready" {
		t.Fatal(v.Status, v.Error)
	}
	t.Log("resolved", v.MIME, *v.Duration)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Stream(w, r, v.ID, "owner", func(status int, e playback.Failure) { http.Error(w, string(e), status) })
	}))
	defer server.Close()
	r, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b := make([]byte, 65536)
	n, err := io.ReadFull(r.Body, b)
	if r.StatusCode != 200 || err != nil || string(b[4:8]) != "ftyp" {
		t.Fatal("media", r.StatusCode, n, err, string(b[:min(n, 50)]))
	}
	t.Log("stream", r.StatusCode, r.Header.Get("Content-Type"), n, "bytes")
	s.Stop(v.ID, "owner")
}
func TestBoundedOutput(t *testing.T) {
	b := &bounded{max: 4}
	n, e := b.Write([]byte("123456"))
	if n != 6 || e != nil || !b.overflow || string(b.data) != "1234" {
		t.Fatal(b)
	}
	b.Write([]byte("more"))
	if len(b.data) != 4 {
		t.Fatal("unbounded")
	}
}
