package storage_test

import (
	"blog-website/backend/internal/auth"
	"blog-website/backend/internal/platform"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPublicSEORevisionNeverExposesPublicationReceipts(t *testing.T) {
	db := testDB(t)
	handler := platform.NewDatabaseHandler(db, platform.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/seo/revision", nil))
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("revision endpoint", w.Code, w.Body.String())
	}
	var payload struct{ Data map[string]any }
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data) != 2 || payload.Data["revision"] != "1" || payload.Data["sourceId"] == "" {
		t.Fatal("revision payload exposed private state or lost string precision", payload.Data)
	}
	ctx := context.Background()
	if err := auth.CreateAccount(ctx, db, "reader", "integration-only-password", "user"); err != nil {
		t.Fatal(err)
	}
	_, token, err := (auth.Service{DB: db, TTL: time.Hour}).Login(ctx, "reader", "integration-only-password")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/api/v1/admin/seo", nil)
	request.Header.Set("Cookie", "blog_session="+token)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, request)
	if w.Code != 403 {
		t.Fatal("ordinary reader saw publication receipts", w.Code)
	}
	if err := db.Model(&auth.User{}).Where("username=?", "reader").Update("role", "admin").Error; err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, request)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("administrator could not inspect pending publication", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data["appliedRevision"] != "0" || payload.Data["lastError"] != "" {
		t.Fatal("publication receipt payload", payload.Data)
	}
}
