package internal

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// route is the compiled, runtime representation of a RouteConfig.
type route struct {
	pattern      *regexp.Regexp // nil for prefix routes
	prefix       string         // empty for pattern routes
	upstream     *url.URL
	upstreamName string // service name used for dynamic registry lookup
	timeout      time.Duration
	public       bool   // skip JWT
	websocket    bool   // upgrade to WS
	stripPrefix  string // strip before forwarding
}

// match returns true when the route applies to the given path.
func (r *route) match(path string) bool {
	if r.pattern != nil {
		return r.pattern.MatchString(path)
	}
	return strings.HasPrefix(path, r.prefix)
}

// buildRoutes compiles a slice of RouteConfig into runtime routes.
func buildRoutes(cfgRoutes []RouteConfig, upstreams map[string]*url.URL) ([]route, error) {
	routes := make([]route, 0, len(cfgRoutes))
	for i, rc := range cfgRoutes {
		up, ok := upstreams[rc.Upstream]
		if !ok {
			return nil, fmt.Errorf("route[%d]: unknown upstream %q", i, rc.Upstream)
		}
		r := route{
			upstream:     up,
			upstreamName: rc.Upstream,
			timeout:      rc.TimeoutDuration(),
			public:       rc.Public,
			websocket:    rc.WebSocket,
			stripPrefix:  rc.StripPrefix,
		}
		switch {
		case rc.Pattern != "":
			re, err := regexp.Compile(rc.Pattern)
			if err != nil {
				return nil, fmt.Errorf("route[%d]: bad pattern: %w", i, err)
			}
			r.pattern = re
		case rc.Prefix != "":
			r.prefix = rc.Prefix
		default:
			return nil, fmt.Errorf("route[%d]: must set pattern or prefix", i)
		}
		routes = append(routes, r)
	}
	return routes, nil
}

// parseUpstreams parses the upstream address strings from config.
func parseUpstreams(raw map[string]string) (map[string]*url.URL, error) {
	out := make(map[string]*url.URL, len(raw))
	for name, addr := range raw {
		u, err := url.Parse(addr)
		if err != nil {
			return nil, fmt.Errorf("upstream %q: %w", name, err)
		}
		out[name] = u
	}
	return out, nil
}
