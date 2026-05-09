package generators

import (
	"strings"
	"sync"
	"time"
)

// smartKeyPool is a drop-in replacement for keyPool that tracks per-key health.
// Keys that receive 429 rate-limit responses are placed on cooldown so subsequent
// requests skip them and use healthy keys instead. Consecutive non-429 failures
// also increase a penalty that temporarily deprioritises the key.
//
// Thread-safe: all public methods are guarded by a mutex.
type smartKeyPool struct {
	mu       sync.Mutex
	keys     []string
	states   []keyState
	next     uint64
}

type keyState struct {
	cooldownUntil time.Time // 429 → key is unusable until this time
	failures      int       // consecutive non-429 failures
	lastFailure   time.Time
}

const (
	// Base cooldown after a 429; doubles with consecutive 429s (capped at maxCooldown).
	baseCooldown = 20 * time.Second
	maxCooldown  = 120 * time.Second
	// After this many consecutive failures the key is deprioritised (not removed).
	maxConsecFailures = 5
	// Failures older than this are forgiven.
	failureDecayWindow = 3 * time.Minute
)

func newSmartKeyPool(keys []string) *smartKeyPool {
	seen := make(map[string]struct{}, len(keys))
	var normalized []string
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		if _, dup := seen[trimmed]; dup {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return &smartKeyPool{
		keys:   normalized,
		states: make([]keyState, len(normalized)),
	}
}

func (p *smartKeyPool) size() int {
	return len(p.keys)
}

// nextKey picks the best available key. It prefers keys that are not on cooldown
// and have the fewest consecutive failures. Falls back to the key whose cooldown
// expires soonest if all keys are cooling down.
func (p *smartKeyPool) nextKey() string {
	if len(p.keys) == 0 {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	n := len(p.keys)

	// Round-robin start index to spread load.
	p.next++
	start := int(p.next-1) % n

	bestIdx := -1
	bestScore := int(^uint(0) >> 1) // max int
	var earliestCooldown time.Time
	fallbackIdx := -1

	for i := 0; i < n; i++ {
		idx := (start + i) % n
		st := &p.states[idx]

		// Decay old failures.
		if !st.lastFailure.IsZero() && now.Sub(st.lastFailure) > failureDecayWindow {
			st.failures = 0
		}

		if now.Before(st.cooldownUntil) {
			// Key is on cooldown — track as fallback.
			if fallbackIdx == -1 || st.cooldownUntil.Before(earliestCooldown) {
				fallbackIdx = idx
				earliestCooldown = st.cooldownUntil
			}
			continue
		}

		// Available key — pick the one with fewest failures.
		if st.failures < bestScore {
			bestScore = st.failures
			bestIdx = idx
		}
	}

	if bestIdx >= 0 {
		return p.keys[bestIdx]
	}
	// All keys on cooldown — return the one that recovers soonest.
	if fallbackIdx >= 0 {
		return p.keys[fallbackIdx]
	}
	return p.keys[start%n]
}

// ReportSuccess resets the failure counter for the given key.
func (p *smartKeyPool) ReportSuccess(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, k := range p.keys {
		if k == key {
			p.states[i].failures = 0
			return
		}
	}
}

// ReportFailure records a failure for the given key.
// If is429 is true the key is placed on exponential cooldown.
func (p *smartKeyPool) ReportFailure(key string, is429 bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, k := range p.keys {
		if k == key {
			st := &p.states[i]
			st.failures++
			st.lastFailure = time.Now()
			if is429 {
				cooldown := baseCooldown * time.Duration(1<<min(st.failures-1, 4))
				if cooldown > maxCooldown {
					cooldown = maxCooldown
				}
				st.cooldownUntil = time.Now().Add(cooldown)
			}
			return
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}