package ingest

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hwasub/unofficial-notion-go/notionapi"
)

type fakePageClient struct {
	getPageCalls        []string
	getPageOptions      []notionapi.PageOptions
	getPageRecordMap    map[string]any
	getPageErr          error
	collectionCalls     []collectionCall
	collectionData      map[string]any
	signedURLCalledWith []string
	signedURLErr        error
}

type collectionCall struct {
	CollectionID string
	ViewID       string
	SpaceID      string
}

func (f *fakePageClient) GetPage(_ context.Context, pageID string, opts notionapi.PageOptions) (map[string]any, error) {
	f.getPageCalls = append(f.getPageCalls, pageID)
	f.getPageOptions = append(f.getPageOptions, opts)
	if f.getPageErr != nil {
		return nil, f.getPageErr
	}
	return cloneMap(f.getPageRecordMap), nil
}

func (f *fakePageClient) AddSignedURLs(_ context.Context, recordMap map[string]any, contentBlockIDs []string) error {
	f.signedURLCalledWith = append([]string{}, contentBlockIDs...)
	if f.signedURLErr != nil {
		return f.signedURLErr
	}
	if _, ok := recordMap["signed_urls"]; !ok {
		recordMap["signed_urls"] = map[string]any{}
	}
	return nil
}

func (f *fakePageClient) GetCollectionData(_ context.Context, collectionID string, viewID string, _ any, opts notionapi.CollectionOptions) (map[string]any, error) {
	f.collectionCalls = append(f.collectionCalls, collectionCall{CollectionID: collectionID, ViewID: viewID, SpaceID: opts.SpaceID})
	return cloneMap(f.collectionData), nil
}

func TestFetchSnapshotFetchesOnlyRequestedPage(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	childID := "878d7114-b59d-453e-adbb-d56faf9c66d4"
	client := &fakePageClient{getPageRecordMap: recordMap(
		pageBlock(rootID, "Root", []string{childID}),
		pageBlock(childID, "Child", nil),
	)}
	snapshot, err := FetchSnapshot(context.Background(), FetchRequest{PageID: rootID}, testLimits(), FetchOptions{
		Client: client,
		Now:    fixedNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(client.getPageCalls, []string{rootID}) {
		t.Fatalf("getPage calls = %#v", client.getPageCalls)
	}
	if snapshot.Page.PageID != rootID || snapshot.Page.Title != "Root" {
		t.Fatalf("snapshot page = %#v", snapshot.Page)
	}
}

func TestFetchSnapshotSigningFailureIsBestEffort(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	client := &fakePageClient{
		getPageRecordMap: recordMap(pageBlock(rootID, "Root", nil)),
		signedURLErr:     errors.New("boom"),
	}
	var loggedSigningFailure bool
	snapshot, err := FetchSnapshot(context.Background(), FetchRequest{PageID: rootID}, testLimits(), FetchOptions{
		Client: client,
		Now:    fixedNow,
		Log: func(event string, _ map[string]any) {
			if event == "notion_asset_signing_failed" {
				loggedSigningFailure = true
			}
		},
	})
	if err != nil {
		t.Fatalf("expected snapshot to succeed, got %v", err)
	}
	if !loggedSigningFailure {
		t.Fatal("expected notion_asset_signing_failed log event")
	}
	if len(snapshot.Errors) != 1 {
		t.Fatalf("snapshot errors = %#v", snapshot.Errors)
	}
	if snapshot.Errors[0].PageID != rootID {
		t.Fatalf("snapshot error page id = %q", snapshot.Errors[0].PageID)
	}
	if !strings.Contains(snapshot.Errors[0].Message, "asset URL signing failed") {
		t.Fatalf("snapshot error message = %q", snapshot.Errors[0].Message)
	}
}

func TestFetchSnapshotRejectsMaxBlocks(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	childID := "878d7114-b59d-453e-adbb-d56faf9c66d4"
	client := &fakePageClient{getPageRecordMap: recordMap(
		pageBlock(rootID, "Root", nil),
		pageBlock(childID, "Child", nil),
	)}
	_, err := FetchSnapshot(context.Background(), FetchRequest{PageID: rootID}, testLimitsWithMaxBlocks(1), FetchOptions{Client: client, Now: fixedNow})
	if err == nil {
		t.Fatal("expected error")
	}
	httpErr, ok := AsHTTPError(err)
	if !ok || httpErr.StatusCode != 413 || httpErr.Code != "max_blocks_exceeded" {
		t.Fatalf("error = %#v", err)
	}
}

func TestFetchSnapshotPassesMaxBlocksToPageClient(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	client := &fakePageClient{getPageRecordMap: recordMap(pageBlock(rootID, "Root", nil))}
	_, err := FetchSnapshot(context.Background(), FetchRequest{PageID: rootID}, testLimitsWithMaxBlocks(17), FetchOptions{Client: client, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.getPageOptions) != 1 {
		t.Fatalf("get page options = %#v", client.getPageOptions)
	}
	if client.getPageOptions[0].MaxBlocks != 17 {
		t.Fatalf("MaxBlocks passed to GetPage = %d, want 17", client.getPageOptions[0].MaxBlocks)
	}
}

func TestFetchSnapshotMarksAssetTruncationOnlyWhenAssetsDropped(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	image1ID := "2ad6e61c-f824-80c9-a6c4-d251043457d3"
	image2ID := "3ad6e61c-f824-80c9-a6c4-d251043457d3"
	client := &fakePageClient{getPageRecordMap: recordMap(
		pageBlock(rootID, "Root", []string{image1ID, image2ID}),
		imageBlock(image1ID, "https://www.notion.so/images/page-cover/solid_blue.png"),
		imageBlock(image2ID, "https://www.notion.so/images/page-cover/solid_red.png"),
	)}

	exact, err := FetchSnapshot(context.Background(), FetchRequest{PageID: rootID}, testLimitsWithMaxAssets(2), FetchOptions{Client: client, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	if exact.Truncated.Assets {
		t.Fatalf("exact asset limit marked truncated: %+v", exact.Truncated)
	}

	truncated, err := FetchSnapshot(context.Background(), FetchRequest{PageID: rootID}, testLimitsWithMaxAssets(1), FetchOptions{Client: client, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	if !truncated.Truncated.Assets || len(truncated.Assets) != 1 || truncated.Page.AssetCount != 2 {
		t.Fatalf("asset truncation = flags %+v, assets %d, total %d", truncated.Truncated, len(truncated.Assets), truncated.Page.AssetCount)
	}
}

func TestFetchSnapshotDoesNotMarkExactBlockLimitAsTruncated(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	childID := "878d7114-b59d-453e-adbb-d56faf9c66d4"
	client := &fakePageClient{getPageRecordMap: recordMap(
		pageBlock(rootID, "Root", []string{childID}),
		pageBlock(childID, "Child", nil),
	)}
	snapshot, err := FetchSnapshot(context.Background(), FetchRequest{PageID: rootID}, testLimitsWithMaxBlocks(2), FetchOptions{Client: client, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Truncated.Blocks {
		t.Fatalf("exact block limit marked truncated: %+v", snapshot.Truncated)
	}
}

func TestFetchSnapshotMapsNotFound(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	client := &fakePageClient{getPageErr: errors.New(`Notion page not found "root"`)}
	_, err := FetchSnapshot(context.Background(), FetchRequest{PageID: rootID}, testLimits(), FetchOptions{Client: client, Now: fixedNow})
	httpErr, ok := AsHTTPError(err)
	if !ok || httpErr.StatusCode != 404 || httpErr.Code != "notion_page_not_found" {
		t.Fatalf("error = %#v", err)
	}
}

func TestMapNotionUpstreamErrors(t *testing.T) {
	tests := []struct {
		name       string
		cause      error
		statusCode int
		code       string
		retryAfter int
	}{
		{
			name:       "upstream 404",
			cause:      &notionapi.HTTPError{StatusCode: http.StatusNotFound, Message: "missing"},
			statusCode: http.StatusNotFound,
			code:       "notion_page_not_found",
		},
		{
			name:       "upstream 429",
			cause:      &notionapi.HTTPError{StatusCode: http.StatusTooManyRequests, Message: "rate limited", RetryAfterMS: 2500},
			statusCode: http.StatusTooManyRequests,
			code:       "notion_rate_limited",
			retryAfter: 2500,
		},
		{
			name:       "oversized response",
			cause:      &notionapi.HTTPError{StatusCode: http.StatusRequestEntityTooLarge, Code: notionapi.ErrorCodeMaxResponseBytesExceeded, Message: "notion response exceeded max response bytes"},
			statusCode: http.StatusRequestEntityTooLarge,
			code:       notionapi.ErrorCodeMaxResponseBytesExceeded,
		},
		{
			name:       "upstream 5xx",
			cause:      &notionapi.HTTPError{StatusCode: http.StatusServiceUnavailable, Message: "temporarily unavailable"},
			statusCode: http.StatusBadGateway,
			code:       "notion_upstream_error",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mapNotionError(tc.cause)
			if got.StatusCode != tc.statusCode || got.Code != tc.code || got.RetryAfterMS != tc.retryAfter {
				t.Fatalf("mapNotionError = %#v", got)
			}
		})
	}
}

func TestFetchSnapshotRepairsCollectionQueries(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	collectionBlockID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	collectionID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	viewID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	rowID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	client := &fakePageClient{
		getPageRecordMap: map[string]any{
			"block": map[string]any{
				rootID:            pageBlock(rootID, "Root", []string{collectionBlockID})[1],
				collectionBlockID: boxed(map[string]any{"id": collectionBlockID, "type": "collection_view", "collection_id": collectionID, "view_ids": []any{viewID}, "space_id": "space-id"}),
			},
			"collection": map[string]any{},
			"collection_view": map[string]any{
				viewID: boxed(map[string]any{"id": viewID, "type": "board", "name": "Board", "format": map[string]any{}}),
			},
			"collection_query": map[string]any{
				collectionID: map[string]any{viewID: map[string]any{"collection_group_results": map[string]any{"blockIds": []any{rowID}}}},
			},
			"signed_urls": map[string]any{},
		},
		collectionData: map[string]any{
			"result": map[string]any{"reducerResults": map[string]any{
				"board_columns": map[string]any{"results": []any{map[string]any{"value": map[string]any{"type": "select", "value": "Ready"}, "visible": true}}},
			}},
			"recordMap": recordMap(pageBlock(rowID, "Card", nil)),
		},
	}
	snapshot, err := FetchSnapshot(context.Background(), FetchRequest{PageID: rootID}, testLimits(), FetchOptions{Client: client, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(client.collectionCalls, []collectionCall{{CollectionID: collectionID, ViewID: viewID, SpaceID: "space-id"}}) {
		t.Fatalf("collection calls = %#v", client.collectionCalls)
	}
	query := snapshot.Page.RecordMap.CollectionQuery.(map[string]any)
	viewQuery := query[collectionID].(map[string]any)[viewID].(map[string]any)
	if viewQuery["board_columns"] == nil {
		t.Fatalf("collection query not repaired: %#v", viewQuery)
	}
	if title := GetTitle(snapshot.Page.RecordMap.Block[rowID]); title != "Card" {
		t.Fatalf("merged row title = %q", title)
	}
}

func TestFetchSnapshotRejectsCollectionRepairOverMaxBlocks(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	collectionBlockID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	collectionID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	viewID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	rowID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	client := &fakePageClient{
		getPageRecordMap: map[string]any{
			"block": map[string]any{
				rootID:            pageBlock(rootID, "Root", []string{collectionBlockID})[1],
				collectionBlockID: boxed(map[string]any{"id": collectionBlockID, "type": "collection_view", "collection_id": collectionID, "view_ids": []any{viewID}}),
			},
			"collection": map[string]any{},
			"collection_view": map[string]any{
				viewID: boxed(map[string]any{"id": viewID, "type": "table", "name": "Table", "format": map[string]any{}}),
			},
			"collection_query": map[string]any{
				collectionID: map[string]any{viewID: map[string]any{"collection_group_results": map[string]any{"blockIds": []any{rowID}}}},
			},
			"signed_urls": map[string]any{},
		},
		collectionData: map[string]any{
			"result":    map[string]any{"reducerResults": map[string]any{}},
			"recordMap": recordMap(pageBlock(rowID, "Card", nil)),
		},
	}
	_, err := FetchSnapshot(context.Background(), FetchRequest{PageID: rootID}, testLimitsWithMaxBlocks(2), FetchOptions{Client: client, Now: fixedNow})
	httpErr, ok := AsHTTPError(err)
	if !ok || httpErr.StatusCode != http.StatusRequestEntityTooLarge || httpErr.Code != "max_blocks_exceeded" {
		t.Fatalf("error = %#v", err)
	}
}

func TestCollectionQueryNeedsRepairForGroupedViews(t *testing.T) {
	view := map[string]any{
		"type": "board",
		"format": map[string]any{
			"board_columns_by": map[string]any{"property": "K{tS", "type": "multi_select"},
			"board_columns": []any{
				map[string]any{"property": "K{tS", "value": map[string]any{"type": "multi_select", "value": "group 1"}},
			},
		},
	}
	query := map[string]any{
		"collection_group_results": map[string]any{"blockIds": []any{"row-id"}},
	}
	recordMap := map[string]any{
		"block": map[string]any{"row-id": boxed(map[string]any{"id": "row-id", "type": "page"})},
	}
	if !collectionQueryNeedsRepair(query, view, recordMap) {
		t.Fatal("expected grouped collection query to need repair")
	}
}

func testLimits() RequestLimits {
	return RequestLimits{MaxAssets: 200, MaxBlocks: 5000, TimeoutMS: 60000, MaxResponseBytes: 15728640}
}

func testLimitsWithMaxBlocks(maxBlocks int) RequestLimits {
	limits := testLimits()
	limits.MaxBlocks = maxBlocks
	return limits
}

func testLimitsWithMaxAssets(maxAssets int) RequestLimits {
	limits := testLimits()
	limits.MaxAssets = maxAssets
	return limits
}

func fixedNow() time.Time {
	return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
}

func pageBlock(id string, title string, content []string) [2]any {
	block := map[string]any{
		"id":         id,
		"type":       "page",
		"properties": map[string]any{"title": []any{[]any{title}}},
	}
	if content != nil {
		items := make([]any, 0, len(content))
		for _, child := range content {
			items = append(items, child)
		}
		block["content"] = items
	}
	return [2]any{id, boxed(block)}
}

func imageBlock(id string, source string) [2]any {
	return [2]any{id, boxed(map[string]any{
		"id":   id,
		"type": "image",
		"properties": map[string]any{
			"source": []any{[]any{source}},
		},
	})}
}

func recordMap(entries ...[2]any) map[string]any {
	blocks := map[string]any{}
	for _, entry := range entries {
		blocks[entry[0].(string)] = entry[1]
	}
	return map[string]any{"block": blocks, "collection": map[string]any{}, "collection_view": map[string]any{}, "collection_query": map[string]any{}, "signed_urls": map[string]any{}}
}

func boxed(value map[string]any) map[string]any {
	return map[string]any{"value": value}
}

func cloneMap(value map[string]any) map[string]any {
	return cloneAny(value).(map[string]any)
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, child := range typed {
			out[key] = cloneAny(child)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = cloneAny(child)
		}
		return out
	default:
		return typed
	}
}
