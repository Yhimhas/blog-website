package netease

import (
	"blog-website/backend/internal/music/playback"
	"blog-website/backend/internal/music/provider"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func neteaseTrack() provider.Track {
	return provider.Track{ID: "netease:123", Provider: "netease", ExternalID: "123"}
}
func TestAudioOfficialRedirect(t *testing.T) {
	calls := 0
	prefix := "ID3\x04\x00\x00\x00\x00\x00\x00" + strings.Repeat("a", 50)
	resolver := &AudioResolver{HTTP: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Cookie") != "" || r.URL.Scheme != "https" {
			t.Fatal("cookie or plaintext request")
		}
		if r.Header.Get("Referer") != audioReferer || r.Header.Get("User-Agent") != audioUserAgent {
			t.Fatal("missing public web playback headers")
		}
		if calls == 1 {
			if r.URL.Host != "music.163.com" || r.URL.Query().Get("id") != "123.mp3" {
				t.Fatal("uncontrolled source")
			}
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"http://m701.music.126.net/song.mp3"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"audio/mpeg"}}, Body: io.NopCloser(strings.NewReader(prefix)), Request: r}, nil
	})}}
	a, err := resolver.Resolve(context.Background(), neteaseTrack())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || a.URL != "https://m701.music.126.net/song.mp3" || a.MIME != "audio/mpeg" || a.InputFormat != "mp3" || a.Transcode {
		t.Fatal(a)
	}
}
func TestAudioDenials(t *testing.T) {
	for _, tc := range []struct {
		status   int
		body, ct string
		want     playback.Failure
	}{
		{404, "", "text/html", playback.SourceUnavailable}, {429, "", "text/html", playback.RateLimited},
		{200, `{"code":-460,"message":"denied"}`, "application/json", playback.Unavailable},
		{200, `{"code":429,"message":"limited"}`, "application/json", playback.RateLimited},
		{200, "<html>login required</html>", "text/html", playback.Unavailable},
		{200, "#EXTM3U\nhttp://127.0.0.1/secret", "audio/mpegurl", playback.Unavailable},
	} {
		r := &AudioResolver{HTTP: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Type": []string{tc.ct}}, Body: io.NopCloser(strings.NewReader(tc.body)), Request: req}, nil
		})}}
		if _, e := r.Resolve(context.Background(), neteaseTrack()); e != tc.want {
			t.Fatalf("%d got %v want %v", tc.status, e, tc.want)
		}
	}
}
func TestAudioRejectsForeignRedirectBeforeDial(t *testing.T) {
	for _, target := range []string{"https://evil.test/a.mp3", "https://music.126.net.evil.test/a.mp3", "https://127.0.0.1/a.mp3", "https://u:p@m701.music.126.net/a.mp3", "file:///secret"} {
		calls := 0
		r := &AudioResolver{HTTP: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{target}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
		})}}
		if _, e := r.Resolve(context.Background(), neteaseTrack()); e == nil || calls != 1 {
			t.Fatal("unsafe redirect followed", target, calls, e)
		}
	}
}
func TestAudioInvalidIdentity(t *testing.T) {
	r := NewAudioResolver()
	for _, v := range []provider.Track{{Provider: "netease", ID: "netease:123", ExternalID: "https://evil.test"}, {Provider: "netease", ID: "netease:456", ExternalID: "123"}} {
		if _, e := r.Resolve(context.Background(), v); e != playback.Unsupported {
			t.Fatal(e)
		}
	}
}
