package netease

import (
	"blog-website/backend/internal/music/provider"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct{ HTTP *http.Client }

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// This public metadata endpoint may deny access. Denial never becomes an empty snapshot.
func (c *Client) FetchPlaylist(ctx context.Context, ref provider.Ref) ([]provider.Track, error) {
	if ref.Provider != "netease" || !provider.Decimal.MatchString(ref.ExternalID) || !provider.OfficialURL(ref.SourceURL, "netease", false) {
		return nil, provider.InvalidSource
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://music.163.com/api/playlist/detail?id="+url.QueryEscape(ref.ExternalID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "PersonalBlogMetadata/1.0")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if e, ok := err.(net.Error); ok && e.Timeout() {
			return nil, provider.Failure("UPSTREAM_TIMEOUT")
		}
		return nil, provider.Failure("UPSTREAM_UNAVAILABLE")
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 401, 403:
		return nil, provider.Failure("UPSTREAM_UNAUTHORIZED")
	case 429:
		return nil, provider.Failure("UPSTREAM_RATE_LIMITED")
	}
	if resp.StatusCode != 200 {
		return nil, provider.Failure("UPSTREAM_HTTP")
	}
	content, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || content != "application/json" {
		return nil, provider.InvalidPayload
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(body) > 2*1024*1024 {
		return nil, provider.Failure("RESPONSE_TOO_LARGE")
	}
	return Parse(body)
}

type artist struct {
	Name string `json:"name"`
}
type song struct {
	ID       json.Number `json:"id"`
	Name     string      `json:"name"`
	Artists  []artist    `json:"artists"`
	AR       []artist    `json:"ar"`
	Duration *int64      `json:"duration"`
	DT       *int64      `json:"dt"`
}
type playlist struct {
	TrackCount *int    `json:"trackCount"`
	Tracks     *[]song `json:"tracks"`
}

func Parse(body []byte) ([]provider.Track, error) {
	var p struct {
		Code     int       `json:"code"`
		Result   *playlist `json:"result"`
		Playlist *playlist `json:"playlist"`
	}
	if json.Unmarshal(body, &p) != nil {
		return nil, provider.InvalidPayload
	}
	if p.Code == 401 || p.Code == 403 {
		return nil, provider.Failure("UPSTREAM_UNAUTHORIZED")
	}
	if p.Code == 429 {
		return nil, provider.Failure("UPSTREAM_RATE_LIMITED")
	}
	if p.Code != 200 {
		return nil, provider.InvalidPayload
	}
	list := p.Playlist
	if list == nil {
		list = p.Result
	}
	if list == nil || list.TrackCount == nil || list.Tracks == nil || *list.TrackCount != len(*list.Tracks) || *list.TrackCount > 2000 {
		return nil, provider.InvalidPayload
	}
	tracks := []provider.Track{}
	seen := map[string]bool{}
	for _, s := range *list.Tracks {
		id := s.ID.String()
		if !provider.Decimal.MatchString(id) {
			return nil, provider.InvalidPayload
		}
		t := provider.Track{ID: "netease:" + id, Provider: "netease", ExternalID: id, Title: s.Name, SourceURL: "https://music.163.com/#/song?id=" + id, Availability: "unknown"}
		artists := s.AR
		if artists == nil {
			artists = s.Artists
		}
		names := []string{}
		for _, a := range artists {
			if strings.TrimSpace(a.Name) != "" {
				names = append(names, a.Name)
			}
		}
		if len(names) > 0 {
			v := strings.Join(names, " / ")
			t.Author = &v
		}
		duration := s.DT
		if duration == nil {
			duration = s.Duration
		}
		if duration != nil && *duration >= 0 && *duration <= int64(7*24*time.Hour/time.Millisecond) {
			v := int(*duration / 1000)
			t.DurationSeconds = &v
		}
		if err := provider.Validate([]provider.Track{t}, "netease"); err != nil {
			return nil, err
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		tracks = append(tracks, t)
	}
	return tracks, nil
}
