package generators

import "testing"

func TestKeyPoolDedupesAndRotates(t *testing.T) {
	t.Parallel()

	pool := newKeyPool([]string{" key-a ", "key-b", "key-a", ""})
	if got := pool.size(); got != 2 {
		t.Fatalf("key pool size = %d, want 2", got)
	}

	first := pool.nextKey()
	second := pool.nextKey()
	third := pool.nextKey()

	if first != "key-a" {
		t.Fatalf("first key = %q, want key-a", first)
	}
	if second != "key-b" {
		t.Fatalf("second key = %q, want key-b", second)
	}
	if third != "key-a" {
		t.Fatalf("third key = %q, want key-a", third)
	}
}
