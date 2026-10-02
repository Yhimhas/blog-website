package playback

import (
	"blog-website/backend/internal/auth"
	"context"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

// FFmpeg receives bytes only, with pipe as its sole allowed protocol. The
// fixed demuxer prevents auto-detecting playlists referencing files or URLs.
func ffmpegArgs(format string) ([]string, error) {
	switch format {
	case "mov", "mp3", "aac", "ogg", "flac", "wav", "matroska":
	default:
		return nil, Unsupported
	}
	return []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-xerror", "-max_alloc", "16777216", "-protocol_whitelist", "pipe", "-probesize", "524288", "-analyzeduration", "5000000", "-threads", "1", "-f", format, "-i", "pipe:0", "-map", "0:a:0", "-vn", "-sn", "-dn", "-map_metadata", "-1", "-threads", "1", "-filter_threads", "1", "-c:a", "libmp3lame", "-b:a", "128k", "-ar", "44100", "-ac", "2", "-f", "mp3", "-write_xing", "0", "-id3v2_version", "0", "-flush_packets", "1", "pipe:1"}, nil
}

type boundedLog struct{ data []byte }

func (b *boundedLog) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 8192 - len(b.data)
	if len(p) > remaining {
		p = p[:remaining]
	}
	b.data = append(b.data, p...)
	return n, nil
}

type transcodeStream struct {
	*os.File
	cancel context.CancelFunc
	input  io.ReadCloser
	done   chan struct{}
	err    error
	once   sync.Once
}

func (t *transcodeStream) Wait() error { <-t.done; return t.err }
func (t *transcodeStream) Close() error {
	t.once.Do(func() { t.cancel(); _ = t.input.Close(); _ = t.File.Close(); <-t.done })
	return nil
}
func (s *Service) transcode(ctx context.Context, input io.ReadCloser, format string, offsets ...float64) (*transcodeStream, error) {
	args, err := ffmpegArgs(format)
	if err != nil {
		return nil, err
	}
	if len(offsets) > 0 && offsets[0] > 0 {
		// Output seeking decodes and discards the prefix; input is a non-seekable
		// pipe, so input -ss or byte Range would be incorrect.
		args = append(args[:len(args)-1], "-ss", strconv.FormatFloat(offsets[0], 'f', 3, 64), "pipe:1")
	}
	if s.options.FFmpeg == "" {
		return nil, TranscodeUnavailable
	}
	select {
	case s.transcoders <- struct{}{}:
	default:
		return nil, Busy
	}
	release := func() { <-s.transcoders }
	leaseID := auth.Token()
	if s.options.Control != nil {
		if err := s.options.Control.Lease(ctx, "transcoding", leaseID, "", s.options.MaxTranscoders, s.options.MaxStreamDuration+time.Minute); err != nil {
			release()
			return nil, Busy
		}
		localRelease := release
		release = func() { localRelease(); s.releaseLease("transcoding", leaseID) }
	}
	ctx, cancel := context.WithCancel(ctx)
	output, writer, err := os.Pipe()
	if err != nil {
		cancel()
		release()
		return nil, TranscodeUnavailable
	}
	cmd := exec.CommandContext(ctx, s.options.FFmpeg, args...)
	configureFFmpeg(cmd)
	cmd.Stdin = input
	cmd.Stdout = writer
	cmd.Stderr = &boundedLog{}
	if err := cmd.Start(); err != nil {
		writer.Close()
		output.Close()
		cancel()
		release()
		return nil, TranscodeUnavailable
	}
	_ = writer.Close()
	stream := &transcodeStream{File: output, cancel: cancel, input: input, done: make(chan struct{})}
	unblock := context.AfterFunc(ctx, func() { _ = input.Close() })
	go func() {
		err := cmd.Wait()
		unblock()
		_ = input.Close()
		if err != nil {
			stream.err = TranscodeFailed
			if ctx.Err() != nil {
				stream.err = Timeout
			}
		}
		cancel()
		release()
		close(stream.done)
	}()
	return stream, nil
}
