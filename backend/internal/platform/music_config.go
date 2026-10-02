package platform

import (
	"blog-website/backend/internal/music/playback"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (c *Config) loadMusic() error {
	c.PlaybackPolicy = playback.Policy{Mode: os.Getenv("MUSIC_PLAYBACK_POLICY")}
	if c.PlaybackPolicy.Mode == "" {
		c.PlaybackPolicy.Mode = "public_only"
	}
	if c.PlaybackPolicy.Mode != "public_only" && c.PlaybackPolicy.Mode != "licensed" {
		return errors.New("invalid MUSIC_PLAYBACK_POLICY")
	}
	var err error
	if v := os.Getenv("MUSIC_REQUIRE_FULL"); v != "" {
		c.PlaybackPolicy.RequireFull, err = strconv.ParseBool(v)
		if err != nil {
			return errors.New("invalid MUSIC_REQUIRE_FULL")
		}
	}
	c.GrantsFile = os.Getenv("MUSIC_PUBLIC_GRANTS_FILE")
	c.NeteaseCookieFile = os.Getenv("MUSIC_NETEASE_COOKIE_FILE")
	c.MusicWebRoot = os.Getenv("MUSIC_WEB_ROOT")
	if c.MusicWebRoot != "" && !filepath.IsAbs(c.MusicWebRoot) {
		return errors.New("MUSIC_WEB_ROOT must be absolute")
	}
	for _, path := range []string{c.GrantsFile, c.NeteaseCookieFile} {
		if path != "" && !filepath.IsAbs(path) {
			return errors.New("music private configuration requires an absolute file path")
		}
		if path != "" {
			realPath, err := filepath.EvalSymlinks(path)
			if err != nil {
				return errors.New("music private configuration file unavailable")
			}
			path = realPath
			if c.MusicWebRoot != "" {
				rel, err := filepath.Rel(c.MusicWebRoot, path)
				if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
					return errors.New("music private configuration is inside the web root")
				}
			}
			cwd, _ := os.Getwd()
			for dir := cwd; dir != ""; dir = filepath.Dir(dir) {
				protected := ""
				if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
					protected = dir
				}
				if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
					protected = filepath.Dir(dir)
				}
				if protected != "" {
					rel, err := filepath.Rel(protected, path)
					if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
						return errors.New("music private configuration must be outside the repository and web root")
					}
					break
				}
				if filepath.Dir(dir) == dir {
					break
				}
			}
		}
	}
	if c.PlaybackPolicy.Mode == "licensed" && c.GrantsFile == "" {
		return errors.New("licensed playback requires an operator grant file")
	}
	c.PlaybackPolicy.Grants, err = playback.LoadGrants(c.GrantsFile)
	if err != nil {
		return err
	}
	for _, raw := range strings.Split(os.Getenv("MUSIC_TRUSTED_PROXIES"), ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if ip := net.ParseIP(raw); ip != nil {
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			raw = ip.String() + "/" + strconv.Itoa(bits)
		}
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			return errors.New("invalid MUSIC_TRUSTED_PROXIES")
		}
		ones, _ := network.Mask.Size()
		if ones == 0 {
			return errors.New("trusting all proxy addresses is not supported")
		}
		c.TrustedMusicProxies = append(c.TrustedMusicProxies, network)
	}
	c.PlaybackOptions = playback.Options{Policy: c.PlaybackPolicy}
	for _, entry := range []struct {
		env      string
		value    *int
		def, max int
	}{
		{"MUSIC_OWNER_REQUESTS_PER_MINUTE", &c.PlaybackOptions.OwnerPerMinute, 10, 600},
		{"MUSIC_IP_REQUESTS_PER_MINUTE", &c.PlaybackOptions.IPPerMinute, 10, 6000},
		{"MUSIC_MAX_SESSIONS", &c.PlaybackOptions.MaxSessions, 128, 4096},
		{"MUSIC_MAX_RESOLVERS", &c.PlaybackOptions.MaxResolving, 2, 32},
		{"MUSIC_MAX_STREAMS", &c.PlaybackOptions.MaxStreams, 4, 128},
		{"MUSIC_MAX_TRANSCODERS", &c.PlaybackOptions.MaxTranscoders, 2, 32},
		{"MUSIC_ACCOUNT_REQUESTS_PER_MINUTE", &c.AccountPerMinute, 20, 120},
		{"MUSIC_ACCOUNT_REJECT_THRESHOLD", &c.AccountRejectThreshold, 3, 20},
	} {
		*entry.value = entry.def
		if raw := os.Getenv(entry.env); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > entry.max {
				return errors.New("invalid " + entry.env)
			}
			*entry.value = n
		}
	}
	for _, entry := range []struct {
		env   string
		value *int64
		def   int64
	}{
		{"MUSIC_MAX_STREAM_BYTES", &c.PlaybackOptions.MaxStreamBytes, 256 * 1024 * 1024},
		{"MUSIC_HOURLY_BYTES", &c.PlaybackOptions.HourlyBytes, 1024 * 1024 * 1024},
	} {
		*entry.value = entry.def
		if raw := os.Getenv(entry.env); raw != "" {
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || n < 1024 || n > 1<<40 {
				return errors.New("invalid " + entry.env)
			}
			*entry.value = n
		}
	}
	c.PlaybackOptions.MaxStreamDuration = 6 * time.Hour
	c.AccountCooldown = time.Minute
	if raw := os.Getenv("MUSIC_ACCOUNT_COOLDOWN"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d < time.Second || d > 24*time.Hour {
			return errors.New("invalid MUSIC_ACCOUNT_COOLDOWN")
		}
		c.AccountCooldown = d
	}
	if raw := os.Getenv("MUSIC_MAX_STREAM_DURATION"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d < time.Second || d > 6*time.Hour {
			return errors.New("invalid MUSIC_MAX_STREAM_DURATION")
		}
		c.PlaybackOptions.MaxStreamDuration = d
	}
	return nil
}

func (c Config) MusicClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil {
		return "invalid-peer"
	}
	trusted := func(ip net.IP) bool {
		for _, cidr := range c.TrustedMusicProxies {
			if cidr.Contains(ip) {
				return true
			}
		}
		return false
	}
	if !trusted(peer) {
		return peer.String()
	}
	raw := r.Header.Get("X-Forwarded-For")
	if len(raw) > 1024 {
		return peer.String()
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 16 {
		return peer.String()
	}
	chain := []net.IP{}
	for _, part := range parts {
		ip := net.ParseIP(strings.TrimSpace(part))
		if ip == nil {
			return peer.String()
		}
		chain = append(chain, ip)
	}
	current := peer
	for i := len(chain) - 1; i >= 0; i-- {
		if !trusted(current) {
			break
		}
		current = chain[i]
	}
	return current.String()
}
