package storage_test

import (
	"blog-website/backend/internal/auth"
	"blog-website/backend/internal/platform"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAccountRoles(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	cfg := platform.Config{Origin: "http://localhost:5173", SessionTTL: time.Hour}
	handler := platform.NewDatabaseHandler(db, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	for _, role := range []string{"user", "admin"} {
		t.Run(role, func(t *testing.T) {
			if err := auth.CreateAccount(ctx, db, role, "integration-only-password", role); err != nil {
				t.Fatal(err)
			}
			request := func(method, path, body string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
				r := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
				r.Header.Set("Origin", cfg.Origin)
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("X-CSRF-Token", csrf)
				if cookie != nil {
					r.AddCookie(cookie)
				}
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				return w
			}
			login := request("POST", "/session", `{"username":"`+role+`","password":"integration-only-password"}`, nil, "")
			if login.Code != 200 {
				t.Fatalf("login: %d %s", login.Code, login.Body.String())
			}
			cookie := login.Result().Cookies()[0]
			session := request("GET", "/session", "", cookie, "")
			var payload struct {
				Data struct {
					User      auth.User
					CSRFToken string
				}
			}
			if err := json.Unmarshal(session.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if session.Code != 200 || payload.Data.User.Role != role || payload.Data.CSRFToken == "" {
				t.Fatal("session role or CSRF missing")
			}
			if session.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("session may be cached")
			}
			want := 403
			if role == "admin" {
				want = 200
			}
			if w := request("GET", "/admin/posts", "", cookie, ""); w.Code != want {
				t.Fatalf("admin list: %d want %d", w.Code, want)
			}
			if role == "user" {
				for _, path := range []string{"/admin/posts", "/admin/posts/missing/publish", "/admin/music/playlists/test/sync"} {
					if w := request("POST", path, `{}`, cookie, payload.Data.CSRFToken); w.Code != 403 || !strings.Contains(w.Body.String(), "ADMIN_REQUIRED") {
						t.Fatalf("user write accepted: %s %d", path, w.Code)
					}
				}
			} else {
				// Existing sessions must lose administrator access immediately after demotion.
				if err := db.Model(&auth.User{}).Where("username=?", role).Update("role", "user").Error; err != nil {
					t.Fatal(err)
				}
				if w := request("GET", "/admin/posts", "", cookie, ""); w.Code != 403 {
					t.Fatal("stale admin privileges")
				}
			}
			if w := request("POST", "/session/logout", `{}`, cookie, ""); w.Code != 403 {
				t.Fatal("logout accepted without CSRF")
			}
			if w := request("POST", "/session/logout", `{}`, cookie, payload.Data.CSRFToken); w.Code != 204 {
				t.Fatal("logout failed")
			}
			if w := request("GET", "/session", "", cookie, ""); w.Code != 401 {
				t.Fatal("logged-out session accepted")
			}
		})
	}
}
