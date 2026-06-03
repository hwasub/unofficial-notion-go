package notion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hwasub/unofficial-notion-go/internal/flight"
)

// CacheStats is a point-in-time snapshot of render cache counters and size,
// suitable for exposing as metrics. It is returned by RenderCacheStats and
// DiskRenderCache.Snapshot.
type CacheStats struct {
	Entries    int   `json:"entries"`
	UsedBytes  int64 `json:"used_bytes"`
	LimitBytes int64 `json:"limit_bytes"`
	Hits       int64 `json:"hits"`
	Misses     int64 `json:"misses"`
	Evictions  int64 `json:"evictions"`
}

// renderCacheVersion is mixed into disk cache keys so renderer, sanitizer,
// template-contract, and local JS/CSS contract changes can invalidate cached
// Notion HTML without rewriting database rows.
const renderCacheVersion = "v1"

var (
	renderCacheMu sync.RWMutex
	renderCache   *DiskRenderCache
)

// DiskRenderCache stores rendered Notion HTML on ephemeral disk. It is a cache
// only; callers must be able to recreate values from durable database state.
type DiskRenderCache struct {
	dir      string
	maxBytes int64
	flight   flight.Group

	hits      atomic.Int64
	misses    atomic.Int64
	evictions atomic.Int64
}

// InitRenderCache enables the Notion render cache using os.TempDir. Passing a
// non-positive budget disables the cache.
func InitRenderCache(maxBytes int64) {
	initRenderCache(maxBytes, filepath.Join(os.TempDir(), "unofficial-notion-go", "notion-render"))
}

func initRenderCache(maxBytes int64, dir string) {
	renderCacheMu.Lock()
	defer renderCacheMu.Unlock()
	if maxBytes <= 0 {
		renderCache = nil
		return
	}
	renderCache = &DiskRenderCache{
		dir:      strings.TrimSpace(dir),
		maxBytes: maxBytes,
	}
}

func currentRenderCache() *DiskRenderCache {
	renderCacheMu.RLock()
	defer renderCacheMu.RUnlock()
	return renderCache
}

// RenderCacheStats returns a stats snapshot (zero value when disabled).
func RenderCacheStats() CacheStats {
	c := currentRenderCache()
	if c == nil {
		return CacheStats{}
	}
	return c.Snapshot()
}

// RenderPageCached renders a Notion page, using the ephemeral disk cache when
// enabled. locale is part of the key because RenderInput.T output is baked into
// warning and UI strings.
func RenderPageCached(input RenderInput, locale string) (string, error) {
	c := currentRenderCache()
	if c == nil {
		return RenderPage(input)
	}
	key := RenderCacheKey(input, locale)
	return c.Do(key, func() (string, error) {
		return RenderPage(input)
	})
}

// RenderCacheKey returns the stable key for a RenderInput. It intentionally
// includes only durable render inputs, not fetched_at or other ingestion-time
// metadata.
func RenderCacheKey(input RenderInput, locale string) string {
	h := sha256.New()
	writeHashPart(h, renderCacheVersion)
	writeHashPart(h, locale)
	writeHashPart(h, input.PageID)
	writeHashPart(h, input.ResourceSlug)
	writeHashPart(h, string(input.RecordMap))
	writeHashMap(h, input.PagePaths)
	writeHashMap(h, input.AssetURLs)
	for _, warning := range input.Warnings {
		writeHashPart(h, warning.Kind)
		writeHashPart(h, strconv.Itoa(warning.Count))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// RenderSourceHash hashes the canonical source needed to render one Notion
// page. It is stored for diagnostics and future cache/debug tooling; full cache
// keys also include paths, assets, locale, and renderer version.
func RenderSourceHash(recordMap json.RawMessage) string {
	h := sha256.Sum256(recordMap)
	return hex.EncodeToString(h[:])
}

// MarshalRenderWarnings serializes warnings to a JSON array string suitable for
// storage. It returns "[]" when there are no warnings or on marshal failure.
func MarshalRenderWarnings(warnings []RenderWarning) string {
	if len(warnings) == 0 {
		return "[]"
	}
	data, err := json.Marshal(warnings)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// ParseRenderWarnings is the inverse of MarshalRenderWarnings. It parses a JSON
// array of warnings, dropping any entry with an empty Kind or non-positive
// Count, and returns nil for empty or invalid input.
func ParseRenderWarnings(raw string) []RenderWarning {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var warnings []RenderWarning
	if err := json.Unmarshal([]byte(raw), &warnings); err != nil {
		return nil
	}
	out := warnings[:0]
	for _, warning := range warnings {
		warning.Kind = strings.TrimSpace(warning.Kind)
		if warning.Kind == "" || warning.Count <= 0 {
			continue
		}
		out = append(out, warning)
	}
	return out
}

func writeHashMap(h interface{ Write([]byte) (int, error) }, values map[string]string) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	writeHashPart(h, strconv.Itoa(len(keys)))
	for _, key := range keys {
		writeHashPart(h, key)
		writeHashPart(h, values[key])
	}
}

func writeHashPart(h interface{ Write([]byte) (int, error) }, value string) {
	_, _ = h.Write([]byte(strconv.Itoa(len(value))))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(value))
	_, _ = h.Write([]byte{0})
}

// Do returns the cached HTML for key, or calls load and stores its result.
// Concurrent loads of the same key are coalesced; load errors are returned and
// not cached, and values larger than the cache budget are not stored. When the
// cache is disabled it simply calls load.
func (c *DiskRenderCache) Do(key string, load func() (string, error)) (string, error) {
	if c == nil || c.maxBytes <= 0 || strings.TrimSpace(c.dir) == "" {
		return load()
	}
	if value, ok := c.get(key, true); ok {
		return value, nil
	}
	res, err := c.flight.Do(key, func() (any, error) {
		if value, ok := c.get(key, false); ok {
			return value, nil
		}
		value, err := load()
		if err != nil {
			return "", err
		}
		if int64(len(value)) <= c.maxBytes {
			_ = c.put(key, value)
		}
		return value, nil
	})
	if err != nil {
		return "", err
	}
	return res.(string), nil
}

func (c *DiskRenderCache) get(key string, countMiss bool) (string, bool) {
	path := c.pathFor(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if countMiss {
			c.misses.Add(1)
		}
		return "", false
	}
	now := time.Now()
	_ = os.Chtimes(path, now, now)
	c.hits.Add(1)
	return string(data), true
}

func (c *DiskRenderCache) put(key string, value string) error {
	path := c.pathFor(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTmp := true
	defer func() {
		if removeTmp {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.WriteString(value); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	removeTmp = false
	_ = c.prune()
	return nil
}

func (c *DiskRenderCache) pathFor(key string) string {
	if len(key) < 2 {
		key = RenderCacheKey(RenderInput{PageID: key}, "")
	}
	prefix := key[:2]
	return filepath.Join(c.dir, renderCacheVersion, prefix, key+".html")
}

type diskCacheFile struct {
	path    string
	size    int64
	modTime time.Time
}

// Snapshot returns current cache statistics by scanning the cache directory for
// entry count and total bytes alongside the live hit, miss, and eviction
// counters.
func (c *DiskRenderCache) Snapshot() CacheStats {
	files, used := c.files()
	return CacheStats{
		Entries:    len(files),
		UsedBytes:  used,
		LimitBytes: c.maxBytes,
		Hits:       c.hits.Load(),
		Misses:     c.misses.Load(),
		Evictions:  c.evictions.Load(),
	}
}

func (c *DiskRenderCache) prune() error {
	files, used := c.files()
	if used <= c.maxBytes {
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].modTime.Equal(files[j].modTime) {
			return files[i].path < files[j].path
		}
		return files[i].modTime.Before(files[j].modTime)
	})
	for _, file := range files {
		if used <= c.maxBytes {
			break
		}
		if err := os.Remove(file.path); err == nil {
			used -= file.size
			c.evictions.Add(1)
		}
	}
	return nil
}

func (c *DiskRenderCache) files() ([]diskCacheFile, int64) {
	base := filepath.Join(c.dir, renderCacheVersion)
	var files []diskCacheFile
	var used int64
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".html") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, diskCacheFile{path: path, size: info.Size(), modTime: info.ModTime()})
		used += info.Size()
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, 0
	}
	return files, used
}
