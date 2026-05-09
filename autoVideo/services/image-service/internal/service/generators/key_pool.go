package generators

import (
	"strings"
	"sync/atomic"
)

type keyPool struct {
	keys []string
	next uint64
}

func newKeyPool(keys []string) keyPool {
	seen := make(map[string]struct{}, len(keys))
	normalized := make([]string, 0, len(keys))
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return keyPool{keys: normalized}
}

func (p *keyPool) size() int {
	return len(p.keys)
}

func (p *keyPool) nextKey() string {
	if len(p.keys) == 0 {
		return ""
	}
	idx := atomic.AddUint64(&p.next, 1) - 1
	return p.keys[idx%uint64(len(p.keys))]
}
