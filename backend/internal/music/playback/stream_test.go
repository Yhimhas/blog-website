package playback

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type waitingBody struct {
	ctx  context.Context
	sent bool
}

func (b *waitingBody) Read(p []byte) (int, error) {
	if !b.sent {
		b.sent = true
		return copy(p, []byte("\x00\x00\x00\x18ftypisom")), nil
	}
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}
func (b *waitingBody) Close() error { return nil }

func TestMediaCookieIsolationAndByteBudget(t *testing.T) {
	s := New(context.Background(), audioResolver{audio: Audio{URL: "https://cdn.music.126.net/test", MIME: "audio/mp4", Headers: map[string]string{"Cookie": "PRIVATE_COOKIE", "Authorization": "PRIVATE_AUTH"}}}, fakeStore{}, Options{MaxStreamBytes: 20})
	defer s.Close()
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse("https://cdn.music.126.net/")
	jar.SetCookies(u, []*http.Cookie{{Name: "account_cookie", Value: "PRIVATE_JAR"}})
	s.client = &http.Client{Jar: jar, Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Fatal("CDN received account or visitor credentials")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"audio/mp4"}}, Body: io.NopCloser(strings.NewReader("\x00\x00\x00\x18ftypisom" + strings.Repeat("a", 100)))}, nil
	})}
	v, _ := s.Create(context.Background(), "owner", "ip", "track")
	ready(t, s, v.ID, "owner")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Stream(w, r, v.ID, "owner", func(status int, code Failure) { http.Error(w, string(code), status) })
	}))
	defer server.Close()
	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 503 {
		t.Fatal("over-budget media started", response.StatusCode)
	}
	view, _ := s.Get(v.ID, "owner")
	if view.Error != BudgetExceeded {
		t.Fatal("byte ceiling classification lost", view)
	}
}

func TestSignedURLRefreshIsBounded(t *testing.T) {
	resolverCalls := 0
	requests := 0
	s := New(context.Background(), audioResolver{audio: Audio{URL: "https://cdn.music.126.net/test", MIME: "audio/mp4"}, calls: &resolverCalls}, fakeStore{})
	defer s.Close()
	s.client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: 403, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("denied"))}, nil
	})}
	v, _ := s.Create(context.Background(), "owner", "ip", "track")
	ready(t, s, v.ID, "owner")
	w := httptest.NewRecorder()
	s.Stream(w, httptest.NewRequest("GET", "/", nil), v.ID, "owner", func(status int, code Failure) { http.Error(w, string(code), status) })
	if requests != 2 || resolverCalls != 2 || w.Code < 400 {
		t.Fatal("URL refresh was unbounded or bypassed policy", requests, resolverCalls, w.Code)
	}
}
func TestStreamFailureBeforeHeaders(t *testing.T) {
	for _, status := range []int{403, 429, 200} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			s := New(context.Background(), fakeResolver{}, fakeStore{})
			defer s.Close()
			s.client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader("<html>forbidden</html>"))}, nil
			})}
			v, _ := s.Create(context.Background(), "owner", "ip", "t")
			ready(t, s, v.ID, "owner")
			w := httptest.NewRecorder()
			s.Stream(w, httptest.NewRequest("GET", "/", nil), v.ID, "owner", func(status int, code Failure) { http.Error(w, string(code), status) })
			if w.Code < 400 {
				t.Fatal("accepted HTML or error")
			}
			out, _ := s.Get(v.ID, "owner")
			if out.Status != "failed" {
				t.Fatal(out)
			}
			if status == 429 && out.Error != RateLimited {
				t.Fatal(out.Error)
			}
		})
	}
}
func TestStopCancelsUpstream(t *testing.T) {
	s := New(context.Background(), fakeResolver{}, fakeStore{})
	defer s.Close()
	var requests atomic.Int32
	s.client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/octet-stream"}}, Body: &waitingBody{ctx: r.Context()}}, nil
	})}
	v, _ := s.Create(context.Background(), "owner", "ip", "t")
	ready(t, s, v.ID, "owner")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Stream(w, r, v.ID, "owner", func(status int, c Failure) { http.Error(w, string(c), status) })
	}))
	defer server.Close()
	resp, e := http.Get(server.URL)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	b := make([]byte, 12)
	if _, e := io.ReadFull(resp.Body, b); e != nil {
		t.Fatal(e)
	}
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "audio/mp4" {
		t.Fatal(resp.Status)
	}
	duplicate, e := http.Get(server.URL)
	if e != nil {
		t.Fatal(e)
	}
	duplicate.Body.Close()
	if duplicate.StatusCode != 409 || requests.Load() != 1 {
		t.Fatal("duplicate reached upstream")
	}
	s.Stop(v.ID, "owner")
	done := make(chan bool)
	go func() { io.Copy(io.Discard, resp.Body); done <- true }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stop did not interrupt upstream")
	}
	out, _ := s.Get(v.ID, "owner")
	if out.Status != "stopped" {
		t.Fatal(out.Status)
	}
}
func TestFourStreamLimit(t *testing.T) {
	s := New(context.Background(), fakeResolver{}, fakeStore{})
	defer s.Close()
	var active []*session
	defer func() {
		for _, v := range active {
			s.release(v, nil)
		}
	}()
	for _, owner := range []string{"1", "2", "3", "4", "5"} {
		v, e := s.Create(context.Background(), owner, owner, "t")
		if e != nil {
			t.Fatal(e)
		}
		ready(t, s, v.ID, owner)
		a, e := s.acquire(v.ID, owner)
		if owner == "5" {
			if e != Busy {
				t.Fatal("fifth admitted")
			}
		} else {
			if e != nil {
				t.Fatal(e)
			}
			active = append(active, a)
		}
	}
}

type brokenBody struct{ sent bool }

func (b *brokenBody) Read(p []byte) (int, error) {
	if b.sent {
		return 0, io.ErrUnexpectedEOF
	}
	b.sent = true
	return copy(p, []byte("\x00\x00\x00\x18ftypisom")), nil
}
func (b *brokenBody) Close() error { return nil }
func TestMidStreamFailureAbortsConnection(t *testing.T) {
	s := New(context.Background(), fakeResolver{}, fakeStore{})
	defer s.Close()
	s.client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"audio/mp4"}}, Body: &brokenBody{}}, nil
	})}
	v, err := s.Create(context.Background(), "a", "a", "t")
	if err != nil {
		t.Fatal(err)
	}
	ready(t, s, v.ID, "a")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Stream(w, r, v.ID, "a", func(code int, f Failure) { http.Error(w, string(f), code) })
	}))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err == nil || len(b) != 12 || string(b[4:8]) != "ftyp" {
		t.Fatal("late error became a clean EOF or appended error data", err, len(b))
	}
	final, _ := s.Get(v.ID, "a")
	if final.Status != "failed" {
		t.Fatal(final.Status)
	}
}
