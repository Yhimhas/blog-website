package platform

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"blog-website/backend/internal/blog"
)

func request(t *testing.T, method, path string) (*httptest.ResponseRecorder, map[string]json.RawMessage) {
	t.Helper()
	var logs bytes.Buffer
	h := NewHandler(blog.NewDemoService(), slog.New(slog.NewJSONHandler(&logs, nil)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	if got := w.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	id := w.Header().Get("X-Request-ID")
	if len(id) != 32 || !strings.Contains(logs.String(), id) {
		t.Fatalf("missing shared request ID: %q, logs %s", id, logs.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code >= 400 {
		var bodyID string
		if err := json.Unmarshal(body["requestId"], &bodyID); err != nil || bodyID != id {
			t.Fatalf("error requestId does not match header: %s", w.Body)
		}
	}
	return w, body
}

func TestHealth(t *testing.T) {
	w, body := request(t, "GET", "/api/v1/health")
	if w.Code != 200 || string(body["data"]) != `{"status":"ok"}` {
		t.Fatalf("health: %d %s", w.Code, w.Body)
	}
}

func TestList(t *testing.T) {
	for _, tc := range []struct {
		name, query, slugs string
		total, page, size  int
	}{
		{"default and stable ID order", "", "hello-go,building", 2, 1, 20},
		{"pagination", "page=2&pageSize=1", "building", 2, 2, 1},
		{"past last page", "page=3&pageSize=1", "", 2, 3, 1},
		{"large page no overflow", "page=" + strconv.Itoa(int(^uint(0)>>1)), "", 2, int(^uint(0) >> 1), 20},
		{"title case insensitive", "q=HELLO", "hello-go", 1, 1, 20},
		{"summary and trim", "q=" + url.QueryEscape("  网站开发  "), "building", 1, 1, 20},
		{"AND filters", "q=" + url.QueryEscape("组件") + "&category=development&tag=go", "building", 1, 1, 20},
		{"AND mismatch", "q=hello&tag=go", "", 0, 1, 20},
		{"unknown category", "category=unknown", "", 0, 1, 20},
		{"unknown tag", "tag=unknown", "", 0, 1, 20},
		{"literal wildcard", "q=%25", "", 0, 1, 20},
		{"literal underscore", "q=_", "", 0, 1, 20},
		{"empty filters", "q=&category=&tag=", "hello-go,building", 2, 1, 20},
		{"max size", "pageSize=50", "hello-go,building", 2, 1, 50},
		{"100 unicode characters", "q=" + url.QueryEscape(strings.Repeat("中", 100)), "", 0, 1, 20},
		{"draft search", "q=" + url.QueryEscape("未发布"), "", 0, 1, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, body := request(t, "GET", "/api/v1/posts?"+tc.query)
			if w.Code != 200 {
				t.Fatalf("status %d: %s", w.Code, w.Body)
			}
			var items []blog.Summary
			if err := json.Unmarshal(body["data"], &items); err != nil {
				t.Fatal(err)
			}
			if items == nil {
				t.Fatal("data must be an array")
			}
			slugs := make([]string, 0)
			for _, item := range items {
				slugs = append(slugs, item.Slug)
				if item.Tags == nil {
					t.Fatal("tags must be an array")
				}
			}
			if strings.Join(slugs, ",") != tc.slugs {
				t.Fatalf("slugs: %v", slugs)
			}
			var pagination struct{ Page, PageSize, Total int }
			if err := json.Unmarshal(body["pagination"], &pagination); err != nil {
				t.Fatal(err)
			}
			if pagination.Page != tc.page || pagination.PageSize != tc.size || pagination.Total != tc.total {
				t.Fatalf("pagination: %+v", pagination)
			}
			for _, forbidden := range []string{"contentMarkdown", "status", "draft-note"} {
				if strings.Contains(w.Body.String(), forbidden) {
					t.Fatalf("list leaked %s", forbidden)
				}
			}
		})
	}
}

func TestInvalidQueries(t *testing.T) {
	for _, query := range []string{"page=0", "page=-1", "page=abc", "page=1.5", "page=", "page=%2B1", "page=99999999999999999999999", "page=1&page=2", "pageSize=0", "pageSize=51", "pageSize=", "pageSize=x", "pageSize=1&pageSize=2", "q=a&q=b", "tag=a&tag=b", "category=a&category=b", "q=%FF", "q=%ZZ", "q=" + url.QueryEscape(strings.Repeat("中", 101))} {
		t.Run(query, func(t *testing.T) {
			w, body := request(t, "GET", "/api/v1/posts?"+query)
			if w.Code != 400 || !strings.Contains(string(body["error"]), `"code":"INVALID_QUERY"`) {
				t.Fatalf("status %d: %s", w.Code, w.Body)
			}
		})
	}
}

func TestDetail(t *testing.T) {
	w, body := request(t, "GET", "/api/v1/posts/hello-go")
	var detail blog.Detail
	if err := json.Unmarshal(body["data"], &detail); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || detail.Slug != "hello-go" || detail.ContentMarkdown == "" {
		t.Fatalf("detail: %s", w.Body)
	}
	if detail.Category != nil || detail.Tags == nil || !strings.Contains(w.Body.String(), `"category":null`) || !strings.Contains(w.Body.String(), `"tags":[]`) {
		t.Fatalf("nullable fields: %s", w.Body)
	}
	if !strings.Contains(w.Body.String(), `"publishedAt":"2026-09-21T02:00:00Z"`) || strings.Contains(w.Body.String(), `"status"`) {
		t.Fatalf("public DTO: %s", w.Body)
	}
}

func TestErrors(t *testing.T) {
	for _, tc := range []struct {
		method, path, code string
		status             int
	}{
		{"GET", "/api/v1/posts/draft-note", "POST_NOT_FOUND", 404},
		{"GET", "/api/v1/posts/missing", "POST_NOT_FOUND", 404},
		{"GET", "/api/v1/unknown", "NOT_FOUND", 404},
		{"GET", "/api/v1/ready", "NOT_FOUND", 404},
		{"GET", "/api/v1/posts/a/b", "NOT_FOUND", 404},
		{"POST", "/api/v1/posts", "METHOD_NOT_ALLOWED", 405},
		{"DELETE", "/api/v1/posts/building", "METHOD_NOT_ALLOWED", 405},
		{"POST", "/api/v1/health", "METHOD_NOT_ALLOWED", 405},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			w, body := request(t, tc.method, tc.path)
			if w.Code != tc.status || !strings.Contains(string(body["error"]), `"code":"`+tc.code+`"`) {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
			if tc.status == http.StatusMethodNotAllowed && w.Header().Get("Allow") != "GET" {
				t.Fatal("missing Allow header")
			}
		})
	}
}
