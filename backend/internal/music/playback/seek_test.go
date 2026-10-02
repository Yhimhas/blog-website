package playback

import (
	"bytes"
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"
)

func TestSeekBoundsAndCapability(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cap    Capability
		ffmpeg string
		offset float64
		want   Failure
		mode   string
	}{
		{"preview inside", Capability{MediaKind: "preview", PreviewStart: intPointer(60), PreviewEnd: intPointer(90)}, "configured", 29.9, "", "restart"},
		{"preview outside", Capability{MediaKind: "preview", PreviewStart: intPointer(60), PreviewEnd: intPointer(90)}, "configured", 30, InvalidSeek, ""},
		{"preview stream shorter", Capability{MediaKind: "preview", PreviewStart: intPointer(60), PreviewEnd: intPointer(90), StreamDuration: intPointer(10)}, "configured", 11, InvalidSeek, ""},
		{"no duration", Capability{MediaKind: "unknown"}, "configured", 1, InvalidSeek, ""},
		{"no ffmpeg", Capability{MediaKind: "unknown", StreamDuration: intPointer(30)}, "", 1, InvalidSeek, ""},
		{"unknown stays unknown", Capability{MediaKind: "unknown", StreamDuration: intPointer(30)}, "configured", 10, "", "restart"},
		{"ordinary no ffmpeg", Capability{MediaKind: "unknown", StreamDuration: intPointer(30)}, "", 0, "", "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(context.Background(), audioResolver{audio: Audio{URL: "https://example.com/audio", Capability: tc.cap}}, fakeStore{}, Options{FFmpeg: tc.ffmpeg})
			defer s.Close()
			v, e := s.Create(context.Background(), "owner", "ip", "track", tc.offset)
			if e != nil {
				t.Fatal(e)
			}
			deadline := time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				v, e = s.Get(v.ID, "owner")
				if e != nil {
					t.Fatal(e)
				}
				if v.Status != "preparing" {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if tc.want != "" {
				if v.Status != "failed" || v.Error != tc.want {
					t.Fatal(v)
				}
				return
			}
			if v.Status != "ready" || v.Seek != tc.mode || v.StartSeconds != tc.offset || v.Capability.MediaKind != tc.cap.MediaKind {
				t.Fatal(v)
			}
			if _, err := s.acquire(v.ID, "wrong-owner"); err != Expired {
				t.Fatal("ownership bypass", err)
			}
		})
	}
	s := New(context.Background(), fakeResolver{}, fakeStore{})
	defer s.Close()
	for _, v := range []float64{-1, math.NaN(), math.Inf(1), 604801} {
		if _, err := s.Create(context.Background(), "owner", "ip", "track", v); err != InvalidSeek {
			t.Fatal(v, err)
		}
	}
}

func TestSeekHTTPDecodesOnlyRequestedSuffix(t *testing.T) {
	binary := ffmpegBinary(t)
	s := New(context.Background(), audioResolver{audio: Audio{URL: "https://example.com/audio", InputFormat: "wav", Capability: Capability{MediaKind: "unknown", StreamDuration: intPointer(1)}}}, fakeStore{}, Options{FFmpeg: binary})
	defer s.Close()
	sample := waveSample()
	// First half is silent, second half is a tone. Seeking must remove the silence.
	clear(sample[44 : 44+44100])
	s.client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"audio/wav"}}, Body: io.NopCloser(bytes.NewReader(sample))}, nil
	})}
	v, err := s.Create(context.Background(), "owner", "ip", "track", .5)
	if err != nil {
		t.Fatal(err)
	}
	ready(t, s, v.ID, "owner")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Stream(w, r, v.ID, "owner", func(code int, f Failure) { http.Error(w, string(f), code) })
	}))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	mp3, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != 200 || resp.Header.Get("Content-Range") != "" {
		t.Fatal(resp.Status, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	decoder := exec.CommandContext(ctx, binary, "-hide_banner", "-loglevel", "error", "-f", "mp3", "-i", "pipe:0", "-f", "s16le", "-ac", "1", "-ar", "8000", "pipe:1")
	decoder.Stdin = bytes.NewReader(mp3)
	pcm, err := decoder.Output()
	if err != nil || len(pcm) < 7500 || len(pcm) > 10000 {
		t.Fatal("expected half-second suffix", len(pcm), err)
	}
	if bytes.Count(pcm[2000:6000], []byte{0}) > 3000 {
		t.Fatal("seek retained silent prefix")
	}
	t.Logf("offset 0.5s: %d MP3 bytes -> %d PCM bytes", len(mp3), len(pcm))
}
