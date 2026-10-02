package playback

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"reflect"
	"strings"
	"time"
)

// Check every DNS result, dial the checked IP, and repeat on redirects. No environment proxy.
func PublicIP(ip net.IP) bool {
	a, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return false
	}
	for _, s := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "2001::/32", "2002::/16", "64:ff9b::/96"} {
		if netip.MustParsePrefix(s).Contains(a) {
			return false
		}
	}
	return true
}
func MediaURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.User == nil && u.Hostname() != "" && mediaPort(u.Hostname(), u.Port()) && u.Fragment == ""
}
func mediaPort(host, port string) bool {
	if port == "" || port == "443" {
		return true
	}
	host = strings.ToLower(host)
	return (port == "8082" || port == "4483") && (strings.HasSuffix(host, ".bilivideo.cn") || strings.HasSuffix(host, ".bilivideo.com"))
}

type idleConn struct{ net.Conn }

func (c idleConn) Read(p []byte) (int, error) {
	_ = c.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	return c.Conn.Read(p)
}
func (c idleConn) Write(p []byte) (int, error) {
	_ = c.Conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	return c.Conn.Write(p)
}
func MediaClient() *http.Client {
	tr := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 15 * time.Second, DisableCompression: true, DisableKeepAlives: true}
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil || !mediaPort(host, port) {
			return nil, Unsupported
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
			return nil, Unavailable
		}
		for _, ip := range ips {
			if !PublicIP(ip.IP) {
				return nil, Unavailable
			}
		}
		c, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ips[0].IP.String(), port))
		if err != nil {
			return nil, err
		}
		return idleConn{c}, nil
	}
	return &http.Client{Transport: tr, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 3 || !MediaURL(r.URL.String()) {
			return Unsupported
		}
		return nil
	}}
}

// Stream accepts no range requests and never writes JSON after committing audio.
func (s *Service) Stream(w http.ResponseWriter, r *http.Request, id, owner string, fail func(int, Failure)) {
	v, err := s.acquire(id, owner)
	if err != nil {
		if err == Conflict {
			fail(409, Conflict)
		} else if err == Busy {
			fail(429, Busy)
		} else if err == Expired {
			fail(410, Expired)
		} else if err == PublicNotAllowed || err == AccessRestricted {
			fail(403, PublicCode(Code(err)))
		} else {
			fail(503, PublicCode(Code(err)))
		}
		return
	}
	var result error
	started := false
	defer func() {
		s.release(v, result)
		// A graceful chunked EOF could be mistaken for a successfully ended song.
		// Abort the connection on late upstream/transcoder failures instead.
		if result != nil && started {
			panic(http.ErrAbortHandler)
		}
	}()
	ctx, cancel := context.WithTimeout(v.ctx, s.options.MaxStreamDuration)
	defer cancel()
	disconnect := context.AfterFunc(r.Context(), cancel)
	defer disconnect()
	if !MediaURL(v.audio.URL) {
		result = Unsupported
		fail(422, Unsupported)
		return
	}
	req, err := http.NewRequestWithContext(ctx, "GET", v.audio.URL, nil)
	if err != nil {
		result = Unsupported
		fail(422, Unsupported)
		return
	}
	for k, value := range v.audio.Headers {
		if strings.EqualFold(k, "User-Agent") || strings.EqualFold(k, "Referer") {
			req.Header.Set(k, value)
		}
	}
	client := s.client
	copyMediaClient := *client
	copyMediaClient.Jar = nil
	client = &copyMediaClient
	if v.audio.RedirectPolicy != nil {
		copyClient := *client
		copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if err := v.audio.RedirectPolicy(req, via); err != nil {
				return err
			}
			if len(via) > 3 || !MediaURL(req.URL.String()) {
				return Unsupported
			}
			return nil
		}
		client = &copyClient
	}
	first := time.AfterFunc(15*time.Second, cancel)
	defer first.Stop()
	resp, err := client.Do(req)
	if err != nil {
		first.Stop()
		result = Code(err)
		if ctx.Err() != nil {
			result = Timeout
		}
		fail(503, Code(result))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 || resp.StatusCode == 410 {
		// Exactly one refresh before any audio bytes; never change audience or kind.
		_ = resp.Body.Close()
		fresh, refreshErr := s.resolveAgain(ctx, v.Attribution)
		fresh = normalizeAudio(fresh, v.Attribution)
		fresh.Transcode = fresh.Transcode || s.options.ForceTranscode
		if refreshErr == nil {
			refreshErr = s.options.Policy.Check(v.Attribution.ID, fresh)
		}
		if refreshErr == nil && (!reflect.DeepEqual(fresh.Capability, v.audio.Capability) || fresh.SourceMode != v.audio.SourceMode || fresh.CredentialVersion != v.audio.CredentialVersion) {
			refreshErr = PublicNotAllowed
		}
		if refreshErr != nil {
			first.Stop()
			result = Code(refreshErr)
			fail(503, PublicCode(Code(refreshErr)))
			return
		}
		freshURL, parseErr := url.Parse(fresh.URL)
		if parseErr != nil || !MediaURL(fresh.URL) {
			first.Stop()
			result = Unsupported
			fail(422, Unsupported)
			return
		}
		retry := req.Clone(ctx)
		retry.URL = freshURL
		copyClient := *s.client
		copyClient.Jar = nil
		if fresh.RedirectPolicy != nil {
			copyClient.CheckRedirect = fresh.RedirectPolicy
		}
		resp, err = copyClient.Do(retry)
		if err != nil {
			first.Stop()
			result = Code(err)
			fail(503, PublicCode(Code(err)))
			return
		}
		defer resp.Body.Close()
		v.audio = fresh
		s.mu.Lock()
		s.metrics.ReResolutions++
		s.mu.Unlock()
	}
	if resp.StatusCode != 200 {
		first.Stop()
		result = Unavailable
		if resp.StatusCode == 429 {
			result = RateLimited
		}
		fail(503, Code(result))
		return
	}
	buf := make([]byte, 64*1024)
	n, err := io.ReadAtLeast(resp.Body, buf, 12)
	if err != nil {
		result = Unavailable
		if ctx.Err() != nil {
			result = Timeout
		}
		fail(503, Code(result))
		return
	}
	format := DetectFormat(buf[:n], resp.Header.Get("Content-Type"))
	if format == "" || (v.audio.InputFormat != "" && format != v.audio.InputFormat) {
		result = Unsupported
		fail(422, Unsupported)
		return
	}
	var reader io.Reader = resp.Body
	outputMIME := FormatMIME(format)
	var converted *transcodeStream
	if v.audio.Transcode {
		input := struct {
			io.Reader
			io.Closer
		}{io.MultiReader(bytes.NewReader(bytes.Clone(buf[:n])), resp.Body), resp.Body}
		converted, err = s.transcode(ctx, input, format)
		if err != nil {
			result = err
			status := 503
			if err == Busy {
				status = 429
			}
			fail(status, Code(err))
			return
		}
		defer converted.Close()
		n, err = io.ReadAtLeast(converted, buf, 12)
		if err != nil || DetectFormat(buf[:n], "audio/mpeg") != "mp3" {
			result = TranscodeFailed
			if ctx.Err() != nil {
				result = Timeout
			}
			fail(503, Code(result))
			return
		}
		reader = converted
		outputMIME = "audio/mpeg"
	} else if format != "mov" && format != "mp3" {
		result = Unsupported
		fail(422, Unsupported)
		return
	}
	first.Stop()
	if ctx.Err() != nil {
		result = Timeout
		fail(503, Timeout)
		return
	}
	w.Header().Set("Content-Type", outputMIME)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Accel-Buffering", "no")
	rc := http.NewResponseController(w)
	unblock := context.AfterFunc(ctx, func() { _ = rc.SetWriteDeadline(time.Now()) })
	defer unblock()
	write := func(b []byte) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e := rc.SetWriteDeadline(time.Now().Add(30 * time.Second)); e != nil {
			return e
		}
		if ctx.Err() != nil {
			_ = rc.SetWriteDeadline(time.Now())
			return ctx.Err()
		}
		if e := s.charge(ctx, v, len(b)); e != nil {
			return e
		}
		started = true
		written, e := w.Write(b)
		s.mu.Lock()
		s.metrics.OutputBytes += int64(written)
		if v.outputBytes == int64(len(b)) {
			s.metrics.FirstAudioSamples++
			s.metrics.FirstAudioMillis += time.Since(v.created).Milliseconds()
		}
		s.mu.Unlock()
		if e != nil {
			return e
		}
		return rc.Flush()
	}
	if err = write(buf[:n]); err != nil {
		result = Code(err)
		if !started {
			fail(503, PublicCode(Code(err)))
		}
		return
	}
	checkCtx, done := context.WithTimeout(ctx, 3*time.Second)
	s.record(checkCtx, v.Attribution.ID, v.audio, "")
	done()
	for {
		n, err = reader.Read(buf)
		if n > 0 {
			if e := write(buf[:n]); e != nil {
				result = Code(e)
				return
			}
		}
		if err == io.EOF {
			if converted != nil {
				result = converted.Wait()
			}
			return
		}
		if err != nil {
			result = Unavailable
			return
		}
		select {
		case <-ctx.Done():
			result = Timeout
			return
		default:
		}
	}
}
