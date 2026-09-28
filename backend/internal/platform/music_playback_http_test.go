package platform

import (
	"blog-website/backend/internal/music/playback"
	"blog-website/backend/internal/music/provider"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type playbackTestStore struct{}

func (playbackTestStore) Find(context.Context, string) (provider.Track, error) {
	p := "1"
	return provider.Track{ID: "bilibili:BV1a4MS67Eey:1", Provider: "bilibili", PartID: &p}, nil
}
func (playbackTestStore) Checked(context.Context, string, playback.Failure) {}

type playbackTestResolver struct{}

func (playbackTestResolver) Resolve(ctx context.Context, _ provider.Track) (playback.Audio, error) {
	<-ctx.Done()
	return playback.Audio{}, ctx.Err()
}
func TestPlaybackHTTPGuards(t *testing.T) {
	s := playback.New(context.Background(), playbackTestResolver{}, playbackTestStore{})
	defer s.Close()
	r := gin.New()
	bindPlayback(r.Group("/api/v1"), s, Config{Origin: "https://site.test", SecureCookie: true})
	call := func(body, origin string, cookie *http.Cookie, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", origin)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		r.ServeHTTP(w, req)
		return w
	}
	path := "/api/v1/music/playback-sessions"
	body := `{"trackId":"bilibili:BV1a4MS67Eey:1"}`
	if w := call(body, "https://evil.test", nil, path); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := call(`{"trackId":"t","url":"https://evil.test"}`, "https://site.test", nil, path); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := call(`{"trackId":"`+strings.Repeat("a", 2000)+`"}`, "https://site.test", nil, path); w.Code != 400 {
		t.Fatal(w.Code)
	}
	w := call(body, "https://site.test", nil, path)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe cookie")
	}
	var result struct{ Data playback.View }
	json.Unmarshal(w.Body.Bytes(), &result)
	if w := call("", "https://site.test", cookies[0], path+"/"+result.Data.ID+"/stop"); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w := call("", "https://site.test", cookies[0], path+"/"+result.Data.ID+"/stop"); w.Code != 204 {
		t.Fatal("not idempotent")
	}
}
