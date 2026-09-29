package bilibili

import (
	"blog-website/backend/internal/music/playback"
	"blog-website/backend/internal/music/provider"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
)

type Resolver struct{ Binary string }
type bounded struct {
	data     []byte
	max      int
	overflow bool
}

func (b *bounded) Write(p []byte) (int, error) {
	n := len(p)
	left := b.max - len(b.data)
	if len(p) > left {
		b.overflow = true
		p = p[:left]
	}
	b.data = append(b.data, p...)
	return n, nil
}
func (r Resolver) Resolve(ctx context.Context, t provider.Track) (playback.Audio, error) {
	if t.Provider != "bilibili" || !provider.BVID.MatchString(t.ExternalID) || t.PartID == nil || *t.PartID != "1" {
		return playback.Audio{}, playback.Unsupported
	}
	cmd := exec.CommandContext(ctx, r.Binary, "--ignore-config", "--no-cache-dir", "--no-playlist", "--skip-download", "--socket-timeout", "10", "--retries", "0", "--extractor-retries", "0", "-f", "bestaudio", "-J", "--", "https://www.bilibili.com/video/"+t.ExternalID+"/?p=1")
	configureProcess(cmd)
	out := &bounded{max: 1024 * 1024}
	stderr := &bounded{max: 8192}
	cmd.Stdout = out
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return playback.Audio{}, playback.Timeout
		}
		if strings.Contains(string(stderr.data), "429") {
			return playback.Audio{}, playback.RateLimited
		}
		return playback.Audio{}, playback.Unavailable
	}
	if out.overflow {
		return playback.Audio{}, playback.Unsupported
	}
	return parseAudio(out.data)
}
func parseAudio(data []byte) (playback.Audio, error) {
	var v struct {
		URL, Ext, Vcodec, Acodec string
		Duration                 float64
		HTTPHeaders              map[string]string `json:"http_headers"`
	}
	if json.Unmarshal(data, &v) != nil || v.Vcodec != "none" || v.Acodec == "none" || v.Acodec == "" || !playback.MediaURL(v.URL) {
		return playback.Audio{}, playback.Unsupported
	}
	format := ""
	switch v.Ext {
	case "m4a", "mp4":
		format = "mov"
	case "mp3":
		format = "mp3"
	case "ogg", "opus":
		format = "ogg"
	case "flac":
		format = "flac"
	case "wav":
		format = "wav"
	case "webm":
		format = "matroska"
	case "aac":
		format = "aac"
	}
	if format == "" {
		return playback.Audio{}, playback.Unsupported
	}
	transcode := format != "mp3" && !(format == "mov" && strings.HasPrefix(v.Acodec, "mp4a"))
	var duration *int
	if v.Duration > 0 {
		d := int(v.Duration)
		duration = &d
	}
	return playback.Audio{URL: v.URL, MIME: playback.FormatMIME(format), InputFormat: format, Transcode: transcode, Headers: v.HTTPHeaders, Duration: duration}, nil
}

var _ io.Writer = (*bounded)(nil)
