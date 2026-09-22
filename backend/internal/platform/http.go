package platform

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"blog-website/backend/internal/blog"
)

type api struct {
	posts  *blog.Service
	logger *slog.Logger
}

func NewHandler(posts *blog.Service, logger *slog.Logger) http.Handler {
	return &api{posts: posts, logger: logger}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message, requestID string) {
	writeJSON(w, status, struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		RequestID string `json:"requestId"`
	}{Error: struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{code, message}, RequestID: requestID})
}

func (a *api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var bytes [16]byte
	_, _ = rand.Read(bytes[:])
	requestID := hex.EncodeToString(bytes[:])
	w.Header().Set("X-Request-ID", requestID)
	status := http.StatusOK
	defer func() {
		a.logger.Info("http request", "requestId", requestID, "method", r.Method, "path", r.URL.Path, "status", status)
	}()
	fail := func(s int, code, message string) { status = s; writeError(w, s, code, message, requestID) }
	path := r.URL.Path
	isDetail := strings.HasPrefix(path, "/api/v1/posts/") && strings.TrimPrefix(path, "/api/v1/posts/") != "" && !strings.Contains(strings.TrimPrefix(path, "/api/v1/posts/"), "/")
	if path != "/api/v1/health" && path != "/api/v1/posts" && !isDetail {
		fail(404, "NOT_FOUND", "接口不存在")
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		fail(405, "METHOD_NOT_ALLOWED", "请求方法不支持")
		return
	}
	if path == "/api/v1/health" {
		writeJSON(w, 200, map[string]any{"data": map[string]string{"status": "ok"}})
		return
	}
	if isDetail {
		post, ok := a.posts.Find(strings.TrimPrefix(path, "/api/v1/posts/"))
		if !ok {
			fail(404, "POST_NOT_FOUND", "文章不存在")
			return
		}
		writeJSON(w, 200, map[string]any{"data": post})
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		fail(400, "INVALID_QUERY", "查询参数格式不正确")
		return
	}
	page, ok := positiveInt(query, "page", 1, 0)
	if !ok {
		fail(400, "INVALID_QUERY", "page 必须为正整数")
		return
	}
	pageSize, ok := positiveInt(query, "pageSize", 20, 50)
	if !ok {
		fail(400, "INVALID_QUERY", "pageSize 必须为 1–50 的整数")
		return
	}
	for _, key := range []string{"q", "category", "tag"} {
		if len(query[key]) > 1 || !utf8.ValidString(query.Get(key)) {
			fail(400, "INVALID_QUERY", "筛选参数格式不正确")
			return
		}
	}
	q := strings.TrimSpace(query.Get("q"))
	if utf8.RuneCountInString(q) > 100 {
		fail(400, "INVALID_QUERY", "q 最多 100 个字符")
		return
	}
	items, total := a.posts.List(blog.Filter{Page: page, PageSize: pageSize, Q: q, Category: query.Get("category"), Tag: query.Get("tag")})
	writeJSON(w, 200, map[string]any{"data": items, "pagination": map[string]int{"page": page, "pageSize": pageSize, "total": total}})
}

func positiveInt(query url.Values, key string, fallback, maximum int) (int, bool) {
	values, exists := query[key]
	if !exists {
		return fallback, true
	}
	if len(values) != 1 || values[0] == "" {
		return 0, false
	}
	for _, c := range values[0] {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(values[0])
	return n, err == nil && n > 0 && (maximum == 0 || n <= maximum)
}
