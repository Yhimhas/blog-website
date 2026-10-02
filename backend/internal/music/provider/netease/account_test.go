package netease

import (
	"blog-website/backend/internal/music/playback"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type memoryCredential struct{ cookie string }

func (c *memoryCredential) Read() (Credential, error) { return Credential{Cookie: c.cookie}, nil }
func accountResponse(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

const accountOK = `{"code":200,"account":{"id":1},"profile":{"userId":1}}`
const unknownAudio = `{"code":200,"data":[{"id":123,"url":"https://cdn.music.126.net/audio.mp3?SIGNED_SENTINEL","code":200,"type":"mp3","time":200000,"freeTrialInfo":null}]}`

func TestAccountPreviewUnknownAndEntitlement(t *testing.T) {
	for _, test := range []struct {
		body, kind string
		err        playback.Failure
	}{
		{unknownAudio, "unknown", ""},
		{strings.Replace(unknownAudio, `"freeTrialInfo":null`, `"freeTrialInfo":{"start":30000,"end":60000}`, 1), "preview", ""},
		{`{"code":200,"data":[{"id":123,"url":null,"code":404,"fee":1}]}`, "", playback.EntitlementRequired},
		{strings.Replace(unknownAudio, "cdn.music.126.net", "evil.test", 1), "", playback.Unsupported},
		{strings.Replace(unknownAudio, `"id":123`, `"id":999`, 1), "", playback.Unsupported},
		{strings.Replace(unknownAudio, `"freeTrialInfo":null`, `"freeTrialInfo":{"start":60000,"end":30000}`, 1), "", playback.Unsupported},
	} {
		a, err := parseAccountAudio([]byte(test.body), neteaseTrack())
		if test.err != "" {
			if err != test.err {
				t.Fatalf("unexpected classification %v", err)
			}
			continue
		}
		if err != nil || a.Capability.MediaKind != test.kind || a.SourceMode != "account" || a.Audience != "private" || a.Headers["Cookie"] != "" || a.Headers["Authorization"] != "" {
			t.Fatal(a, err)
		}
		if test.kind == "preview" && (*a.Capability.PreviewStart != 30 || *a.Capability.PreviewEnd != 60 || *a.Capability.StreamDuration != 30) {
			t.Fatal("preview interval lost")
		}
	}
}

func TestAccountExpiryDisablesVersionAndRotationRestarts(t *testing.T) {
	credential := &memoryCredential{cookie: "MUSIC_U=COOKIE_SENTINEL_ONE"}
	requests := 0
	expired := true
	r := &AccountResolver{Credentials: credential, HTTP: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Host != "music.163.com" || req.URL.Scheme != "https" || req.Header.Get("Cookie") != credential.cookie || strings.Contains(req.URL.String(), "COOKIE_SENTINEL") {
			t.Fatal("account credential sent to wrong destination")
		}
		if req.URL.Path == "/api/w/nuser/account/get" {
			if expired {
				return accountResponse(200, `{"code":200,"account":null,"profile":null}`), nil
			}
			return accountResponse(200, accountOK), nil
		}
		return accountResponse(200, unknownAudio), nil
	})}}
	for i := 0; i < 2; i++ {
		if _, err := r.Resolve(context.Background(), neteaseTrack()); err != playback.SourceAuthExpired {
			t.Fatal(err)
		}
	}
	if requests != 1 {
		t.Fatal("expired cookie was retried", requests)
	}
	credential.cookie = "MUSIC_U=COOKIE_SENTINEL_TWO"
	expired = false
	a, err := r.Resolve(context.Background(), neteaseTrack())
	if err != nil || requests != 3 || !a.Valid() {
		t.Fatal(err, requests)
	}
	credential.cookie = "MUSIC_U=COOKIE_SENTINEL_THREE"
	if a.Valid() {
		t.Fatal("old account session survived credential rotation")
	}
}

func TestAccount403IsRestrictedAndCircuitIsBounded(t *testing.T) {
	requests := 0
	r := &AccountResolver{Credentials: &memoryCredential{cookie: "MUSIC_U=TEST_ONLY"}, HTTP: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { requests++; return accountResponse(403, `{}`), nil })}}
	for i := 0; i < 4; i++ {
		if _, err := r.Resolve(context.Background(), neteaseTrack()); err != playback.AccessRestricted {
			t.Fatal(err)
		}
	}
	if r.disabledVersion != "" || requests != 3 {
		t.Fatal("403 treated as invalid login or circuit did not open", requests)
	}
}

func TestAccountClientRejectsRedirects(t *testing.T) {
	requests := 0
	r := &AccountResolver{Credentials: &memoryCredential{cookie: "MUSIC_U=TEST_ONLY"}, HTTP: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Host != "music.163.com" {
			t.Fatal("account cookie redirect leak")
		}
		response := accountResponse(302, `{}`)
		response.Header.Set("Location", "https://cdn.music.126.net/leak")
		return response, nil
	})}}
	if _, err := r.Resolve(context.Background(), neteaseTrack()); err == nil || requests != 1 {
		t.Fatal(err, requests)
	}
}
