package netease

import (
	"blog-website/backend/internal/music/playback"
	"blog-website/backend/internal/music/provider"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Credential struct{ Cookie, Version string }
type CredentialReader interface{ Read() (Credential, error) }
type CookieFile struct{ Path string }

func (f CookieFile) Read() (Credential, error) {
	if !filepath.IsAbs(f.Path) {
		return Credential{}, playback.SourceAuthExpired
	}
	info, err := os.Lstat(f.Path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 16*1024 || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return Credential{}, playback.SourceAuthExpired
	}
	file, err := os.Open(f.Path)
	if err != nil {
		return Credential{}, playback.SourceAuthExpired
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return Credential{}, playback.SourceAuthExpired
	}
	bytes, err := io.ReadAll(io.LimitReader(file, 16*1024+1))
	if err != nil || len(bytes) > 16*1024 {
		return Credential{}, playback.SourceAuthExpired
	}
	cookie := strings.TrimSpace(string(bytes))
	if cookie == "" || strings.ContainsAny(cookie, "\r\n\x00") {
		return Credential{}, playback.SourceAuthExpired
	}
	for _, b := range []byte(cookie) {
		if b < 32 || b > 126 {
			return Credential{}, playback.SourceAuthExpired
		}
	}
	req := http.Request{Header: http.Header{"Cookie": {cookie}}}
	if c, err := req.Cookie("MUSIC_U"); err != nil || c.Value == "" {
		return Credential{}, playback.SourceAuthExpired
	}
	sum := sha256.Sum256([]byte(cookie))
	return Credential{Cookie: cookie, Version: hex.EncodeToString(sum[:])}, nil
}

// Fixed official endpoints only, no Jar, redirects or environment proxy.
// Account credentials never enter Audio.Headers or the CDN media client.
type AccountResolver struct {
	Credentials                    CredentialReader
	HTTP                           *http.Client
	Control                        playback.Control
	Limits                         AccountLimits
	mu                             sync.Mutex
	version, disabledVersion       string
	checkedUntil, cooldown, window time.Time
	requests, rejected             int
}

type AccountLimits struct {
	PerMinute, RejectThreshold int
	Cooldown                   time.Duration
}

func (l AccountLimits) defaults() AccountLimits {
	if l.PerMinute <= 0 {
		l.PerMinute = 20
	}
	if l.RejectThreshold <= 0 {
		l.RejectThreshold = 3
	}
	if l.Cooldown <= 0 {
		l.Cooldown = time.Minute
	}
	return l
}
func NewAccountResolver(path string, control playback.Control, limits ...AccountLimits) (*AccountResolver, error) {
	f := CookieFile{Path: path}
	if _, err := f.Read(); err != nil {
		return nil, err
	}
	r := &AccountResolver{Credentials: f, HTTP: New().HTTP, Control: control}
	if len(limits) > 0 {
		r.Limits = limits[0]
	}
	return r, nil
}
func (r *AccountResolver) credential() (Credential, error) {
	c, err := r.Credentials.Read()
	if err != nil {
		return Credential{}, playback.SourceAuthExpired
	}
	if c.Version == "" {
		sum := sha256.Sum256([]byte(c.Cookie))
		c.Version = hex.EncodeToString(sum[:])
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.version != c.Version {
		r.version = c.Version
		r.disabledVersion = ""
		r.checkedUntil = time.Time{}
		r.cooldown = time.Time{}
		r.rejected = 0
	}
	if r.disabledVersion == c.Version {
		return Credential{}, playback.SourceAuthExpired
	}
	if time.Now().Before(r.cooldown) {
		return Credential{}, playback.AccessRestricted
	}
	return c, nil
}
func (r *AccountResolver) valid(version string) bool {
	c, err := r.credential()
	return err == nil && c.Version == version
}
func (r *AccountResolver) rejectedCall(version string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.version != version {
		return
	}
	if err == playback.SourceAuthExpired {
		r.disabledVersion = version
		return
	}
	if err == playback.AccessRestricted || err == playback.RateLimited {
		r.rejected++
		if r.rejected >= r.Limits.defaults().RejectThreshold {
			r.cooldown = time.Now().Add(r.Limits.defaults().Cooldown)
		}
	}
}
func (r *AccountResolver) request(ctx context.Context, c Credential, path string, q url.Values) ([]byte, error) {
	r.mu.Lock()
	now := time.Now()
	if now.Sub(r.window) >= time.Minute {
		r.window = now
		r.requests = 0
	}
	if r.requests >= r.Limits.defaults().PerMinute {
		r.mu.Unlock()
		return nil, playback.RateLimited
	}
	r.requests++
	r.mu.Unlock()
	if r.Control != nil {
		if err := r.Control.Rate(ctx, "netease-account", r.Limits.defaults().PerMinute, time.Minute); err != nil {
			return nil, playback.RateLimited
		}
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://music.163.com"+path+"?"+q.Encode(), nil)
	if err != nil {
		return nil, playback.Unsupported
	}
	req.Header.Set("Cookie", c.Cookie)
	req.Header.Set("User-Agent", audioUserAgent)
	req.Header.Set("Referer", audioReferer)
	req.Header.Set("Accept", "application/json")
	client := r.HTTP
	if client == nil {
		client = New().HTTP
	}
	copyClient := *client
	copyClient.Jar = nil
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := copyClient.Do(req)
	if err != nil {
		return nil, playback.Code(err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 401:
		if path == "/api/w/nuser/account/get" {
			return nil, playback.SourceAuthExpired
		}
		return nil, playback.AccessRestricted
	case 403:
		return nil, playback.AccessRestricted
	case 429:
		return nil, playback.RateLimited
	}
	if resp.StatusCode != 200 {
		return nil, playback.Unavailable
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024+1))
	if err != nil {
		return nil, playback.Code(err)
	}
	if len(body) > 512*1024 {
		return nil, playback.Unsupported
	}
	return body, nil
}
func (r *AccountResolver) Resolve(ctx context.Context, t provider.Track) (audio playback.Audio, err error) {
	if t.Provider != "netease" || !provider.Decimal.MatchString(t.ExternalID) || t.ID != "netease:"+t.ExternalID {
		return audio, playback.Unsupported
	}
	c, err := r.credential()
	if err != nil {
		return audio, err
	}
	defer func() {
		audio.SourceMode = "account"
		audio.CredentialVersion = c.Version
		if err != nil {
			audio.Audience = "blocked"
			r.rejectedCall(c.Version, err)
		}
	}()
	r.mu.Lock()
	check := time.Now().After(r.checkedUntil)
	r.mu.Unlock()
	if check {
		body, e := r.request(ctx, c, "/api/w/nuser/account/get", nil)
		if e != nil {
			return audio, e
		}
		var status struct {
			Code    *int            `json:"code"`
			Account json.RawMessage `json:"account"`
			Profile json.RawMessage `json:"profile"`
		}
		if json.Unmarshal(body, &status) != nil || status.Code == nil {
			return audio, playback.Unsupported
		}
		if *status.Code != 200 {
			if *status.Code == 301 || *status.Code == 401 {
				return audio, playback.SourceAuthExpired
			}
			return audio, playback.AccessRestricted
		}
		if len(status.Account) == 0 || len(status.Profile) == 0 {
			return audio, playback.Unsupported
		}
		if string(status.Account) == "null" || string(status.Profile) == "null" {
			return audio, playback.SourceAuthExpired
		}
		var account, profile map[string]json.RawMessage
		if json.Unmarshal(status.Account, &account) != nil || json.Unmarshal(status.Profile, &profile) != nil || len(account) == 0 || len(profile) == 0 {
			return audio, playback.Unsupported
		}
		r.mu.Lock()
		if r.version == c.Version {
			r.checkedUntil = time.Now().Add(time.Minute)
		}
		r.mu.Unlock()
	}
	body, err := r.request(ctx, c, "/api/song/enhance/player/url", url.Values{"ids": {"[" + t.ExternalID + "]"}, "br": {"320000"}})
	if err != nil {
		return audio, err
	}
	audio, err = parseAccountAudio(body, t)
	if err != nil {
		return audio, err
	}
	audio.CredentialVersion = c.Version
	audio.Valid = func() bool { return r.valid(c.Version) }
	r.mu.Lock()
	r.rejected = 0
	r.mu.Unlock()
	return audio, nil
}

func parseAccountAudio(body []byte, t provider.Track) (playback.Audio, error) {
	var envelope struct {
		Code *int            `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(body, &envelope) != nil || envelope.Code == nil {
		return playback.Audio{}, playback.Unsupported
	}
	switch *envelope.Code {
	case 200:
	case 401, 301:
		return playback.Audio{}, playback.AccessRestricted
	case 429:
		return playback.Audio{}, playback.RateLimited
	default:
		return playback.Audio{}, playback.AccessRestricted
	}
	var entries []struct {
		ID        json.Number     `json:"id"`
		URL       *string         `json:"url"`
		Code      int             `json:"code"`
		Type      string          `json:"type"`
		Time      *int64          `json:"time"`
		Fee       int             `json:"fee"`
		FreeTrial json.RawMessage `json:"freeTrialInfo"`
	}
	if json.Unmarshal(envelope.Data, &entries) != nil || len(entries) != 1 || entries[0].ID.String() != t.ExternalID {
		return playback.Audio{}, playback.Unsupported
	}
	e := entries[0]
	if e.URL == nil || *e.URL == "" || e.Code != 200 {
		if e.Code == 429 {
			return playback.Audio{}, playback.RateLimited
		}
		if e.Fee > 0 {
			return playback.Audio{}, playback.EntitlementRequired
		}
		return playback.Audio{}, playback.SourceUnavailable
	}
	u, err := url.Parse(*e.URL)
	if err != nil || !officialAudioHost(u.Hostname()) {
		return playback.Audio{}, playback.Unsupported
	}
	if u.Scheme == "http" && u.Port() == "" {
		u.Scheme = "https"
	}
	if !playback.MediaURL(u.String()) {
		return playback.Audio{}, playback.Unsupported
	}
	format := e.Type
	switch format {
	case "mp3", "flac", "aac", "ogg", "wav":
	default:
		return playback.Audio{}, playback.Unsupported
	}
	a := playback.Audio{URL: u.String(), MIME: playback.FormatMIME(format), InputFormat: format, Transcode: format != "mp3", SourceMode: "account", Audience: "private", RedirectPolicy: audioRedirect, Headers: map[string]string{"User-Agent": audioUserAgent, "Referer": audioReferer}, Capability: playback.Capability{MediaKind: "unknown", TrackDuration: t.DurationSeconds}}
	if e.Time != nil && *e.Time > 0 && *e.Time <= 7*24*3600*1000 {
		seconds := int(*e.Time / 1000)
		a.Duration = &seconds
		a.Capability.StreamDuration = &seconds
	}
	if len(e.FreeTrial) > 0 && string(e.FreeTrial) != "null" {
		var trial struct {
			Start *int64 `json:"start"`
			End   *int64 `json:"end"`
		}
		if json.Unmarshal(e.FreeTrial, &trial) != nil || trial.Start == nil || trial.End == nil || *trial.Start < 0 || *trial.End <= *trial.Start || *trial.End > 7*24*3600*1000 {
			return playback.Audio{}, playback.Unsupported
		}
		start, end := int(*trial.Start/1000), int(*trial.End/1000)
		if end <= start {
			return playback.Audio{}, playback.Unsupported
		}
		duration := end - start
		a.Capability.MediaKind = "preview"
		a.Capability.PreviewStart = &start
		a.Capability.PreviewEnd = &end
		a.Capability.StreamDuration = &duration
		a.Duration = &duration
	}
	// Missing/null trial information is not proof of completeness.
	return a, nil
}
