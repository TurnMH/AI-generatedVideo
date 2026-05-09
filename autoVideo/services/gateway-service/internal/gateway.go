package internal

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// Gateway is the central HTTP handler: it matches routes, validates JWT,
// applies CORS, and proxies requests to the appropriate upstream.
type Gateway struct {
	routes         []route
	jwtSecret      []byte
	allowedOrigins []string
	logger         *zap.Logger

	// pre-built reverse proxies keyed by upstream host:path
	proxies map[string]*proxyEntry

	// registry enables dynamic service discovery; cfg provides fallback upstreams
	registry   *Registry
	cfg        *Config
	dynMu      sync.RWMutex
	dynProxies map[string]*proxyEntry // keyed by "host|stripPrefix|timeout" for dynamically resolved addresses
}

type proxyEntry struct {
	rp          http.Handler
	upstream    *url.URL
	timeout     time.Duration
	stripPrefix string
	websocket   bool
	public      bool
}

// NewGateway constructs a Gateway from the parsed configuration.
func NewGateway(cfg *Config, logger *zap.Logger, registry *Registry) (*Gateway, error) {
	ups, err := parseUpstreams(cfg.Upstreams)
	if err != nil {
		return nil, err
	}
	routes, err := buildRoutes(cfg.Routes, ups)
	if err != nil {
		return nil, err
	}

	// Build one ReverseProxy per unique (upstream, stripPrefix, timeout) combination.
	proxies := make(map[string]*proxyEntry)
	for _, r := range routes {
		key := proxyKey(r)
		if _, exists := proxies[key]; !exists {
			rp := newReverseProxy(r.upstream, r.stripPrefix, r.timeout, logger)
			proxies[key] = &proxyEntry{
				rp:          rp,
				upstream:    r.upstream,
				timeout:     r.timeout,
				stripPrefix: r.stripPrefix,
				websocket:   r.websocket,
				public:      r.public,
			}
		}
	}

	return &Gateway{
		routes:         routes,
		jwtSecret:      []byte(cfg.JWT.Secret),
		allowedOrigins: cfg.CORS.AllowedOrigins,
		logger:         logger,
		proxies:        proxies,
		registry:       registry,
		cfg:            cfg,
		dynProxies:     make(map[string]*proxyEntry),
	}, nil
}

// ServeHTTP satisfies http.Handler — this is the single entry point for all traffic.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for all responses when origin is allowed.
	origin := r.Header.Get("Origin")
	if origin != "" && g.originAllowed(origin) {
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, X-User-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Built-in health check (no auth, no routing).
	if r.URL.Path == "/healthz" || r.URL.Path == "/health" {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"gateway"}`))
		return
	}

	// Route matching (first match wins, same semantics as nginx).
	rt := g.matchRoute(r.URL.Path)
	if rt == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, `{"code":404,"message":"no route matched"}`)
		return
	}

	// JWT authentication (skipped for public routes).
	if !rt.public {
		userID, role, err := g.validateJWT(r)
		if err != nil {
			g.logger.Debug("auth rejected", zap.String("path", r.URL.Path), zap.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":401,"message":"unauthorized"}`))
			return
		}
		// Attach resolved identity headers so downstream services don't need to re-parse JWT.
		r = r.Clone(r.Context())
		r.Header.Set("X-User-ID", fmt.Sprintf("%d", userID))
		r.Header.Set("X-User-Role", role)
	}

	// WebSocket upgrade path.
	if rt.websocket || isWebSocketUpgrade(r) {
		proxyWebSocket(w, r, g.resolveUpstreamURL(rt), rt.timeout, g.logger)
		return
	}

	// Standard HTTP reverse proxy.
	proxy := g.getProxy(rt)
	proxy.rp.ServeHTTP(w, r)
}

// resolveUpstream returns the upstream address for name: registry first, then config fallback.
func (g *Gateway) resolveUpstream(name string) string {
	if addr, ok := g.registry.Resolve(name); ok {
		return addr
	}
	return g.cfg.Upstreams[name]
}

// resolveUpstreamURL resolves the current upstream URL for a route.
func (g *Gateway) resolveUpstreamURL(rt *route) *url.URL {
	addr := g.resolveUpstream(rt.upstreamName)
	if addr == "" {
		return rt.upstream
	}
	u, err := url.Parse(addr)
	if err != nil {
		return rt.upstream
	}
	return u
}

// getProxy returns the reverse proxy for a route, using a dynamically resolved upstream if available.
func (g *Gateway) getProxy(rt *route) *proxyEntry {
	addr := g.resolveUpstream(rt.upstreamName)
	if addr == "" {
		return g.proxies[proxyKey(*rt)]
	}
	u, err := url.Parse(addr)
	if err != nil {
		return g.proxies[proxyKey(*rt)]
	}
	// Build a key in the same format as proxyKey so static and dynamic caches are consistent.
	key := fmt.Sprintf("%s|%s|%s", u.Host, rt.stripPrefix, rt.timeout)

	// Fast path: config-based proxy (host matches).
	if p, ok := g.proxies[key]; ok {
		return p
	}

	// Check dynamic proxy cache.
	g.dynMu.RLock()
	if p, ok := g.dynProxies[key]; ok {
		g.dynMu.RUnlock()
		return p
	}
	g.dynMu.RUnlock()

	// Build and cache a new proxy for the dynamically resolved address.
	rp := newReverseProxy(u, rt.stripPrefix, rt.timeout, g.logger)
	p := &proxyEntry{
		rp:          rp,
		upstream:    u,
		timeout:     rt.timeout,
		stripPrefix: rt.stripPrefix,
		websocket:   rt.websocket,
		public:      rt.public,
	}
	g.dynMu.Lock()
	g.dynProxies[key] = p
	g.dynMu.Unlock()
	return p
}

// matchRoute finds the first route whose pattern or prefix matches path.
func (g *Gateway) matchRoute(path string) *route {
	for i := range g.routes {
		if g.routes[i].match(path) {
			return &g.routes[i]
		}
	}
	return nil
}

// validateJWT extracts and validates the Bearer token from Authorization header.
// Returns (userID, role, error). Only supports HMAC-signed tokens.
func (g *Gateway) validateJWT(r *http.Request) (uint64, string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return 0, "", fmt.Errorf("missing authorization header")
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return 0, "", fmt.Errorf("malformed authorization header")
	}

	token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return g.jwtSecret, nil
	}, jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}))
	if err != nil || !token.Valid {
		return 0, "", fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, "", fmt.Errorf("invalid claims type")
	}

	userID, err := claimUint64(claims, "user_id")
	if err != nil || userID == 0 {
		return 0, "", fmt.Errorf("invalid user_id claim")
	}
	role, _ := claims["role"].(string)
	return userID, role, nil
}

// originAllowed checks whether origin is in the CORS allowlist (or uses a localhost pattern).
func (g *Gateway) originAllowed(origin string) bool {
	for _, allowed := range g.allowedOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
	}
	// Always allow localhost regardless of port during development.
	if strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://127.0.0.1:") {
		return true
	}
	return false
}

// claimUint64 extracts a numeric user_id from JWT MapClaims (which uses float64 for JSON numbers).
func claimUint64(claims jwt.MapClaims, key string) (uint64, error) {
	v, ok := claims[key]
	if !ok {
		return 0, fmt.Errorf("missing claim %q", key)
	}
	switch val := v.(type) {
	case float64:
		return uint64(val), nil
	case string:
		var u uint64
		_, err := fmt.Sscanf(val, "%d", &u)
		return u, err
	default:
		return 0, fmt.Errorf("unexpected type for claim %q: %T", key, v)
	}
}

// proxyKey builds a map key for the pre-built reverse proxy cache.
func proxyKey(r route) string {
	return fmt.Sprintf("%s|%s|%s", r.upstream.Host, r.stripPrefix, r.timeout)
}
