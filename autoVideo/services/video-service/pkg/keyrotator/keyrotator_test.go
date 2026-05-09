package keyrotator_test

import (
	"sync"
	"testing"

	"github.com/autovideo/video-service/pkg/keyrotator"
)

func TestNew_DeduplicatesAndFiltersEmpty(t *testing.T) {
	r := keyrotator.New("a", "b", "", "a", "c")
	if r.Len() != 3 {
		t.Fatalf("expected 3 unique keys, got %d", r.Len())
	}
}

func TestNext_RoundRobin(t *testing.T) {
	r := keyrotator.New("k1", "k2", "k3")
	seen := map[string]int{}
	for i := 0; i < 9; i++ {
		seen[r.Next()]++
	}
	for _, k := range []string{"k1", "k2", "k3"} {
		if seen[k] != 3 {
			t.Errorf("key %q used %d times, want 3", k, seen[k])
		}
	}
}

func TestNext_EmptyRotator(t *testing.T) {
	r := keyrotator.New()
	if got := r.Next(); got != "" {
		t.Errorf("empty rotator Next() = %q, want \"\"", got)
	}
}

func TestNext_Concurrent(t *testing.T) {
	r := keyrotator.New("x", "y")
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			k := r.Next()
			if k != "x" && k != "y" {
				t.Errorf("unexpected key %q", k)
			}
		}()
	}
	wg.Wait()
}
