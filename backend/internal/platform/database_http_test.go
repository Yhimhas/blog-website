package platform

import (
	"blog-website/backend/internal/auth"
	"blog-website/backend/internal/music"
	"blog-website/backend/internal/music/provider/netease"
	"context"
	"github.com/gin-gonic/gin"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDatabaseHTTPGuards(t *testing.T) {
	cfg := Config{Origin: "http://localhost:5173", SessionTTL: time.Hour}
	m := music.New(nil, netease.New(), context.Background())
	handler := NewDatabaseHandler(nil, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), m)
	for _, tc := range []struct {
		method, path, body, origin string
		want                       int
	}{{"GET", "/api/v1/health", "", "", 200}, {"GET", "/api/v1/admin/posts", "", "", 401}, {"POST", "/api/v1/admin/session", `{}`, "https://evil.test", 403}, {"POST", "/api/v1/admin/session", `{"username":"a","password":"b","admin":true}`, cfg.Origin, 400}, {"POST", "/api/v1/admin/session", `null`, cfg.Origin, 400}, {"GET", "/api/v1/posts?page=0", "", "", 400}, {"GET", "/api/v1/posts?page=1&page=2", "", "", 400}, {"GET", "/api/v1/music/recommendations?date=2100-01-01", "", "", 400}, {"POST", "/api/v1/health", "", "", 405}, {"GET", "/missing", "", "", 404}} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("%s %s = %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if w.Header().Get("X-Request-ID") == "" {
			t.Error("missing request id")
		}
		if strings.Contains(tc.path, "/admin") && w.Header().Get("Cache-Control") != "no-store" {
			t.Error("private response cached")
		}
		if w.Code == http.StatusMethodNotAllowed && w.Header().Get("Allow") != "GET" {
			t.Error("Allow header")
		}
	}
}

func TestAuthenticatedWriteRequiresOriginAndCSRF(t *testing.T) {
	token := auth.Token()
	a := databaseAPI{config: Config{Origin: "http://localhost:5173"}}
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("sessionToken", token); c.Set("requestId", "test"); c.Next() })
	router.POST("/write", a.requireWrite, func(c *gin.Context) { c.Status(204) })
	for _, tc := range []struct {
		origin, csrf string
		want         int
	}{{a.config.Origin, "", 403}, {"https://evil.test", auth.CSRF(token), 403}, {a.config.Origin, auth.CSRF(auth.Token()), 403}, {a.config.Origin, auth.CSRF(token), 204}} {
		r := httptest.NewRequest("POST", "/write", nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("X-CSRF-Token", tc.csrf)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("write guard returned %d want %d", w.Code, tc.want)
		}
	}
}
