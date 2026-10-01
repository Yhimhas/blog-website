package netease

import (
	"blog-website/backend/internal/music/provider"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
)

type Client struct{ HTTP *http.Client }

const maxTracks = 2000
const detailBatchSize = 50

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// This public metadata endpoint may deny access. Denial never becomes an empty snapshot.
func (c *Client) FetchPlaylist(ctx context.Context, ref provider.Ref) ([]provider.Track, error) {
	if ref.Provider != "netease" || !provider.Decimal.MatchString(ref.ExternalID) || !provider.SourceURL(ref.SourceURL, "netease") {
		return nil, provider.InvalidSource
	}
	source, _ := url.Parse(ref.SourceURL)
	if source.Query().Get("id") != ref.ExternalID {
		return nil, provider.InvalidSource
	}
	detail, err := c.fetchDetail(ctx, ref.ExternalID)
	if err != nil {
		return nil, err
	}
	missing := []string{}
	for _, id := range uniqueIDs(detail.TrackIDs) {
		if _, ok := detail.Tracks[id]; !ok {
			missing = append(missing, id)
		}
	}
	for start := 0; start < len(missing); start += detailBatchSize {
		end := start + detailBatchSize
		if end > len(missing) {
			end = len(missing)
		}
		items, err := c.fetchSongDetails(ctx, missing[start:end])
		if err != nil {
			return nil, err
		}
		for id, track := range items {
			detail.Tracks[id] = track
		}
	}
	if len(missing) > 0 {
		// Do not mix details from two versions of a playlist. No automatic retries.
		latest, err := c.fetchDetail(ctx, ref.ExternalID)
		if err != nil {
			return nil, err
		}
		if latest.TrackCount != detail.TrackCount || !reflect.DeepEqual(latest.TrackIDs, detail.TrackIDs) {
			return nil, provider.IncompletePlaylist
		}
	}
	return assembleComplete(detail)
}

func (c *Client) fetchDetail(ctx context.Context, id string) (*PlaylistDetail, error) {
	body, err := c.requestJSON(ctx, "/api/v6/playlist/detail", url.Values{"id": {id}, "n": {"2000"}, "s": {"0"}})
	if err != nil {
		return nil, err
	}
	detail, err := parseDetail(body)
	if err == nil {
		provider.SyncLogger(ctx).Info("music playlist metadata", "expectedCount", detail.TrackCount, "idCount", len(detail.TrackIDs), "actualCount", len(detail.Tracks))
	}
	return detail, err
}

func (c *Client) requestJSON(ctx context.Context, path string, query url.Values) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://music.163.com"+path+"?"+query.Encode(), nil)
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
	provider.SyncLogger(ctx).Info("music metadata HTTP", "endpoint", path, "httpStatus", resp.StatusCode)
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
	// These fixed official endpoints also return JSON labelled text/plain.
	// JSON decoding below remains mandatory; HTML and arbitrary content are rejected.
	if err != nil || (content != "application/json" && content != "text/plain") {
		return nil, provider.InvalidPayload
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			return nil, provider.Failure("UPSTREAM_TIMEOUT")
		}
		return nil, provider.Failure("UPSTREAM_UNAVAILABLE")
	}
	if len(body) > 2*1024*1024 {
		return nil, provider.Failure("RESPONSE_TOO_LARGE")
	}
	var status struct {
		Code *int `json:"code"`
	}
	if json.Unmarshal(body, &status) == nil && status.Code != nil {
		provider.SyncLogger(ctx).Info("music metadata business status", "endpoint", path, "businessCode", *status.Code)
	}
	return body, nil
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
	TrackCount *int      `json:"trackCount"`
	TrackIDs   *[]songID `json:"trackIds"`
	Tracks     *[]song   `json:"tracks"`
}

type songID struct {
	ID json.Number `json:"id"`
}

// TrackIDs preserves the declared count and order, including repeated entries.
// Tracks is only a candidate index; it is never submitted as a partial snapshot.
type PlaylistDetail struct {
	TrackCount int
	TrackIDs   []string
	Tracks     map[string]provider.Track
}

func Parse(body []byte) ([]provider.Track, error) {
	detail, err := parseDetail(body)
	if err != nil {
		return nil, err
	}
	return assembleComplete(detail)
}

func classifyBusinessCode(code *int) error {
	if code == nil {
		return provider.InvalidPayload
	}
	switch *code {
	case 200:
		return nil
	case 401, 403:
		return provider.Failure("UPSTREAM_UNAUTHORIZED")
	case 429:
		return provider.Failure("UPSTREAM_RATE_LIMITED")
	case 20001:
		return provider.UpstreamAccessRestricted
	default:
		return provider.UpstreamRejected
	}
}

func parseDetail(body []byte) (*PlaylistDetail, error) {
	var p struct {
		Code     *int            `json:"code"`
		Result   json.RawMessage `json:"result"`
		Playlist json.RawMessage `json:"playlist"`
	}
	if json.Unmarshal(body, &p) != nil || p.Code == nil {
		return nil, provider.InvalidPayload
	}
	// Read the business status before decoding success-only playlist fields.
	// 20001 is an explicit refusal; it does not establish why access failed.
	if err := classifyBusinessCode(p.Code); err != nil {
		return nil, err
	}
	raw := p.Playlist
	if len(raw) == 0 || string(raw) == "null" {
		raw = p.Result
	}
	var list *playlist
	if json.Unmarshal(raw, &list) != nil || list == nil || list.TrackCount == nil || list.Tracks == nil || *list.TrackCount < 0 || *list.TrackCount > maxTracks || len(*list.Tracks) > maxTracks {
		return nil, provider.InvalidPayload
	}
	if len(*list.Tracks) > *list.TrackCount {
		return nil, provider.IncompletePlaylist
	}
	detail := &PlaylistDetail{TrackCount: *list.TrackCount, TrackIDs: []string{}, Tracks: map[string]provider.Track{}}
	if list.TrackIDs != nil {
		if len(*list.TrackIDs) != detail.TrackCount {
			return nil, provider.IncompletePlaylist
		}
		for _, v := range *list.TrackIDs {
			if !provider.Decimal.MatchString(v.ID.String()) {
				return nil, provider.InvalidPayload
			}
			detail.TrackIDs = append(detail.TrackIDs, v.ID.String())
		}
	} else if detail.TrackCount != len(*list.Tracks) {
		return nil, provider.IncompletePlaylist
	}
	for _, s := range *list.Tracks {
		t, err := mapSong(s)
		if err != nil {
			return nil, err
		}
		if list.TrackIDs == nil {
			detail.TrackIDs = append(detail.TrackIDs, t.ExternalID)
		}
		if old, exists := detail.Tracks[t.ExternalID]; exists && list.TrackIDs != nil && !reflect.DeepEqual(old, t) {
			return nil, provider.InvalidPayload
		} else if !exists {
			detail.Tracks[t.ExternalID] = t
		}
	}
	allowed := map[string]bool{}
	for _, id := range detail.TrackIDs {
		allowed[id] = true
	}
	for id := range detail.Tracks {
		if !allowed[id] {
			return nil, provider.InvalidPayload
		}
	}
	return detail, nil
}

func mapSong(s song) (provider.Track, error) {
	id := s.ID.String()
	if !provider.Decimal.MatchString(id) {
		return provider.Track{}, provider.InvalidPayload
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
		return provider.Track{}, err
	}
	return t, nil
}

func uniqueIDs(ids []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func assembleComplete(detail *PlaylistDetail) ([]provider.Track, error) {
	tracks := []provider.Track{}
	for _, id := range uniqueIDs(detail.TrackIDs) {
		track, ok := detail.Tracks[id]
		if !ok {
			return nil, provider.IncompletePlaylist
		}
		tracks = append(tracks, track)
	}
	if err := provider.Validate(tracks, "netease"); err != nil {
		return nil, err
	}
	return tracks, nil
}

func (c *Client) fetchSongDetails(ctx context.Context, ids []string) (map[string]provider.Track, error) {
	requested := map[string]bool{}
	queryIDs := []songID{}
	for _, id := range ids {
		requested[id] = true
		queryIDs = append(queryIDs, songID{ID: json.Number(id)})
	}
	encoded, err := json.Marshal(queryIDs)
	if err != nil {
		return nil, provider.InvalidPayload
	}
	body, err := c.requestJSON(ctx, "/api/v3/song/detail", url.Values{"c": {string(encoded)}})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Code  *int            `json:"code"`
		Songs json.RawMessage `json:"songs"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return nil, provider.InvalidPayload
	}
	if err := classifyBusinessCode(envelope.Code); err != nil {
		return nil, err
	}
	var songs *[]song
	if json.Unmarshal(envelope.Songs, &songs) != nil || songs == nil || len(*songs) > detailBatchSize {
		return nil, provider.InvalidPayload
	}
	items := map[string]provider.Track{}
	for _, s := range *songs {
		track, err := mapSong(s)
		if err != nil || !requested[track.ExternalID] {
			return nil, provider.InvalidPayload
		}
		if old, exists := items[track.ExternalID]; exists && !reflect.DeepEqual(old, track) {
			return nil, provider.InvalidPayload
		}
		items[track.ExternalID] = track
	}
	if len(items) != len(requested) {
		provider.SyncLogger(ctx).Info("music song details incomplete", "expectedCount", len(requested), "actualCount", len(items))
		return nil, provider.IncompletePlaylist
	}
	return items, nil
}
