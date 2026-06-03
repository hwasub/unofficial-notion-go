package notion_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hwasub/unofficial-notion-go/ingest"
	"github.com/hwasub/unofficial-notion-go/notion"
	"github.com/hwasub/unofficial-notion-go/notionapi"
)

// fakePageClient is a tiny in-memory implementation of ingest.PageClient. It
// returns a minimal Notion record map containing a single page block with one
// text child, which is enough to drive ingest.FetchSnapshot and notion.RenderPage
// without contacting Notion. AddSignedURLs and GetCollectionData are no-ops.
type fakePageClient struct{}

func (fakePageClient) GetPage(_ context.Context, pageID string, _ notionapi.PageOptions) (map[string]any, error) {
	const textID = "878d7114b59d453eadbbd56faf9c66d4"
	// The record map mirrors Notion's "boxed" shape: each record is wrapped in a
	// {"value": {...}} envelope. NormalizeRecordMap unboxes and re-keys it.
	return map[string]any{
		"block": map[string]any{
			pageID: map[string]any{
				"value": map[string]any{
					"id":         pageID,
					"type":       "page",
					"properties": map[string]any{"title": []any{[]any{"Hello, Notion"}}},
					"content":    []any{textID},
				},
			},
			textID: map[string]any{
				"value": map[string]any{
					"id":         textID,
					"type":       "text",
					"parent_id":  pageID,
					"properties": map[string]any{"title": []any{[]any{"A paragraph of text."}}},
				},
			},
		},
	}, nil
}

func (fakePageClient) AddSignedURLs(context.Context, map[string]any, []string) error {
	return nil
}

func (fakePageClient) GetCollectionData(context.Context, string, string, any, notionapi.CollectionOptions) (map[string]any, error) {
	return map[string]any{}, nil
}

// Example demonstrates the high-level flow: fetch a page snapshot with
// ingest.FetchSnapshot (here backed by an in-memory PageClient) and inspect the
// resulting Snapshot. In production you would omit FetchOptions.Client and let
// FetchSnapshot construct a real notionapi client.
func Example() {
	ctx := context.Background()
	request := ingest.FetchRequest{PageID: "1ad6e61cf82480c9a6c4d251043457d3"}
	limits := ingest.RequestLimits{MaxAssets: 100, MaxBlocks: 1000, TimeoutMS: 5000}

	snapshot, err := ingest.FetchSnapshot(ctx, request, limits, ingest.FetchOptions{
		Client: fakePageClient{},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(snapshot.RootPageID)
	fmt.Println(snapshot.Page.Title)
	// Output:
	// 1ad6e61c-f824-80c9-a6c4-d251043457d3
	// Hello, Notion
}

// ExampleRenderPage shows how to turn a fetched Snapshot's record map into
// sanitized, self-contained HTML with notion.RenderPage. The record map carried
// in a Snapshot serializes directly into RenderInput.RecordMap.
func ExampleRenderPage() {
	ctx := context.Background()
	request := ingest.FetchRequest{PageID: "1ad6e61cf82480c9a6c4d251043457d3"}
	limits := ingest.RequestLimits{MaxAssets: 100, MaxBlocks: 1000, TimeoutMS: 5000}

	snapshot, err := ingest.FetchSnapshot(ctx, request, limits, ingest.FetchOptions{
		Client: fakePageClient{},
	})
	if err != nil {
		panic(err)
	}

	recordMap, err := json.Marshal(snapshot.Page.RecordMap)
	if err != nil {
		panic(err)
	}

	htmlOut, err := notion.RenderPage(notion.RenderInput{
		RecordMap: recordMap,
		PageID:    snapshot.RootPageID,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(strings.Contains(htmlOut, "A paragraph of text."))
	// Output:
	// true
}
