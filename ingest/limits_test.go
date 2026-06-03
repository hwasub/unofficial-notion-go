package ingest

import "testing"

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
