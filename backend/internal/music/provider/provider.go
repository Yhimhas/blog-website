package provider

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

type Ref struct {
	Provider, ExternalID, Title, SourceURL string
	EmbedURL                               *string
}
type Track struct {
	ID              string  `json:"id"`
	Provider        string  `json:"provider"`
	ExternalID      string  `json:"externalId"`
	PartID          *string `json:"partId"`
	Title           string  `json:"title"`
	Author          *string `json:"author"`
	SourceURL       string  `json:"sourceUrl"`
	DurationSeconds *int    `json:"durationSeconds"`
	Availability    string  `json:"availability"`
	EmbedURL        *string `json:"embedUrl"`
	PlaylistID      string  `json:"playlistId,omitempty"`
}
type Adapter interface {
	FetchPlaylist(context.Context, Ref) ([]Track, error)
}

var Decimal = regexp.MustCompile(`^[1-9][0-9]{0,29}$`)
var BVID = regexp.MustCompile(`^BV[0-9A-Za-z]{10}$`)

type Failure string

func (e Failure) Error() string { return string(e) }

const InvalidPayload Failure = "INVALID_PAYLOAD"
const InvalidSource Failure = "INVALID_SOURCE"

func Code(err error) string {
	var failure Failure
	if errors.As(err, &failure) {
		return string(failure)
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "UPSTREAM_TIMEOUT"
	}
	return "UPSTREAM_UNAVAILABLE"
}
func OfficialURL(raw, kind string, embed bool) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	if kind == "netease" {
		if u.Host != "music.163.com" {
			return false
		}
		if embed {
			return u.Path == "/outchain/player" && Decimal.MatchString(u.Query().Get("id"))
		}
		return u.Path == "/playlist" || u.Path == "/m/playlist" || u.Path == "/song" || (u.Path == "/" && strings.HasPrefix(u.Fragment, "/song?id="))
	}
	if kind == "bilibili" {
		if embed {
			return u.Host == "player.bilibili.com" && u.Path == "/player.html" && BVID.MatchString(u.Query().Get("bvid"))
		}
		return u.Host == "www.bilibili.com" && strings.HasPrefix(u.Path, "/video/")
	}
	return false
}
func Validate(items []Track, kind string) error {
	if len(items) > 2000 {
		return InvalidPayload
	}
	seen := map[string]bool{}
	for _, t := range items {
		if t.Provider != kind || strings.TrimSpace(t.Title) == "" || !utf8.ValidString(t.Title) || utf8.RuneCountInString(t.Title) > 300 || strings.ContainsRune(t.Title, 0) || !OfficialURL(t.SourceURL, kind, false) {
			return InvalidPayload
		}
		if kind == "netease" && (!Decimal.MatchString(t.ExternalID) || t.PartID != nil || t.ID != "netease:"+t.ExternalID) {
			return InvalidPayload
		}
		if kind == "bilibili" && (!BVID.MatchString(t.ExternalID) || t.PartID == nil || !Decimal.MatchString(*t.PartID) || t.ID != "bilibili:"+t.ExternalID+":"+*t.PartID) {
			return InvalidPayload
		}
		if t.Author != nil && (!utf8.ValidString(*t.Author) || utf8.RuneCountInString(*t.Author) > 160 || strings.ContainsRune(*t.Author, 0)) {
			return InvalidPayload
		}
		if t.Availability != "available" && t.Availability != "unknown" && t.Availability != "unavailable" {
			return InvalidPayload
		}
		if t.DurationSeconds != nil && *t.DurationSeconds < 0 {
			return InvalidPayload
		}
		if t.EmbedURL != nil && !OfficialURL(*t.EmbedURL, kind, true) {
			return InvalidPayload
		}
		if seen[t.ID] {
			return InvalidPayload
		}
		seen[t.ID] = true
	}
	return nil
}
