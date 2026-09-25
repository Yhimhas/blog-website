package netease

import (
	"blog-website/backend/internal/music/provider"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestParseCompleteSnapshot(t *testing.T) {
	body := `{"code":200,"playlist":{"trackCount":3,"tracks":[{"id":123,"name":"音乐","ar":[{"name":"甲"},{"name":"乙"}],"dt":1999},{"id":123,"name":"音乐"},{"id":456,"name":"无作者"}]}}`
	tracks, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 2 || *tracks[0].DurationSeconds != 1 || *tracks[0].Author != "甲 / 乙" || tracks[1].Author != nil || tracks[1].DurationSeconds != nil {
		t.Fatalf("unexpected tracks %+v", tracks)
	}
	for _, body := range []string{`{}`, `{"code":200,"playlist":{"trackCount":1,"tracks":[]}}`, `{"code":200,"playlist":{"trackCount":0}}`, `{"code":200,"result":{"trackCount":1,"tracks":[{"id":123,"name":""}]}}`, `{"code":200,"result":{"trackCount":1,"tracks":[{"name":"missing"}]}}`} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("accepted %s", body)
		}
	}
	empty, err := Parse([]byte(`{"code":200,"result":{"trackCount":0,"tracks":[]}}`))
	if err != nil || len(empty) != 0 {
		t.Fatal("explicit empty snapshot rejected")
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestUpstreamFailures(t *testing.T) {
	for _, tc := range []struct {
		status            int
		body, ctype, code string
	}{{401, "", "application/json", "UPSTREAM_UNAUTHORIZED"}, {403, "", "application/json", "UPSTREAM_UNAUTHORIZED"}, {429, "", "application/json", "UPSTREAM_RATE_LIMITED"}, {503, "", "application/json", "UPSTREAM_HTTP"}, {200, "<html>", "text/html", "INVALID_PAYLOAD"}, {200, strings.Repeat("x", 2*1024*1024+1), "application/json", "RESPONSE_TOO_LARGE"}} {
		t.Run(tc.code, func(t *testing.T) {
			c := &Client{HTTP: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "music.163.com" || r.Header.Get("Cookie") != "" {
					t.Fatal("unsafe request")
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Type": []string{tc.ctype}}, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}}
			_, err := c.FetchPlaylist(context.Background(), provider.Ref{Provider: "netease", ExternalID: "123", SourceURL: "https://music.163.com/playlist?id=123"})
			if provider.Code(err) != tc.code {
				t.Fatalf("got %v", err)
			}
		})
	}
}
