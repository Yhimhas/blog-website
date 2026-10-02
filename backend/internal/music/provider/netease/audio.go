package netease

import (
	"blog-website/backend/internal/music/playback"
	"blog-website/backend/internal/music/provider"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// AudioResolver uses the official public external-player endpoint only. It does
// not use login cookies, premium APIs, decryption, or third-party audio mirrors.
type AudioResolver struct{ HTTP *http.Client }

// The public outer-player endpoint expects ordinary web playback headers.
// No login, device identity, user cookies or premium entitlement is supplied.
const audioUserAgent = "Mozilla/5.0"
const audioReferer = "https://music.163.com/"

func NewAudioResolver() *AudioResolver { return &AudioResolver{HTTP: playback.MediaClient()} }
func officialAudioHost(host string) bool {
	return host == "music.163.com" || strings.HasSuffix(host, ".music.126.net") || strings.HasSuffix(host, ".music.163.com")
}
func audioRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > 3 || !officialAudioHost(req.URL.Hostname()) {
		return playback.Unsupported
	}
	// Some official outer links return an HTTP CDN location. Upgrade before any
	// connection; plaintext requests and TLS downgrades are never sent.
	if req.URL.Scheme == "http" && req.URL.Port() == "" {
		req.URL.Scheme = "https"
	}
	if !playback.MediaURL(req.URL.String()) {
		return playback.Unsupported
	}
	return nil
}
func (r *AudioResolver) Resolve(ctx context.Context, t provider.Track) (playback.Audio, error) {
	if t.Provider != "netease" || !provider.Decimal.MatchString(t.ExternalID) || t.ID != "netease:"+t.ExternalID || t.PartID != nil {
		return playback.Audio{}, playback.Unsupported
	}
	client := r.HTTP
	if client == nil {
		client = playback.MediaClient()
	}
	copyClient := *client
	copyClient.CheckRedirect = audioRedirect
	copyClient.Jar = nil
	req, err := http.NewRequestWithContext(ctx, "GET", "https://music.163.com/song/media/outer/url?id="+url.QueryEscape(t.ExternalID)+".mp3", nil)
	if err != nil {
		return playback.Audio{}, playback.Unsupported
	}
	req.Header.Set("User-Agent", audioUserAgent)
	req.Header.Set("Referer", audioReferer)
	resp, err := copyClient.Do(req)
	if err != nil {
		return playback.Audio{}, playback.Code(err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 429:
		return playback.Audio{}, playback.RateLimited
	case 404, 410:
		return playback.Audio{}, playback.SourceUnavailable
	}
	if resp.StatusCode != 200 {
		return playback.Audio{}, playback.Unavailable
	}
	// Read only a prefix, never the entire song. Successful metadata is not yet a
	// playback check; the streaming handler verifies the subsequent request again.
	var prefix [512]byte
	n, err := io.ReadAtLeast(resp.Body, prefix[:], 12)
	if err != nil {
		return playback.Audio{}, playback.Unavailable
	}
	format := playback.DetectFormat(prefix[:n], resp.Header.Get("Content-Type"))
	if format == "" {
		if strings.Contains(resp.Header.Get("Content-Type"), "json") {
			var result struct {
				Code int `json:"code"`
			}
			_ = json.Unmarshal(prefix[:n], &result)
			if result.Code == 429 {
				return playback.Audio{}, playback.RateLimited
			}
			// Risk-control/login/geographic refusals (-460/20001/403) are transient;
			// never permanently mark the whole collection unavailable.
		}
		return playback.Audio{}, playback.Unavailable
	}
	if resp.Request == nil || !officialAudioHost(resp.Request.URL.Hostname()) || !playback.MediaURL(resp.Request.URL.String()) {
		return playback.Audio{}, playback.Unsupported
	}
	return playback.Audio{URL: resp.Request.URL.String(), MIME: playback.FormatMIME(format), InputFormat: format, Transcode: format != "mp3", RedirectPolicy: audioRedirect, Capability: playback.Capability{MediaKind: "unknown", TrackDuration: t.DurationSeconds}, Headers: map[string]string{"User-Agent": audioUserAgent, "Referer": audioReferer}}, nil
}
