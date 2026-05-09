package breaker

import (
	"sync"
	"time"
)

// State represents the circuit-breaker state machine.
type State int

const (
	StateClosed   State = iota // normal operation
	StateOpen                  // blocking requests after too many failures
	StateHalfOpen              // probe state: allow one request to test recovery
)

// Breaker is a simple, goroutine-safe circuit breaker.
type Breaker struct {
	mu           sync.Mutex
	state        State
	failureCount int
	threshold    int           // consecutive failures before opening
	timeout      time.Duration // how long to stay open before half-open
	lastFailure  time.Time
}

// New —— 创建熔断器实例，设置失败阈值和开路超时时间
// New returns a Breaker with the given failure threshold and open timeout.
// Sensible defaults: threshold=5, timeout=60s.
func New(threshold int, timeout time.Duration) *Breaker {
	if threshold <= 0 {
		threshold = 5
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Breaker{
		state:     StateClosed,
		threshold: threshold,
		timeout:   timeout,
	}
}

// Allow —— 判断当前熔断器状态是否允许新请求通过
// Allow reports whether a new request may proceed.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(b.lastFailure) >= b.timeout {
			b.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		// Let exactly one probe through.
		return true
	}
	return true
}

// OnSuccess —— 记录一次成功调用，将熔断器重置为关闭状态
// OnSuccess resets the breaker to the closed state.
func (b *Breaker) OnSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failureCount = 0
	b.state = StateClosed
}

// OnFailure —— 记录一次失败调用，失败达到阈值时触发熔断
// OnFailure records a failure and may trip the breaker to open.
func (b *Breaker) OnFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failureCount++
	b.lastFailure = time.Now()

	if b.state == StateHalfOpen || b.failureCount >= b.threshold {
		b.state = StateOpen
	}
}

// State —— 返回熔断器当前状态的线程安全快照
// State returns the current breaker state (thread-safe snapshot).
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// StateString —— 返回熔断器当前状态的可读字符串（closed/open/half-open）
// StateString returns a human-readable breaker state.
func (b *Breaker) StateString() string {
	switch b.State() {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	}
	return "unknown"
}
