package cache

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A loader returning (nil, nil) for an interface-typed cache must not panic on
// the internal res.(V) assertion (regression: nil interface assertion panics).
func TestDoNilLoaderDoesNotPanic(t *testing.T) {
	c := NewBytes[any](1<<10, func(any) int64 { return 1 })
	v, err := c.Do("k", func() (any, error) { return nil, nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != nil {
		t.Fatalf("value = %#v, want nil", v)
	}
}

func TestPutEvictsByByteBudget(t *testing.T) {
	c := NewBytes[[]byte](10, func(b []byte) int64 { return int64(len(b)) })
	c.Put("a", make([]byte, 6))
	c.Put("b", make([]byte, 6)) // total 12 > 10 budget → evict oldest ("a")

	if _, ok := c.Get("a"); ok {
		t.Fatal("expected oldest entry to be evicted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Fatal("expected newest entry to remain")
	}
	if stats := c.Snapshot(); stats.Evictions == 0 || stats.Entries != 1 {
		t.Fatalf("stats = %+v, want one entry and at least one eviction", stats)
	}
}

func TestDisabledCacheIsAlwaysMiss(t *testing.T) {
	c := NewBytes[int](0, func(int) int64 { return 1 })
	c.Put("k", 7)
	if _, ok := c.Get("k"); ok {
		t.Fatal("disabled cache should not store values")
	}
	v, err := c.Do("k", func() (int, error) { return 9, nil })
	if err != nil || v != 9 {
		t.Fatalf("Do = %d, %v; want 9, nil", v, err)
	}
}

func TestDoCoalescesConcurrentLoads(t *testing.T) {
	c := NewBytes[int](1<<10, func(int) int64 { return 8 })
	var calls atomic.Int64
	release := make(chan struct{})

	const n = 16
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := c.Do("k", func() (int, error) {
				calls.Add(1)
				<-release // hold the singleflight slot so peers coalesce behind it
				return 42, nil
			})
			if err != nil || v != 42 {
				t.Errorf("Do = %d, %v; want 42, nil", v, err)
			}
		}()
	}

	// Let the goroutines queue on the same key, then release the single loader.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("loader called %d times, want 1 (coalesced)", got)
	}
}
