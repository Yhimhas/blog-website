package platform

import (
	"blog-website/backend/internal/auth"
	"blog-website/backend/internal/music/playback"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"net/http"
	"strings"
)

const playbackCookie = "music_playback"

func playbackStatus(e playback.Failure) int {
	switch e {
	case playback.NotFound:
		return 404
	case playback.Unsupported, playback.SourceUnavailable:
		return 422
	case playback.Busy, playback.RateLimited:
		return 429
	case playback.Expired:
		return 410
	default:
		return 503
	}
}
func bindPlayback(v *gin.RouterGroup, p *playback.Service, cfg Config) {
	write := func(c *gin.Context) bool {
		c.Header("Cache-Control", "no-store")
		if c.GetHeader("Origin") != cfg.Origin {
			apiFail(c, 403, "ORIGIN_REJECTED", "请从本站发起播放")
			return false
		}
		return true
	}
	enabled := func(c *gin.Context) bool {
		c.Header("Cache-Control", "no-store")
		if p == nil {
			apiFail(c, 503, "PLAYBACK_DISABLED", "站内播放暂未开放，请前往原站")
			return false
		}
		return true
	}
	owner := func(c *gin.Context) string { x, _ := c.Cookie(playbackCookie); return x }
	v.POST("/music/playback-sessions", func(c *gin.Context) {
		if !enabled(c) || !write(c) {
			return
		}
		media, _, e := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if e != nil || media != "application/json" {
			apiFail(c, 400, "INVALID_BODY", "需要 JSON")
			return
		}
		var body struct {
			TrackID string `json:"trackId"`
		}
		d := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1024))
		d.DisallowUnknownFields()
		if d.Decode(&body) != nil || d.Decode(new(any)) != io.EOF || body.TrackID == "" || len(body.TrackID) > 150 {
			apiFail(c, 400, "INVALID_BODY", "曲目参数不正确")
			return
		}
		o := owner(c)
		if len(o) != 64 {
			o = auth.Token()
			http.SetCookie(c.Writer, &http.Cookie{Name: playbackCookie, Value: o, Path: "/api/v1/music", HttpOnly: true, Secure: cfg.SecureCookie, SameSite: http.SameSiteStrictMode, MaxAge: 7 * 3600})
		}
		ip := cfg.MusicClientIP(c.Request)
		out, err := p.Create(c.Request.Context(), o, ip, body.TrackID)
		if err != nil {
			code := playback.PublicCode(playback.Code(err))
			apiFail(c, playbackStatus(code), string(code), "暂时无法播放此曲目")
			return
		}
		data(c, 202, out)
	})
	v.GET("/music/playback-sessions/:id", func(c *gin.Context) {
		if !enabled(c) {
			return
		}
		out, err := p.Get(c.Param("id"), owner(c))
		if err != nil {
			apiFail(c, 410, "SESSION_EXPIRED", "会话已过期，请重新播放")
			return
		}
		data(c, 200, out)
	})
	v.POST("/music/playback-sessions/:id/stop", func(c *gin.Context) {
		if !enabled(c) || !write(c) {
			return
		}
		if err := p.Stop(c.Param("id"), owner(c)); err != nil {
			apiFail(c, 410, "SESSION_EXPIRED", "会话已过期")
			return
		}
		c.Status(204)
	})
}
func NewMusicStreamHandler(p *playback.Service, cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		fail := func(status int, e playback.Failure) {
			e = playback.PublicCode(e)
			writeError(w, status, string(e), "音频流暂时不可用，请重新播放", auth.Token()[:32])
		}
		if r.Method != "GET" {
			w.Header().Set("Allow", "GET")
			fail(405, playback.Unsupported)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/music/streams/") {
			fail(404, playback.NotFound)
			return
		}
		if o := r.Header.Get("Origin"); o != "" && o != cfg.Origin {
			fail(403, playback.Unsupported)
			return
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			fail(403, playback.Unsupported)
			return
		}
		cookie, err := r.Cookie(playbackCookie)
		if err != nil || len(cookie.Value) != 64 {
			fail(410, playback.Expired)
			return
		}
		// Range is intentionally ignored: respond with a full 200 stream, never a fake 206.
		p.Stream(w, r, strings.TrimPrefix(r.URL.Path, "/api/v1/music/streams/"), cookie.Value, fail)
	})
}
