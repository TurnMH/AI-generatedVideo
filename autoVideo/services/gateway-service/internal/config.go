package internal

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level gateway configuration.
type Config struct {
	Port      int               `yaml:"port"`
	JWT       JWTConfig         `yaml:"jwt"`
	CORS      CORSConfig        `yaml:"cors"`
	Upstreams map[string]string `yaml:"upstreams"`
	Routes    []RouteConfig     `yaml:"routes"`
}

// JWTConfig holds the shared HMAC secret used to validate access tokens.
type JWTConfig struct {
	Secret string `yaml:"secret"`
}

// CORSConfig lists origins allowed to make cross-origin requests.
type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
}

// RouteConfig is the YAML representation of a single routing rule.
// Either Pattern (regex) or Prefix must be set.
type RouteConfig struct {
	Pattern     string `yaml:"pattern"`      // regex matched against request path
	Prefix      string `yaml:"prefix"`       // simple HasPrefix match
	Upstream    string `yaml:"upstream"`     // key in upstreams map
	Timeout     string `yaml:"timeout"`      // e.g. "30s"; default 30s
	Public      bool   `yaml:"public"`       // skip JWT auth when true
	WebSocket   bool   `yaml:"websocket"`    // proxy as WebSocket
	StripPrefix string `yaml:"strip_prefix"` // strip this prefix before forwarding
}

// TimeoutDuration parses the Timeout string, returning 30 s as default.
func (r RouteConfig) TimeoutDuration() time.Duration {
	if r.Timeout == "" {
		return 30 * time.Second
	}
	d, err := time.ParseDuration(r.Timeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

// LoadConfig reads and parses the YAML config file at path.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 8000
	}
	return &cfg, nil
}
