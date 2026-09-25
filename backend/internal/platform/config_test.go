package platform

import "testing"

func TestConfigFailsClosed(t *testing.T) {
	for _, key := range []string{"APP_ENV", "HTTP_ADDR", "DATABASE_URL", "ALLOWED_ORIGIN", "SESSION_TTL", "COOKIE_SECURE", "DEMO_MODE", "MUSIC_AUTO_SYNC"} {
		t.Setenv(key, "")
	}
	if _, err := LoadConfig(); err == nil {
		t.Fatal("missing database silently accepted")
	}
	t.Setenv("DEMO_MODE", "true")
	if _, err := LoadConfig(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("ALLOWED_ORIGIN", "https://example.test")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("production demo accepted")
	}
	t.Setenv("DEMO_MODE", "false")
	t.Setenv("DATABASE_URL", "postgres://unused")
	t.Setenv("COOKIE_SECURE", "false")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("insecure production cookie")
	}
	t.Setenv("COOKIE_SECURE", "true")
	cfg, err := LoadConfig()
	if err != nil || !cfg.SecureCookie {
		t.Fatal("valid production rejected", err)
	}
	t.Setenv("ALLOWED_ORIGIN", "https://example.test/")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("origin with path accepted")
	}
}
