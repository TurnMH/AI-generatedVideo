package internal

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const registryTTL = 30 * time.Second

type registryEntry struct {
	Addr      string
	UpdatedAt time.Time
}

// Registry is a thread-safe in-memory service registry.
// Services call Register() periodically; Resolve() returns the address if live.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]registryEntry
}

func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]registryEntry)}
}

func (r *Registry) Register(name, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[name] = registryEntry{Addr: addr, UpdatedAt: time.Now()}
}

// Resolve returns the registered address if it was refreshed within TTL.
func (r *Registry) Resolve(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[name]
	if !ok || time.Since(e.UpdatedAt) > registryTTL {
		return "", false
	}
	return e.Addr, true
}

// HandleRegister handles POST /_internal/register
// TODO: In production, this endpoint must be firewall-protected (internal traffic only).
func (r *Registry) HandleRegister(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Name string `json:"name"`
		Addr string `json:"addr"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.Name == "" || body.Addr == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	r.Register(body.Name, body.Addr)
	w.WriteHeader(http.StatusOK)
}

// HandleList handles GET /_internal/services
// TODO: In production, this endpoint must be firewall-protected (internal traffic only).
func (r *Registry) HandleList(w http.ResponseWriter, _ *http.Request) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	type entry struct {
		Name      string    `json:"name"`
		Addr      string    `json:"addr"`
		UpdatedAt time.Time `json:"updated_at"`
		Live      bool      `json:"live"`
	}
	var result []entry
	for name, e := range r.entries {
		result = append(result, entry{
			Name:      name,
			Addr:      e.Addr,
			UpdatedAt: e.UpdatedAt,
			Live:      time.Since(e.UpdatedAt) <= registryTTL,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result) //nolint:errcheck
}
