package ingest

import (
	"strings"
	"testing"
)

func TestCollectCurrentNotionAssetsAndRejectForeignSignatures(t *testing.T) {
	const id = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	const current = "https://file.notion.com/f/f/space/file/image.png?signature=secret"
	for _, typ := range []string{"image", "page"} {
		block := map[string]any{"id": id, "type": typ, "properties": map[string]any{"source": []any{[]any{current}}}, "format": map[string]any{"page_cover": current}}
		rm := NormalizedRecordMap{Block: map[string]map[string]any{id: block}, SignedURLs: map[string]string{id: "http://127.0.0.1/private"}}
		assets := CollectAssets(rm, id)
		if len(assets) != 1 || assets[0].Source != "attachment:file:image.png" || strings.Contains(assets[0].SignedURL, "127.0.0.1") {
			t.Fatalf("assets: %#v", assets)
		}
	}
}
