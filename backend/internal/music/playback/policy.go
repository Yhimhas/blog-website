package playback

import (
	"blog-website/backend/internal/music/provider"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"time"
)

type Capability struct {
	MediaKind      string `json:"mediaKind"`
	TrackDuration  *int   `json:"trackDurationSeconds"`
	StreamDuration *int   `json:"streamDurationSeconds"`
	PreviewStart   *int   `json:"previewStartSeconds"`
	PreviewEnd     *int   `json:"previewEndSeconds"`
}

type Grant struct {
	TrackID    string    `json:"trackId"`
	SourceMode string    `json:"sourceMode"`
	Audience   string    `json:"audience"`
	Evidence   string    `json:"evidence"`
	Expires    time.Time `json:"expiresAt"`
}

// Only operator-controlled grants can expand public_only to account audio.
// Evidence is an administrative record, not an upstream entitlement assertion.
type Policy struct {
	Mode        string
	RequireFull bool
	Grants      []Grant
}

func LoadGrants(path string) ([]Grant, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("cannot read playback grants")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1024*1024 || (runtime.GOOS != "windows" && info.Mode().Perm()&0022 != 0) {
		return nil, errors.New("playback grants must be an operator-controlled regular file")
	}
	var grants []Grant
	d := json.NewDecoder(io.LimitReader(f, 1024*1024+1))
	d.DisallowUnknownFields()
	if d.Decode(&grants) != nil || d.Decode(new(any)) != io.EOF || grants == nil || len(grants) > 2000 {
		return nil, errors.New("invalid playback grants")
	}
	seen := map[string]bool{}
	for _, grant := range grants {
		key := grant.TrackID + ":" + grant.SourceMode
		if grant.TrackID == "" || len(grant.TrackID) > 150 || (grant.SourceMode != "public" && grant.SourceMode != "account") || grant.Audience != "public" || len(strings.TrimSpace(grant.Evidence)) < 1 || len(grant.Evidence) > 500 || grant.Expires.IsZero() || seen[key] {
			return nil, errors.New("invalid playback grant")
		}
		seen[key] = true
	}
	return grants, nil
}

func (p Policy) AllowsAccount(id string) bool {
	if p.Mode != "licensed" {
		return false
	}
	for _, grant := range p.Grants {
		if grant.TrackID == id && grant.SourceMode == "account" && grant.Audience == "public" && time.Now().Before(grant.Expires) {
			return true
		}
	}
	return false
}

func normalizeAudio(a Audio, track provider.Track) Audio {
	if a.SourceMode == "" {
		a.SourceMode = "public"
	}
	if a.Audience == "" {
		if a.SourceMode == "public" {
			a.Audience = "public"
		} else {
			a.Audience = "blocked"
		}
	}
	if a.Capability.MediaKind == "" {
		a.Capability.MediaKind = "unknown"
	}
	a.Capability.TrackDuration = track.DurationSeconds
	if a.Capability.StreamDuration == nil {
		a.Capability.StreamDuration = a.Duration
	}
	return a
}

func (p Policy) Check(id string, a Audio) error {
	if (a.SourceMode != "public" && a.SourceMode != "account") || a.Audience != "public" {
		return PublicNotAllowed
	}
	if a.SourceMode == "account" && !p.AllowsAccount(id) {
		return PublicNotAllowed
	}
	if a.SourceMode == "public" && p.Grants != nil {
		allowed := false
		for _, grant := range p.Grants {
			if grant.TrackID == id && grant.SourceMode == "public" && grant.Audience == "public" && time.Now().Before(grant.Expires) {
				allowed = true
				break
			}
		}
		if !allowed {
			return PublicNotAllowed
		}
	}
	c := a.Capability
	if c.MediaKind != "full" && c.MediaKind != "preview" && c.MediaKind != "unknown" {
		return Unsupported
	}
	if c.MediaKind == "full" && a.CompletenessEvidence == "" {
		return Unsupported
	}
	for _, value := range []*int{c.TrackDuration, c.StreamDuration, c.PreviewStart, c.PreviewEnd} {
		if value != nil && (*value < 0 || *value > 7*24*3600) {
			return Unsupported
		}
	}
	if c.MediaKind == "preview" && (c.PreviewStart == nil || c.PreviewEnd == nil || *c.PreviewEnd <= *c.PreviewStart) {
		return Unsupported
	}
	if p.RequireFull && c.MediaKind != "full" {
		return PublicNotAllowed
	}
	if a.Valid != nil && !a.Valid() {
		return SourceAuthExpired
	}
	return nil
}

// An account request is never attempted for an unlicensed track. All fallback is bounded.
type FallbackResolver struct {
	Public  Resolver
	Account Resolver
	Policy  Policy
}

func (r FallbackResolver) Resolve(ctx context.Context, track provider.Track) (Audio, error) {
	a, err := r.Public.Resolve(ctx, track)
	if err == nil {
		a = normalizeAudio(a, track)
		err = r.Policy.Check(track.ID, a)
		if err == nil {
			return a, nil
		}
	}
	if r.Account == nil || !r.Policy.AllowsAccount(track.ID) {
		return Audio{}, err
	}
	a, err = r.Account.Resolve(ctx, track)
	a.SourceMode = "account"
	if err == nil {
		a.Audience = "public"
	} else {
		a.Audience = "blocked"
	}
	return a, err
}

type PlaybackCheck struct {
	SourceMode, Audience, MediaKind, CredentialVersion string
	Error                                              Failure
}
type ScopedStore interface {
	RecordPlayback(context.Context, string, PlaybackCheck)
}

func PublicCode(code Failure) Failure {
	if code == SourceAuthExpired {
		return Unavailable
	}
	return code
}
