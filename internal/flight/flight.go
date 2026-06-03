// Package flight provides a minimal single-flight call coalescer: concurrent
// calls keyed by the same string share a single execution of the supplied
// function. It is a small standard-library replacement for the one Do use case
// this module needs from golang.org/x/sync/singleflight, which keeps the module
// free of external dependencies.
package flight

import "sync"

// Group coalesces concurrent calls keyed by string so only one invocation of
// the supplied function runs at a time per key; callers that arrive while a
// call is in flight wait for and share its result. The zero value is ready to
// use and must not be copied after first use.
type Group struct {
	mu sync.Mutex
	m  map[string]*call
}

type call struct {
	wg         sync.WaitGroup
	val        any
	err        error
	panicValue any
}

// Do runs fn for key, coalescing concurrent calls with the same key. The
// returned value and error are shared by every caller that joined the same
// in-flight call. The key is released once fn returns, so a later Do re-runs fn.
func (g *Group) Do(key string, fn func() (any, error)) (any, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		if c.panicValue != nil {
			panic(c.panicValue)
		}
		return c.val, c.err
	}
	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	defer func() {
		if recovered := recover(); recovered != nil {
			c.panicValue = recovered
		}
		c.wg.Done()

		g.mu.Lock()
		delete(g.m, key)
		g.mu.Unlock()

		if c.panicValue != nil {
			panic(c.panicValue)
		}
	}()

	c.val, c.err = fn()
	return c.val, c.err
}
