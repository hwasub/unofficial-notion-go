package ingest

import (
	"math"
	"os"
	"strconv"

	"github.com/hwasub/unofficial-notion-go/internal/notionrecordmap"
)

// DefaultLimitsFromEnv builds DefaultLimits from NOTION_INGESTOR_* environment
// variables, falling back to built-in defaults when a variable is unset or not
// a positive integer.
func DefaultLimitsFromEnv() DefaultLimits {
	hardMaxAssets := readInt("NOTION_INGESTOR_HARD_MAX_ASSETS", 500)
	hardMaxBlocks := readInt("NOTION_INGESTOR_HARD_MAX_BLOCKS", 10000)
	return normalizeDefaultLimits(DefaultLimits{
		HardMaxAssets:    hardMaxAssets,
		HardMaxBlocks:    hardMaxBlocks,
		MaxAssets:        readInt("NOTION_INGESTOR_MAX_ASSETS", 200),
		MaxBlocks:        readInt("NOTION_INGESTOR_MAX_BLOCKS", 5000),
		TimeoutMS:        readInt("NOTION_INGESTOR_TIMEOUT_MS", 60000),
		MaxResponseBytes: readInt("NOTION_INGESTOR_MAX_RESPONSE_BYTES", 15728640),
	})
}

// RequestLimitsFromMap resolves the effective RequestLimits from a raw request
// body map and the given defaults. Each override is parsed leniently and
// clamped to its allowed range; missing or invalid values fall back to the
// corresponding default.
func RequestLimitsFromMap(body map[string]any, defaults DefaultLimits) RequestLimits {
	defaults = normalizeDefaultLimits(defaults)
	return RequestLimits{
		MaxAssets:        clamp(body["max_assets"], defaults.MaxAssets, 0, defaults.HardMaxAssets),
		MaxBlocks:        clamp(body["max_blocks"], defaults.MaxBlocks, 1, defaults.HardMaxBlocks),
		TimeoutMS:        clamp(body["timeout_ms"], defaults.TimeoutMS, 1000, 300000),
		MaxResponseBytes: defaults.MaxResponseBytes,
	}
}

// RequestLimitsForRequest resolves the effective RequestLimits for a
// FetchRequest, applying only the request's non-zero override fields on top of
// defaults.
func RequestLimitsForRequest(req FetchRequest, defaults DefaultLimits) RequestLimits {
	body := map[string]any{}
	if req.MaxAssets != 0 {
		body["max_assets"] = req.MaxAssets
	}
	if req.MaxBlocks != 0 {
		body["max_blocks"] = req.MaxBlocks
	}
	if req.TimeoutMS != 0 {
		body["timeout_ms"] = req.TimeoutMS
	}
	return RequestLimitsFromMap(body, defaults)
}

// FetchRequestFromMap builds a FetchRequest from a raw request body map,
// reading the page_id, url, max_assets, max_blocks, and timeout_ms fields and
// parsing numeric values leniently.
func FetchRequestFromMap(body map[string]any) FetchRequest {
	return FetchRequest{
		PageID:    notionrecordmap.StringValue(body["page_id"]),
		URL:       notionrecordmap.StringValue(body["url"]),
		MaxAssets: intValue(body["max_assets"]),
		MaxBlocks: intValue(body["max_blocks"]),
		TimeoutMS: intValue(body["timeout_ms"]),
	}
}

func normalizeDefaultLimits(defaults DefaultLimits) DefaultLimits {
	defaults.HardMaxAssets = max(defaults.HardMaxAssets, 0)
	defaults.HardMaxBlocks = max(defaults.HardMaxBlocks, 1)
	defaults.MaxAssets = clampInt(defaults.MaxAssets, 0, defaults.HardMaxAssets)
	defaults.MaxBlocks = clampInt(defaults.MaxBlocks, 1, defaults.HardMaxBlocks)
	defaults.TimeoutMS = clampInt(defaults.TimeoutMS, 1000, 300000)
	if defaults.MaxResponseBytes <= 0 {
		defaults.MaxResponseBytes = 15728640
	}
	return defaults
}

func normalizeRequestLimits(limits RequestLimits) RequestLimits {
	limits.MaxAssets = max(limits.MaxAssets, 0)
	limits.MaxBlocks = max(limits.MaxBlocks, 1)
	limits.TimeoutMS = clampInt(limits.TimeoutMS, 1000, 300000)
	if limits.MaxResponseBytes <= 0 {
		limits.MaxResponseBytes = 15728640
	}
	return limits
}

func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func readInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func clamp(value any, fallback int, min int, max int) int {
	parsed, ok := parseIntValue(value)
	if !ok {
		return fallback
	}
	if parsed < min {
		return min
	}
	if parsed > max {
		return max
	}
	return parsed
}

func intValue(value any) int {
	parsed, _ := parseIntValue(value)
	return parsed
}

func parseIntValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		if typed > math.MaxInt || typed < math.MinInt {
			return 0, false
		}
		return int(typed), true
	case float64:
		// float64(math.MaxInt) rounds up to exactly 2^63, hence >= on the
		// upper bound; math.MinInt (-2^63) converts exactly.
		if math.IsNaN(typed) || math.IsInf(typed, 0) || typed < math.MinInt || typed >= float64(math.MaxInt) {
			return 0, false
		}
		return int(typed), true
	case jsonNumber:
		parsed, err := strconv.Atoi(typed.String())
		return parsed, err == nil
	case string:
		parsed, err := strconv.Atoi(typed)
		return parsed, err == nil
	case nil:
		return 0, false
	default:
		parsed, err := strconv.Atoi(notionrecordmap.StringValue(value))
		return parsed, err == nil
	}
}

type jsonNumber interface {
	String() string
}
