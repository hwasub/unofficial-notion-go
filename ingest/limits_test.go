package ingest

import (
	"math"
	"testing"
)

func TestRequestLimitsClamp(t *testing.T) {
	defaults := DefaultLimits{
		HardMaxAssets:    500,
		HardMaxBlocks:    10000,
		MaxAssets:        200,
		MaxBlocks:        5000,
		TimeoutMS:        60000,
		MaxResponseBytes: 123,
	}
	got := RequestLimitsFromMap(map[string]any{
		"max_assets": 0,
		"max_blocks": "2",
		"timeout_ms": 10,
	}, defaults)
	if got.MaxAssets != 0 || got.MaxBlocks != 2 || got.TimeoutMS != 1000 || got.MaxResponseBytes != 123 {
		t.Fatalf("RequestLimitsFromMap = %+v", got)
	}
}

func TestParseIntValueRejectsOutOfRangeAndNonFinite(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  int
		ok    bool
	}{
		{"nan", math.NaN(), 0, false},
		{"positive infinity", math.Inf(1), 0, false},
		{"negative infinity", math.Inf(-1), 0, false},
		{"huge float", 1e300, 0, false},
		{"max int64 as float", float64(math.MaxInt64), 0, false},
		{"large valid float", 1e9, 1000000000, true},
		{"int64", int64(42), 42, true},
		{"negative int64", int64(-7), -7, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseIntValue(tc.value)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("parseIntValue(%v) = (%d, %t), want (%d, %t)", tc.value, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestRequestLimitsNormalizeDefaultsBeforeFallback(t *testing.T) {
	defaults := DefaultLimits{
		HardMaxAssets:    50,
		HardMaxBlocks:    100,
		MaxAssets:        200,
		MaxBlocks:        5000,
		TimeoutMS:        10,
		MaxResponseBytes: 0,
	}
	got := RequestLimitsFromMap(map[string]any{}, defaults)
	if got.MaxAssets != 50 || got.MaxBlocks != 100 || got.TimeoutMS != 1000 || got.MaxResponseBytes != 15728640 {
		t.Fatalf("RequestLimitsFromMap = %+v", got)
	}
}
