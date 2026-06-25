// Package cache provides a small byte-budgeted LRU keyed by string with
// singleflight coalescing. It is intentionally minimal and suited for stashing
// rendered payloads or parsed assets behind hot public URLs. The byte budget is
// a soft ceiling; eviction happens during Put when usage exceeds it.
package cache

import (
	"container/list"
	"sync"
	"sync/atomic"

	"github.com/hwasub/unofficial-notion-go/internal/flight"
)

// Cache is a byte-budgeted LRU. Use NewBytes to construct one.
type Cache[V any] struct {
	mu       sync.Mutex
	items    map[string]*list.Element
	order    *list.List
	maxBytes int64
	used     int64
	sizeOf   func(V) int64

	flight flight.Group

	hits      atomic.Int64
	misses    atomic.Int64
	evictions atomic.Int64
}

type entry[V any] struct {
	key   string
	value V
	bytes int64
}

// NewBytes returns a cache with a soft byte budget. sizeOf is required and
// returns the per-entry cost; the cache adds nothing to that. If maxBytes is
// non-positive the cache disables itself (every Get is a miss, Put is a no-op).
func NewBytes[V any](maxBytes int64, sizeOf func(V) int64) *Cache[V] {
	if sizeOf == nil {
		panic("cache.NewBytes: sizeOf is required")
	}
	return &Cache[V]{
		items:    make(map[string]*list.Element),
		order:    list.New(),
		maxBytes: maxBytes,
		sizeOf:   sizeOf,
	}
}

// Get returns the stored value, marking it as most-recently-used on a hit.
func (c *Cache[V]) Get(key string) (V, bool) {
	return c.get(key, true)
}

func (c *Cache[V]) get(key string, countMiss bool) (V, bool) {
	if c == nil || c.maxBytes <= 0 {
		var zero V
		return zero, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		if countMiss {
			c.misses.Add(1)
		}
		var zero V
		return zero, false
	}
	c.order.MoveToFront(el)
	c.hits.Add(1)
	return el.Value.(*entry[V]).value, true
}

// Put stores or replaces a value, evicting oldest entries until the byte
// budget is satisfied.
func (c *Cache[V]) Put(key string, value V) {
	if c == nil || c.maxBytes <= 0 {
		return
	}
	bytes := c.sizeOf(value)
	if bytes <= 0 {
		bytes = 1
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		prev := el.Value.(*entry[V])
		c.used -= prev.bytes
		prev.value = value
		prev.bytes = bytes
		c.used += bytes
		c.order.MoveToFront(el)
		c.evictLocked()
		return
	}
	el := c.order.PushFront(&entry[V]{key: key, value: value, bytes: bytes})
	c.items[key] = el
	c.used += bytes
	c.evictLocked()
}

// Do returns the cached value or loads it via load(). Concurrent loads of the
// same key are coalesced via singleflight; load errors are returned to the
// caller and not cached.
func (c *Cache[V]) Do(key string, load func() (V, error)) (V, error) {
	if c == nil || c.maxBytes <= 0 {
		return load()
	}
	if v, ok := c.Get(key); ok {
		return v, nil
	}
	res, err := c.flight.Do(key, func() (any, error) {
		// Re-check after acquiring the singleflight slot: a concurrent loader
		// may have populated the cache while we were queued.
		if v, ok := c.get(key, false); ok {
			return v, nil
		}
		v, err := load()
		if err != nil {
			return nil, err
		}
		c.Put(key, v)
		return v, nil
	})
	if err != nil {
		var zero V
		return zero, err
	}
	// res is a nil interface when V is itself an interface type and the loader
	// returned (nil, nil). A comma-ok assertion yields the zero value instead of
	// panicking on res.(V) (which flight would then re-panic to every waiter).
	v, _ := res.(V)
	return v, nil
}

func (c *Cache[V]) evictLocked() {
	for c.used > c.maxBytes {
		el := c.order.Back()
		if el == nil {
			return
		}
		e := el.Value.(*entry[V])
		c.order.Remove(el)
		delete(c.items, e.key)
		c.used -= e.bytes
		c.evictions.Add(1)
	}
}

// Stats is a point-in-time snapshot of cache counters and size, suitable for
// exposing as metrics.
type Stats struct {
	Entries    int   `json:"entries"`
	UsedBytes  int64 `json:"used_bytes"`
	LimitBytes int64 `json:"limit_bytes"`
	Hits       int64 `json:"hits"`
	Misses     int64 `json:"misses"`
	Evictions  int64 `json:"evictions"`
}

// Snapshot returns a thread-safe view of the cache's counters.
func (c *Cache[V]) Snapshot() Stats {
	if c == nil {
		return Stats{}
	}
	c.mu.Lock()
	entries := len(c.items)
	used := c.used
	c.mu.Unlock()
	return Stats{
		Entries:    entries,
		UsedBytes:  used,
		LimitBytes: c.maxBytes,
		Hits:       c.hits.Load(),
		Misses:     c.misses.Load(),
		Evictions:  c.evictions.Load(),
	}
}
