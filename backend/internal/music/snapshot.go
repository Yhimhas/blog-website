package music

import (
	"blog-website/backend/internal/music/provider"
	"encoding/json"
	"io"
	"net/url"
	"strings"
)

// DecodeBilibiliSnapshot validates the entire legacy snapshot before any DB writes.
func DecodeBilibiliSnapshot(r io.Reader) (Source, []provider.Track, error) {
	var v struct {
		ID, Platform, Title, URL string
		Tracks                   []struct{ ID, Title, Artist, URL, EmbedURL string }
	}
	d := json.NewDecoder(io.LimitReader(r, 4*1024*1024+1))
	if d.Decode(&v) != nil || d.Decode(new(any)) != io.EOF || v.Platform != "bilibili" || len(v.Tracks) == 0 || !provider.SourceURL(v.URL, v.Platform) {
		return Source{}, nil, provider.InvalidPayload
	}
	external := strings.TrimPrefix(v.ID, "bilibili:")
	u, _ := url.Parse(v.URL)
	if !provider.Decimal.MatchString(external) || u.Query().Get("fid") != external {
		return Source{}, nil, provider.InvalidSource
	}
	out := make([]provider.Track, 0, len(v.Tracks))
	for _, t := range v.Tracks {
		bv := strings.TrimPrefix(t.ID, "bilibili:")
		part := "1"
		if !provider.BVID.MatchString(bv) || t.URL != "https://www.bilibili.com/video/"+bv+"/" {
			return Source{}, nil, provider.InvalidPayload
		}
		out = append(out, provider.Track{ID: "bilibili:" + bv + ":1", Provider: "bilibili", ExternalID: bv, PartID: &part, Title: t.Title, Author: &t.Artist, SourceURL: t.URL, EmbedURL: &t.EmbedURL, Availability: "unknown"})
	}
	if err := provider.Validate(out, "bilibili"); err != nil {
		return Source{}, nil, err
	}
	return Source{ID: v.ID, Provider: v.Platform, ExternalID: external, Title: v.Title, SourceURL: v.URL}, out, nil
}
