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

func TestParseFailureClassification(t *testing.T) {
	for _, tc := range []struct{ name, body, code string }{
		{"observed refusal with empty msg", `{"msg":"","code":20001}`, "UPSTREAM_ACCESS_RESTRICTED"},
		{"refusal before success fields", `{"code":20001,"playlist":"unavailable","msg":"private upstream text"}`, "UPSTREAM_ACCESS_RESTRICTED"},
		{"unknown business failure", `{"code":50001,"result":false}`, "UPSTREAM_REJECTED"},
		{"unauthorized", `{"code":401}`, "UPSTREAM_UNAUTHORIZED"},
		{"forbidden", `{"code":403}`, "UPSTREAM_UNAUTHORIZED"},
		{"rate limited", `{"code":429}`, "UPSTREAM_RATE_LIMITED"},
		{"invalid JSON", `{"code":20001`, "INVALID_PAYLOAD"},
		{"missing code", `{}`, "INVALID_PAYLOAD"},
		{"null code", `{"code":null}`, "INVALID_PAYLOAD"},
		{"string code", `{"code":"20001"}`, "INVALID_PAYLOAD"},
		{"fractional code", `{"code":200.5}`, "INVALID_PAYLOAD"},
		{"null envelope", `null`, "INVALID_PAYLOAD"},
		{"missing success data", `{"code":200}`, "INVALID_PAYLOAD"},
		{"wrong success shape", `{"code":200,"playlist":"bad"}`, "INVALID_PAYLOAD"},
		{"wrong count type", `{"code":200,"playlist":{"trackCount":"1","tracks":[]}}`, "INVALID_PAYLOAD"},
		{"missing tracks", `{"code":200,"playlist":{"trackCount":0}}`, "INVALID_PAYLOAD"},
		{"negative count", `{"code":200,"playlist":{"trackCount":-1,"tracks":[]}}`, "INVALID_PAYLOAD"},
		{"over limit", `{"code":200,"playlist":{"trackCount":2001,"tracks":[]}}`, "INVALID_PAYLOAD"},
		{"partial playlist", `{"code":200,"playlist":{"trackCount":265,"tracks":[{"id":123,"name":"preview"}]}}`, "INCOMPLETE_PLAYLIST"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tracks, err := Parse([]byte(tc.body))
			if err == nil || provider.Code(err) != tc.code || tracks != nil {
				t.Fatalf("got tracks=%v error=%v; want %s with no candidate snapshot", tracks, err, tc.code)
			}
		})
	}
}

func TestUpstreamFailures(t *testing.T) {
	for _, tc := range []struct {
		status            int
		body, ctype, code string
	}{{401, "", "application/json", "UPSTREAM_UNAUTHORIZED"}, {403, "", "application/json", "UPSTREAM_UNAUTHORIZED"}, {429, "", "application/json", "UPSTREAM_RATE_LIMITED"}, {503, "", "application/json", "UPSTREAM_HTTP"}, {200, `{"code":20001,"msg":""}`, "application/json", "UPSTREAM_ACCESS_RESTRICTED"}, {200, `{"code":50001}`, "application/json", "UPSTREAM_REJECTED"}, {200, `{"code":200}`, "application/json", "INVALID_PAYLOAD"}, {200, "<html>", "text/html", "INVALID_PAYLOAD"}, {200, strings.Repeat("x", 2*1024*1024+1), "application/json", "RESPONSE_TOO_LARGE"}} {
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
