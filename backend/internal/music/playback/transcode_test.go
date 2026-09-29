package playback

import (
	"blog-website/backend/internal/music/provider"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func waveSample() []byte {
	const samples = 44100
	b := make([]byte, 44+samples*2)
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], 44100)
	binary.LittleEndian.PutUint32(b[28:], 88200)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], samples*2)
	for i := 0; i < samples; i++ {
		binary.LittleEndian.PutUint16(b[44+i*2:], uint16(int16(8000*math.Sin(2*math.Pi*440*float64(i)/44100))))
	}
	return b
}
func ffmpegBinary(t *testing.T) string {
	t.Helper()
	p, e := exec.LookPath("ffmpeg")
	if e != nil {
		t.Skip("FFmpeg not installed; real transcode test not executed")
	}
	return p
}
func TestFFmpegRealWaveToMP3(t *testing.T) {
	binary := ffmpegBinary(t)
	s := New(context.Background(), fakeResolver{}, fakeStore{}, Options{FFmpeg: binary})
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	worker, err := s.transcode(ctx, io.NopCloser(bytes.NewReader(waveSample())), "wav")
	if err != nil {
		t.Fatal(err)
	}
	defer worker.Close()
	mp3, err := io.ReadAll(io.LimitReader(worker, 100000))
	if err != nil || worker.Wait() != nil {
		t.Fatal("transcode failed", err)
	}
	if DetectFormat(mp3, "audio/mpeg") != "mp3" {
		t.Fatal("output is not MP3")
	}
	decoder := exec.CommandContext(ctx, binary, "-hide_banner", "-loglevel", "error", "-protocol_whitelist", "pipe", "-f", "mp3", "-i", "pipe:0", "-f", "s16le", "-ac", "1", "-ar", "8000", "pipe:1")
	decoder.Stdin = bytes.NewReader(mp3)
	pcm, err := decoder.Output()
	if err != nil || len(pcm) < 15000 {
		t.Fatal("MP3 not decodable", err, len(pcm))
	}
	if bytes.Count(pcm, []byte{0}) == len(pcm) {
		t.Fatal("silent output")
	}
	t.Log("real FFmpeg produced", len(mp3), "MP3 bytes and", len(pcm), "decoded PCM bytes")
}
func TestFFmpegCancellationAndCapacity(t *testing.T) {
	s := New(context.Background(), fakeResolver{}, fakeStore{}, Options{FFmpeg: ffmpegBinary(t)})
	defer s.Close()
	var workers []*transcodeStream
	for i := 0; i < 2; i++ {
		r, w := io.Pipe()
		defer w.Close()
		worker, err := s.transcode(context.Background(), r, "wav")
		if err != nil {
			t.Fatal(err)
		}
		workers = append(workers, worker)
		defer worker.Close()
	}
	if _, err := s.transcode(context.Background(), io.NopCloser(strings.NewReader("")), "wav"); err != Busy {
		t.Fatal("third transcode admitted", err)
	}
	start := time.Now()
	for _, w := range workers {
		w.Close()
	}
	if time.Since(start) > 5*time.Second || len(s.transcoders) != 0 {
		t.Fatal("worker cancellation or quota leaked")
	}
}
func TestFFmpegFailureAndFormatBoundary(t *testing.T) {
	for _, format := range []string{"hls", "concat", "http", "file", "mov -i evil"} {
		if _, err := ffmpegArgs(format); err != Unsupported {
			t.Fatal("unbounded demuxer", format)
		}
	}
	s := New(context.Background(), fakeResolver{}, fakeStore{}, Options{FFmpeg: ffmpegBinary(t)})
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	w, e := s.transcode(ctx, io.NopCloser(strings.NewReader("not a wav")), "wav")
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	b, _ := io.ReadAll(w)
	if len(b) != 0 || w.Wait() != TranscodeFailed {
		t.Fatal("invalid input reported success")
	}
}

type waveResolver struct{}

func (waveResolver) Resolve(context.Context, provider.Track) (Audio, error) {
	return Audio{URL: "https://example.com/audio", MIME: "audio/wav", InputFormat: "wav", Transcode: true}, nil
}
func TestFFmpegHTTPStream(t *testing.T) {
	s := New(context.Background(), waveResolver{}, fakeStore{}, Options{FFmpeg: ffmpegBinary(t)})
	defer s.Close()
	s.client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"audio/wav"}}, Body: io.NopCloser(bytes.NewReader(waveSample()))}, nil
	})}
	v, e := s.Create(context.Background(), "a", "a", "t")
	if e != nil {
		t.Fatal(e)
	}
	ready(t, s, v.ID, "a")
	v, _ = s.Get(v.ID, "a")
	if v.MIME != "audio/mpeg" {
		t.Fatal("ready MIME disagrees with transcoder")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Stream(w, r, v.ID, "a", func(code int, f Failure) { http.Error(w, string(f), code) })
	}))
	defer srv.Close()
	r, e := http.Get(srv.URL)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	b, e := io.ReadAll(r.Body)
	if e != nil || r.StatusCode != 200 || r.Header.Get("Content-Type") != "audio/mpeg" || DetectFormat(b, "audio/mpeg") != "mp3" {
		t.Fatal("stream failed", r.Status, e)
	}
}

func TestFFmpegStreamingContainers(t *testing.T) {
	binary := ffmpegBinary(t)
	for _, tc := range []struct {
		format string
		args   []string
	}{
		{"flac", []string{"-c:a", "flac", "-f", "flac"}},
		{"ogg", []string{"-c:a", "libvorbis", "-f", "ogg"}},
		{"matroska", []string{"-c:a", "libopus", "-f", "webm"}},
		{"aac", []string{"-c:a", "aac", "-f", "adts"}},
		{"mov", []string{"-c:a", "aac", "-movflags", "frag_keyframe+empty_moov", "-f", "mp4"}},
	} {
		t.Run(tc.format, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			args := append([]string{"-hide_banner", "-loglevel", "error", "-f", "wav", "-i", "pipe:0"}, tc.args...)
			args = append(args, "pipe:1")
			encode := exec.CommandContext(ctx, binary, args...)
			encode.Stdin = bytes.NewReader(waveSample())
			fixture, err := encode.Output()
			if err != nil {
				t.Fatal("fixture encoder unavailable", err)
			}
			if DetectFormat(fixture, "application/octet-stream") != tc.format {
				t.Fatal("signature does not match", tc.format)
			}
			s := New(ctx, fakeResolver{}, fakeStore{}, Options{FFmpeg: binary})
			defer s.Close()
			worker, err := s.transcode(ctx, io.NopCloser(bytes.NewReader(fixture)), tc.format)
			if err != nil {
				t.Fatal(err)
			}
			defer worker.Close()
			mp3, err := io.ReadAll(worker)
			if err != nil || worker.Wait() != nil || DetectFormat(mp3, "audio/mpeg") != "mp3" {
				t.Fatal("pipe conversion failed", err, worker.err)
			}
		})
	}
}
