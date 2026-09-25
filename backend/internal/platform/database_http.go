package platform

import (
	"blog-website/backend/internal/auth"
	"blog-website/backend/internal/blog"
	"blog-website/backend/internal/music"
	"blog-website/backend/internal/music/provider"
	"blog-website/backend/internal/recommendation"
	"blog-website/backend/internal/storage"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type databaseAPI struct {
	db                    *gorm.DB
	config                Config
	posts                 blog.Repository
	auth                  auth.Service
	music                 *music.Service
	recommendation        recommendation.Service
	loginLimit, syncLimit auth.Limiter
}

func NewDatabaseHandler(db *gorm.DB, cfg Config, logger *slog.Logger, m *music.Service) http.Handler {
	a := &databaseAPI{db: db, config: cfg, posts: blog.Repository{DB: db}, auth: auth.Service{DB: db, TTL: cfg.SessionTTL}, music: m, recommendation: recommendation.Service{DB: db}, loginLimit: auth.Limiter{Limit: 10, Window: time.Minute}, syncLimit: auth.Limiter{Limit: 4, Window: time.Minute}}
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	router.HandleMethodNotAllowed = true
	router.RedirectTrailingSlash = false
	router.RedirectFixedPath = false
	router.Use(func(c *gin.Context) {
		id := auth.Token()[:32]
		c.Set("requestId", id)
		c.Header("X-Request-ID", id)
		c.Header("X-Content-Type-Options", "nosniff")
		if strings.HasPrefix(c.Request.URL.Path, "/api/v1/admin") {
			c.Header("Cache-Control", "no-store")
			c.Header("X-Robots-Tag", "noindex")
		}
		defer func() {
			if recover() != nil {
				c.Abort()
				apiFail(c, 500, "INTERNAL_ERROR", "内部错误")
			}
			logger.Info("http request", "requestId", id, "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status())
		}()
		c.Next()
	})
	router.NoRoute(func(c *gin.Context) { apiFail(c, 404, "NOT_FOUND", "接口不存在") })
	router.NoMethod(func(c *gin.Context) {
		allowed := []string{}
		parts := strings.Split(c.Request.URL.Path, "/")
		for _, route := range router.Routes() {
			pattern := strings.Split(route.Path, "/")
			if len(pattern) != len(parts) {
				continue
			}
			match := true
			for i, p := range pattern {
				if !strings.HasPrefix(p, ":") && p != parts[i] {
					match = false
					break
				}
			}
			if match {
				allowed = append(allowed, route.Method)
			}
		}
		sort.Strings(allowed)
		c.Header("Allow", strings.Join(allowed, ", "))
		apiFail(c, 405, "METHOD_NOT_ALLOWED", "请求方法不支持")
	})
	v := router.Group("/api/v1")
	v.GET("/health", func(c *gin.Context) { data(c, 200, gin.H{"status": "ok"}) })
	v.GET("/ready", func(c *gin.Context) {
		if storage.Ready(c.Request.Context(), db) != nil {
			apiFail(c, 503, "NOT_READY", "数据库未就绪")
			return
		}
		data(c, 200, gin.H{"status": "ready"})
	})
	v.GET("/posts", a.publicPosts)
	v.GET("/posts/:slug", func(c *gin.Context) {
		p, err := a.posts.FindPublic(c.Request.Context(), c.Param("slug"))
		if handleError(c, err, "POST_NOT_FOUND") {
			return
		}
		data(c, 200, p)
	})
	for _, table := range []string{"categories", "tags"} {
		v.GET("/"+table, func(c *gin.Context) {
			items, err := a.posts.Terms(c.Request.Context(), table)
			if handleError(c, err, "NOT_FOUND") {
				return
			}
			data(c, 200, items)
		})
	}
	v.GET("/music/playlists", func(c *gin.Context) {
		items, err := m.Playlists(c.Request.Context())
		if handleError(c, err, "PLAYLIST_NOT_FOUND") {
			return
		}
		data(c, 200, items)
	})
	v.GET("/music/playlists/:id/tracks", func(c *gin.Context) {
		f, _, ok := readFilter(c)
		if !ok {
			return
		}
		items, total, err := m.Tracks(c.Request.Context(), c.Param("id"), f.Page, f.PageSize)
		if handleError(c, err, "PLAYLIST_NOT_FOUND") {
			return
		}
		list(c, items, total, f)
	})
	v.GET("/music/recommendations/today", func(c *gin.Context) { a.daily(c, true) })
	v.GET("/music/recommendations", func(c *gin.Context) { a.daily(c, false) })
	v.POST("/admin/session", a.login)
	admin := v.Group("/admin", a.requireSession)
	admin.GET("/session", func(c *gin.Context) {
		data(c, 200, gin.H{"user": c.MustGet("user"), "csrfToken": auth.CSRF(c.GetString("sessionToken"))})
	})
	admin.POST("/session/logout", a.requireWrite, func(c *gin.Context) {
		if handleError(c, a.auth.Logout(c.Request.Context(), c.GetString("sessionToken")), "NOT_FOUND") {
			return
		}
		a.cookie(c, "", -1)
		c.Status(204)
	})
	admin.GET("/posts", func(c *gin.Context) {
		f, q, ok := readFilter(c)
		if !ok {
			return
		}
		status := q.Get("status")
		if status != "" && status != "draft" && status != "published" && status != "archived" {
			apiFail(c, 400, "INVALID_QUERY", "status 不合法")
			return
		}
		items, total, err := a.posts.ListAdmin(c.Request.Context(), f.Page, f.PageSize, status)
		if handleError(c, err, "POST_NOT_FOUND") {
			return
		}
		list(c, items, total, f)
	})
	admin.GET("/posts/:id", func(c *gin.Context) {
		p, err := a.posts.Get(c.Request.Context(), c.Param("id"))
		if handleError(c, err, "POST_NOT_FOUND") {
			return
		}
		data(c, 200, p)
	})
	admin.POST("/posts", a.requireWrite, a.createPost)
	admin.PATCH("/posts/:id", a.requireWrite, func(c *gin.Context) { a.changePost(c, "patch") })
	admin.POST("/posts/:id/publish", a.requireWrite, func(c *gin.Context) { a.changePost(c, "publish") })
	admin.POST("/posts/:id/archive", a.requireWrite, func(c *gin.Context) { a.changePost(c, "archive") })
	admin.POST("/music/playlists/:id/sync", a.requireWrite, func(c *gin.Context) {
		if !a.syncLimit.Allow(time.Now()) {
			apiFail(c, 429, "RATE_LIMITED", "同步请求过于频繁")
			return
		}
		run, err := m.StartSync(c.Request.Context(), c.Param("id"), c.GetString("requestId"))
		if handleError(c, err, "PLAYLIST_NOT_FOUND") {
			return
		}
		data(c, 202, gin.H{"runId": run.ID, "status": run.Status})
	})
	admin.GET("/music/sync-runs/:id", func(c *gin.Context) {
		run, err := m.GetRun(c.Request.Context(), c.Param("id"))
		if handleError(c, err, "SYNC_RUN_NOT_FOUND") {
			return
		}
		data(c, 200, run)
	})
	return router
}
func data(c *gin.Context, status int, v any) { c.JSON(status, gin.H{"data": v}) }
func list(c *gin.Context, v any, total int64, f blog.Filter) {
	c.JSON(200, gin.H{"data": v, "pagination": gin.H{"page": f.Page, "pageSize": f.PageSize, "total": total}})
}
func apiFail(c *gin.Context, status int, code, message string) {
	c.Abort()
	writeError(c.Writer, status, code, message, c.GetString("requestId"))
}
func handleError(c *gin.Context, err error, missing string) bool {
	if err == nil {
		return false
	}
	var network net.Error
	var sqlState interface{ SQLState() string }
	unavailable := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.As(err, &network)
	if errors.As(err, &sqlState) {
		code := sqlState.SQLState()
		unavailable = unavailable || strings.HasPrefix(code, "08") || strings.HasPrefix(code, "53") || strings.HasPrefix(code, "57") || code == "42P01"
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		apiFail(c, 404, missing, "资源不存在")
	case errors.Is(err, auth.ErrUnauthorized):
		apiFail(c, 401, "UNAUTHORIZED", "未登录或凭据无效")
	case errors.Is(err, blog.ErrInvalid), errors.Is(err, gorm.ErrForeignKeyViolated), errors.Is(err, provider.InvalidSource), errors.Is(err, provider.InvalidPayload):
		apiFail(c, 400, "INVALID_INPUT", "输入无效")
	case errors.Is(err, blog.ErrConflict), errors.Is(err, gorm.ErrDuplicatedKey):
		apiFail(c, 409, "CONFLICT", "slug 或版本冲突")
	case errors.Is(err, music.ErrBusy):
		apiFail(c, 409, "SYNC_IN_PROGRESS", "同步正在进行")
	case errors.Is(err, music.ErrCapacity):
		apiFail(c, 429, "RATE_LIMITED", "同步任务繁忙")
	case unavailable:
		apiFail(c, 503, "SERVICE_UNAVAILABLE", "服务暂时不可用")
	default:
		apiFail(c, 500, "INTERNAL_ERROR", "内部错误")
	}
	return true
}
func readFilter(c *gin.Context) (blog.Filter, url.Values, bool) {
	q, err := url.ParseQuery(c.Request.URL.RawQuery)
	f := blog.Filter{}
	if err != nil {
		apiFail(c, 400, "INVALID_QUERY", "查询参数格式不正确")
		return f, q, false
	}
	for _, values := range q {
		if len(values) != 1 || !utf8.ValidString(values[0]) || strings.ContainsRune(values[0], 0) {
			apiFail(c, 400, "INVALID_QUERY", "参数不可重复且必须为有效 UTF-8")
			return f, q, false
		}
	}
	var ok bool
	f.Page, ok = positiveInt(q, "page", 1, 0)
	if !ok {
		apiFail(c, 400, "INVALID_QUERY", "page 必须为正整数")
		return f, q, false
	}
	f.PageSize, ok = positiveInt(q, "pageSize", 20, 50)
	if !ok {
		apiFail(c, 400, "INVALID_QUERY", "pageSize 必须为 1–50")
		return f, q, false
	}
	f.Q = strings.TrimSpace(q.Get("q"))
	f.Category = q.Get("category")
	f.Tag = q.Get("tag")
	if utf8.RuneCountInString(f.Q) > 100 {
		apiFail(c, 400, "INVALID_QUERY", "q 最多 100 字符")
		return f, q, false
	}
	return f, q, true
}
func decode(c *gin.Context, dst any) bool {
	typ, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || typ != "application/json" {
		apiFail(c, 400, "INVALID_BODY", "需要 application/json")
		return false
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 2*1024*1024))
	if err != nil || !utf8.Valid(body) || bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
		apiFail(c, 400, "INVALID_BODY", "JSON 无效或请求体过大")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(dst) != nil {
		apiFail(c, 400, "INVALID_BODY", "JSON 字段或格式不正确")
		return false
	}
	if decoder.Decode(new(any)) != io.EOF {
		apiFail(c, 400, "INVALID_BODY", "仅允许一个 JSON 对象")
		return false
	}
	return true
}
func (a *databaseAPI) publicPosts(c *gin.Context) {
	f, _, ok := readFilter(c)
	if !ok {
		return
	}
	items, total, err := a.posts.ListPublic(c.Request.Context(), f)
	if handleError(c, err, "POST_NOT_FOUND") {
		return
	}
	list(c, items, total, f)
}
func (a *databaseAPI) origin(c *gin.Context) bool {
	if c.GetHeader("Origin") != a.config.Origin {
		apiFail(c, 403, "ORIGIN_FORBIDDEN", "Origin 不允许")
		return false
	}
	return true
}
func (a *databaseAPI) cookie(c *gin.Context, token string, age int) {
	http.SetCookie(c.Writer, &http.Cookie{Name: "blog_session", Value: token, Path: "/", MaxAge: age, HttpOnly: true, Secure: a.config.SecureCookie, SameSite: http.SameSiteLaxMode})
}
func (a *databaseAPI) login(c *gin.Context) {
	if !a.origin(c) {
		return
	}
	if !a.loginLimit.Allow(time.Now()) {
		c.Header("Retry-After", "60")
		apiFail(c, 429, "RATE_LIMITED", "登录请求过于频繁")
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(c, &input) {
		return
	}
	if len(input.Username) > 400 || len(input.Password) > 72 || input.Password == "" {
		apiFail(c, 400, "INVALID_INPUT", "凭据格式不正确")
		return
	}
	user, token, err := a.auth.Login(c.Request.Context(), input.Username, input.Password)
	if handleError(c, err, "NOT_FOUND") {
		return
	}
	a.cookie(c, token, int(a.config.SessionTTL.Seconds()))
	data(c, 200, gin.H{"user": user})
}
func (a *databaseAPI) requireSession(c *gin.Context) {
	cookie, err := c.Request.Cookie("blog_session")
	if err != nil {
		apiFail(c, 401, "UNAUTHORIZED", "未登录")
		return
	}
	user, err := a.auth.Current(c.Request.Context(), cookie.Value)
	if handleError(c, err, "NOT_FOUND") {
		return
	}
	c.Set("user", user)
	c.Set("sessionToken", cookie.Value)
	c.Next()
}
func (a *databaseAPI) requireWrite(c *gin.Context) {
	if !a.origin(c) {
		return
	}
	if !auth.CheckCSRF(c.GetString("sessionToken"), c.GetHeader("X-CSRF-Token")) {
		apiFail(c, 403, "CSRF_FAILED", "CSRF 校验失败")
		return
	}
	c.Next()
}
func (a *databaseAPI) createPost(c *gin.Context) {
	var input struct {
		Slug            string   `json:"slug"`
		Title           string   `json:"title"`
		Summary         string   `json:"summary"`
		ContentMarkdown string   `json:"contentMarkdown"`
		CategoryID      *string  `json:"categoryId"`
		TagIDs          []string `json:"tagIds"`
	}
	// Raw fields reject explicit null for fields other than categoryId.
	var fields map[string]json.RawMessage
	if !decode(c, &fields) {
		return
	}
	for key, value := range fields {
		if string(value) == "null" && key != "categoryId" {
			apiFail(c, 400, "INVALID_INPUT", "字段不接受 null")
			return
		}
	}
	raw, _ := json.Marshal(fields)
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&input) != nil {
		apiFail(c, 400, "INVALID_INPUT", "创建字段不正确")
		return
	}
	if input.TagIDs == nil {
		input.TagIDs = []string{}
	}
	p, err := a.posts.Create(c.Request.Context(), blog.Record{ID: auth.Token(), Slug: input.Slug, Title: input.Title, Summary: input.Summary, ContentMarkdown: input.ContentMarkdown, CategoryID: input.CategoryID, TagIDs: input.TagIDs})
	if handleError(c, err, "POST_NOT_FOUND") {
		return
	}
	data(c, 201, p)
}
func (a *databaseAPI) changePost(c *gin.Context, action string) {
	var fields map[string]json.RawMessage
	if !decode(c, &fields) {
		return
	}
	var version int64
	if json.Unmarshal(fields["version"], &version) != nil || version < 1 {
		apiFail(c, 400, "INVALID_INPUT", "version 必须为正整数")
		return
	}
	if action != "patch" && len(fields) != 1 {
		apiFail(c, 400, "INVALID_INPUT", "状态操作仅接受 version")
		return
	}
	p, err := a.posts.Change(c.Request.Context(), c.Param("id"), version, action, fields)
	if handleError(c, err, "POST_NOT_FOUND") {
		return
	}
	data(c, 200, p)
}
func (a *databaseAPI) daily(c *gin.Context, today bool) {
	now := time.Now().In(recommendation.Shanghai)
	date := recommendation.Date(now)
	if !today {
		_, query, ok := readFilter(c)
		if !ok {
			return
		}
		date = query.Get("date")
		parsed, err := time.ParseInLocation("2006-01-02", date, recommendation.Shanghai)
		midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, recommendation.Shanghai)
		if err != nil || parsed.After(midnight) || parsed.Before(midnight.AddDate(0, 0, -30)) {
			apiFail(c, 400, "INVALID_DATE", "date 必须为今天及前 30 个自然日")
			return
		}
	}
	result, err := a.recommendation.Get(c.Request.Context(), date)
	if today && errors.Is(err, gorm.ErrRecordNotFound) {
		apiFail(c, 503, "RECOMMENDATION_NOT_READY", "今日推荐尚未生成")
		return
	}
	if handleError(c, err, "RECOMMENDATION_NOT_FOUND") {
		return
	}
	data(c, 200, result)
}
