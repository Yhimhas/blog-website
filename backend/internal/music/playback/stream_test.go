package playback

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
