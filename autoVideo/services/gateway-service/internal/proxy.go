package internal

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

// newReverseProxy creates an httputil.ReverseProxy that forwards to target,
// optionally stripping stripPrefix from the request path before proxying.
func newReverseProxy(target *url.URL, stripPrefix string, timeout time.Duration, logger *zap.Logger) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host

		// Strip prefix if configured (e.g. /media → MinIO root)
		if stripPrefix != "" {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, stripPrefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
			req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, stripPrefix)
		}

		// Standard proxy headers
		req.Header.Set("X-Real-IP", clientIP(req))
		req.Header.Del("X-Forwarded-For")
		req.Header.Add("X-Forwarded-For", clientIP(req))
		req.Header.Set("X-Forwarded-Proto", scheme(req))

		// Downstream services must not attempt to re-validate the upstream Host
		req.Host = target.Host
	}

	transport := &http.Transport{
		ResponseHeaderTimeout: timeout,
		// Reuse connections across requests
		MaxIdleConnsPerHost: 32,
	}

	rp := &httputil.ReverseProxy{
		Director:  director,
		Transport: transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			logger.Warn("proxy error",
				zap.String("path", r.URL.Path),
				zap.String("upstream", target.Host),
				zap.Error(err),
			)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprintf(w, `{"code":502,"message":"upstream unavailable"}`)
		},
	}
	return rp
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.SplitN(fwd, ",", 2)[0]
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return strings.Trim(ip, "[]")
}

func scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
