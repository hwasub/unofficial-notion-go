package flight

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGroupReturnsValueAndError(t *testing.T) {
	var g Group

	v, err := g.Do("k", func() (any, error) { return 42, nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 42 {
		t.Fatalf("value = %v, want 42", v)
	}

	wantErr := errors.New("boom")
	_, err = g.Do("k", func() (any, error) { return nil, wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestGroupReleasesKeyAfterCall(t *testing.T) {
	var g Group
	var calls atomic.Int64

	for i := 0; i < 3; i++ {
		if _, err := g.Do("k", func() (any, error) {
			calls.Add(1)
			return "v", nil
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	// Each sequential call must re-run fn because the key is released once fn
	// returns; coalescing only applies to overlapping in-flight calls.
	if got := calls.Load(); got != 3 {
		t.Fatalf("fn ran %d times, want 3", got)
	}
}

func TestGroupReleasesKeyAfterPanic(t *testing.T) {
	var g Group

	recovered := mustPanic(t, func() {
		_, _ = g.Do("k", func() (any, error) {
			panic("boom")
		})
	})
	if recovered != "boom" {
		t.Fatalf("panic = %#v, want boom", recovered)
	}

	v, err := g.Do("k", func() (any, error) {
		return "after", nil
	})
	if err != nil {
		t.Fatalf("unexpected error after panic: %v", err)
	}
	if v != "after" {
		t.Fatalf("value after panic = %v, want after", v)
	}
}

func TestGroupCoalescesConcurrentCalls(t *testing.T) {
	var g Group
	var calls atomic.Int64
	entered := make(chan struct{})
	release := make(chan struct{})

	// Occupy the in-flight slot for key "k" and hold it open until release.
	leader := make(chan any, 1)
	go func() {
		v, _ := g.Do("k", func() (any, error) {
			calls.Add(1)
			close(entered)
			<-release
			return "shared", nil
		})
		leader <- v
	}()
	<-entered // the leader is now running fn and holding the key

	// Followers join while the leader is still in flight; their fn must not run.
	const n = 16
	var wg sync.WaitGroup
	wg.Add(n)
	out := make([]any, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			v, _ := g.Do("k", func() (any, error) {
				calls.Add(1)
				return "own", nil
			})
			out[i] = v
		}(i)
	}
	// Let the followers park on the shared call before releasing the leader.
	for i := 0; i < 100; i++ {
		runtime.Gosched()
	}
	time.Sleep(10 * time.Millisecond)
	close(release)

	wg.Wait()
	leaderVal := <-leader

	if got := calls.Load(); got != 1 {
		t.Fatalf("fn ran %d times, want 1 (concurrent calls must coalesce)", got)
	}
	if leaderVal != "shared" {
		t.Fatalf("leader value = %v, want %q", leaderVal, "shared")
	}
	for i, v := range out {
		if v != "shared" {
			t.Fatalf("follower[%d] = %v, want shared result %q", i, v, "shared")
		}
	}
}

func TestGroupPropagatesPanicToFollowers(t *testing.T) {
	var g Group
	entered := make(chan struct{})
	release := make(chan struct{})

	leaderRecovered := make(chan any, 1)
	go func() {
		defer func() { leaderRecovered <- recover() }()
		_, _ = g.Do("k", func() (any, error) {
			close(entered)
			<-release
			panic("boom")
		})
	}()
	<-entered

	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	recovered := make([]any, n)
	var followerFns atomic.Int64
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			defer func() { recovered[i] = recover() }()
			_, _ = g.Do("k", func() (any, error) {
				followerFns.Add(1)
				return "own", nil
			})
		}(i)
	}
	for i := 0; i < 100; i++ {
		runtime.Gosched()
	}
	time.Sleep(10 * time.Millisecond)
	close(release)

	wg.Wait()
	if got := <-leaderRecovered; got != "boom" {
		t.Fatalf("leader panic = %#v, want boom", got)
	}
	if got := followerFns.Load(); got != 0 {
		t.Fatalf("follower fn ran %d times, want 0", got)
	}
	for i, got := range recovered {
		if got != "boom" {
			t.Fatalf("follower[%d] panic = %#v, want boom", i, got)
		}
	}
}

func mustPanic(t *testing.T, fn func()) any {
	t.Helper()
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		fn()
	}()
	if recovered == nil {
		t.Fatal("expected panic")
	}
	return recovered
}
