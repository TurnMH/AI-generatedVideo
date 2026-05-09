// Package keyrotator provides a thread-safe round-robin API key rotator.
// Use it to spread requests across multiple API keys to avoid rate limits.
//
// Usage:
//
//	kr := keyrotator.New(cfg.Models.KlingKeys...)
//	key := kr.Next() // returns "" when no keys configured
package keyrotator

import "sync/atomic"

// Rotator cycles through a list of API keys in round-robin order.
// It is safe for concurrent use.
type Rotator struct {
	keys    []string
	counter atomic.Uint64
}

// New creates a Rotator from the provided keys. Duplicate and empty keys are
// filtered out. If no valid keys remain, Next() always returns "".
func New(keys ...string) *Rotator {
	r := &Rotator{}
	seen := make(map[string]bool)
	for _, k := range keys {
		if k != "" && !seen[k] {
			r.keys = append(r.keys, k)
			seen[k] = true
		}
	}
	return r
}

// Next returns the next API key in round-robin order.
// Returns "" if the rotator was created with no valid keys.
func (r *Rotator) Next() string {
	if len(r.keys) == 0 {
		return ""
	}
	idx := r.counter.Add(1) - 1
	return r.keys[idx%uint64(len(r.keys))]
}

// Len returns the number of keys in the rotator.
func (r *Rotator) Len() int {
	return len(r.keys)
}
