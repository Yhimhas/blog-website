package platform

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr, DatabaseURL, Origin, Env    string
	SessionTTL                        time.Duration
	SecureCookie, Demo, MusicAutoSync bool
}

func LoadConfig() (Config, error) {
	c := Config{Addr: os.Getenv("HTTP_ADDR"), DatabaseURL: os.Getenv("DATABASE_URL"), Origin: os.Getenv("ALLOWED_ORIGIN"), Env: os.Getenv("APP_ENV"), SessionTTL: 24 * time.Hour}
	if c.Addr == "" {
		c.Addr = "127.0.0.1:8081"
	}
	if c.Env == "" {
		c.Env = "development"
	}
	if c.Env != "development" && c.Env != "production" {
		return c, errors.New("APP_ENV must be development or production")
	}
	if c.Origin == "" {
		c.Origin = "http://localhost:5173"
	}
	u, err := url.Parse(c.Origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return c, errors.New("ALLOWED_ORIGIN must be an exact http(s) origin without a trailing slash")
	}
	if v := os.Getenv("SESSION_TTL"); v != "" {
		c.SessionTTL, err = time.ParseDuration(v)
		if err != nil || c.SessionTTL < time.Minute || c.SessionTTL > 30*24*time.Hour {
			return c, errors.New("SESSION_TTL must be between 1m and 720h")
		}
	}
	c.SecureCookie = c.Env == "production"
	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		c.SecureCookie, err = strconv.ParseBool(v)
		if err != nil {
			return c, errors.New("COOKIE_SECURE must be boolean")
		}
	}
	if c.Env == "production" && (!c.SecureCookie || u.Scheme != "https") {
		return c, errors.New("production requires HTTPS origin and secure cookies")
	}
	if v := os.Getenv("DEMO_MODE"); v != "" {
		c.Demo, err = strconv.ParseBool(v)
		if err != nil {
			return c, errors.New("DEMO_MODE must be boolean")
		}
	}
	if c.Demo && c.Env == "production" {
		return c, errors.New("demo mode is development only")
	}
	if v := os.Getenv("MUSIC_AUTO_SYNC"); v != "" {
		c.MusicAutoSync, err = strconv.ParseBool(v)
		if err != nil {
			return c, errors.New("MUSIC_AUTO_SYNC must be boolean")
		}
	}
	if !c.Demo && c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required; use DEMO_MODE=true only for the original in-memory demo")
	}
	return c, nil
}
