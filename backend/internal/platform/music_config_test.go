package platform

import (
	"net"
	"net/http/httptest"
	"testing"
)

func TestTrustedMusicProxyIP(t *testing.T) {
	_, v4, _ := net.ParseCIDR("127.0.0.1/32")
	_, v6, _ := net.ParseCIDR("::1/128")
	c := Config{TrustedMusicProxies: []*net.IPNet{v4, v6}}
	for _, test := range []struct{ peer, forwarded, want string }{
		{"198.51.100.9:2345", "1.2.3.4", "198.51.100.9"},
		{"127.0.0.1:2345", "198.51.100.9", "198.51.100.9"},
		{"[::1]:2345", "2001:db8::123", "2001:db8::123"},
		{"127.0.0.1:2345", "1.2.3.4, 198.51.100.9", "198.51.100.9"},
		{"127.0.0.1:2345", "bad-value", "127.0.0.1"},
		{"127.0.0.1:2345", "", "127.0.0.1"},
	} {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = test.peer
		req.Header.Set("X-Forwarded-For", test.forwarded)
		if got := c.MusicClientIP(req); got != test.want {
			t.Fatalf("peer=%s got=%s want=%s", test.peer, got, test.want)
		}
	}
}

func TestMusicConfigLimitsFailClosed(t *testing.T) {
	t.Setenv("MUSIC_PLAYBACK_POLICY", "public_only")
	for _, test := range []struct{ key, value string }{
		{"MUSIC_TRUSTED_PROXIES", "0.0.0.0/0"}, {"MUSIC_TRUSTED_PROXIES", "bad"},
		{"MUSIC_MAX_STREAM_BYTES", "-1"}, {"MUSIC_OWNER_REQUESTS_PER_MINUTE", "0"},
		{"MUSIC_MAX_STREAM_DURATION", "7h"}, {"MUSIC_PLAYBACK_POLICY", "open_account"},
		{"MUSIC_NETEASE_COOKIE_FILE", "credentials-inside-repository"},
	} {
		t.Run(test.key+test.value, func(t *testing.T) {
			t.Setenv(test.key, test.value)
			c := Config{}
			if err := c.loadMusic(); err == nil {
				t.Fatal("invalid music config accepted")
			}
		})
	}
}
