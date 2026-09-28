package playback

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
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
		} else {
			fail(410, Expired)
		}
		return
	}
	var result error
	defer func() { s.release(v, result) }()
	ctx, cancel := context.WithTimeout(v.ctx, 6*time.Hour)
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
	first := time.AfterFunc(15*time.Second, cancel)
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
	first.Stop()
	ct := strings.ToLower(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	// Bilibili returns octet-stream for fMP4; validate the ftyp box before accepting it.
	if err != nil {
		result = Unavailable
		if ctx.Err() != nil {
			result = Timeout
		}
		fail(503, Code(result))
		return
	}
	if n < 12 || string(buf[4:8]) != "ftyp" || (ct != "application/octet-stream" && ct != "audio/mp4" && ct != "video/mp4") {
		result = Unsupported
		fail(422, Unsupported)
		return
	}
	checkCtx, done := context.WithTimeout(ctx, 3*time.Second)
	s.store.Checked(checkCtx, v.Attribution.ID, "")
	done()
	w.Header().Set("Content-Type", v.audio.MIME)
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
		if _, e := w.Write(b); e != nil {
			return e
		}
		return rc.Flush()
	}
	if err = write(buf[:n]); err != nil {
		result = Unavailable
		return
	}
	for {
		n, err = resp.Body.Read(buf)
		if n > 0 {
			if e := write(buf[:n]); e != nil {
				result = Unavailable
				return
			}
		}
		if err == io.EOF {
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
