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
	var v struct {
		URL, Ext, Vcodec, Acodec string
		Duration                 float64
		HTTPHeaders              map[string]string `json:"http_headers"`
	}
	if json.Unmarshal(out.data, &v) != nil || v.Vcodec != "none" || v.Ext != "m4a" || !strings.HasPrefix(v.Acodec, "mp4a") || !playback.MediaURL(v.URL) {
		return playback.Audio{}, playback.Unsupported
	}
	var duration *int
	if v.Duration > 0 {
		d := int(v.Duration)
		duration = &d
	}
	return playback.Audio{URL: v.URL, MIME: "audio/mp4", Headers: v.HTTPHeaders, Duration: duration}, nil
}

var _ io.Writer = (*bounded)(nil)
