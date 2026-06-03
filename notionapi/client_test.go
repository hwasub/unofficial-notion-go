package notionapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewAppliesDefaultsThenOptions(t *testing.T) {
	client := New()
	if client.apiBaseURL != "https://www.notion.so/api/v3" {
		t.Fatalf("apiBaseURL = %q", client.apiBaseURL)
	}
	if client.userTimeZone != "America/New_York" {
		t.Fatalf("userTimeZone = %q", client.userTimeZone)
	}
	if client.httpClient == nil || client.httpClient.Timeout != 60*time.Second {
		t.Fatalf("httpClient = %#v", client.httpClient)
	}
	if client.maxResponseBytes != defaultMaxResponseBytes {
		t.Fatalf("maxResponseBytes = %d", client.maxResponseBytes)
	}

	hc := &http.Client{Timeout: time.Second}
	configured := New(
		WithAPIBaseURL("https://example.test/api/"),
		WithAuthToken("token"),
		WithActiveUser("user"),
		WithUserTimeZone("UTC"),
		WithHTTPClient(hc),
		WithMaxResponseBytes(1024),
	)
	if configured.apiBaseURL != "https://example.test/api" {
		t.Fatalf("apiBaseURL trailing slash not trimmed: %q", configured.apiBaseURL)
	}
	if configured.authToken != "token" || configured.activeUser != "user" {
		t.Fatalf("auth/user not set: %#v", configured)
	}
	if configured.userTimeZone != "UTC" {
		t.Fatalf("userTimeZone = %q", configured.userTimeZone)
	}
	if configured.httpClient != hc {
		t.Fatalf("httpClient not overridden")
	}
	if configured.maxResponseBytes != 1024 {
		t.Fatalf("maxResponseBytes = %d", configured.maxResponseBytes)
	}
}

func TestZeroValueClientNormalizesDefaults(t *testing.T) {
	c := (&Client{}).normalized()
	if c.apiBaseURL != "https://www.notion.so/api/v3" {
		t.Fatalf("apiBaseURL = %q", c.apiBaseURL)
	}
	if c.userTimeZone != "America/New_York" {
		t.Fatalf("userTimeZone = %q", c.userTimeZone)
	}
	if c.httpClient == nil {
		t.Fatal("httpClient nil")
	}
	if c.maxResponseBytes != defaultMaxResponseBytes {
		t.Fatalf("maxResponseBytes = %d", c.maxResponseBytes)
	}
}

// normalized must not mutate the receiver, so a shared *Client stays safe for
// concurrent use across request methods. Run with -race.
func TestClientConcurrentUseIsRaceFree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := client.Fetch(context.Background(), "loadPageChunk", map[string]any{}, nil, nil); err != nil {
				t.Errorf("Fetch: %v", err)
			}
		}()
	}
	wg.Wait()
}

// A nil *Client falls back to a default client rather than panicking.
func TestNilClientUsesDefaults(t *testing.T) {
	var c *Client
	if got := c.normalized().apiBaseURL; got != "https://www.notion.so/api/v3" {
		t.Fatalf("nil client apiBaseURL = %q", got)
	}
}

func TestFetchRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"a":"` + strings.Repeat("x", 64) + `"}`))
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL), WithMaxResponseBytes(16))
	_, err := client.Fetch(context.Background(), "loadPageChunk", map[string]any{}, nil, nil)
	if err == nil {
		t.Fatal("expected error for oversized response")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error type = %#v", err)
	}
	if httpErr.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status code = %d", httpErr.StatusCode)
	}
	if httpErr.Code != ErrorCodeMaxResponseBytesExceeded {
		t.Fatalf("error code = %q", httpErr.Code)
	}
	if !strings.Contains(httpErr.Message, "max response bytes") {
		t.Fatalf("error message = %q", httpErr.Message)
	}
}

func TestFetchAllowsResponseAtLimit(t *testing.T) {
	body := `{"ok":true}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL), WithMaxResponseBytes(int64(len(body))))
	out, err := client.Fetch(context.Background(), "loadPageChunk", map[string]any{}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("out = %#v", out)
	}
}

func TestAddSignedURLsReturnsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	recordMap := map[string]any{
		"block": map[string]any{
			"block-1": map[string]any{"value": map[string]any{
				"id":         "block-1",
				"type":       "file",
				"properties": map[string]any{"source": []any{[]any{"https://prod-files-secure/x"}}},
			}},
		},
	}
	err := client.AddSignedURLs(context.Background(), recordMap, []string{"block-1"})
	if err == nil {
		t.Fatal("expected error from AddSignedURLs")
	}
	if !strings.Contains(err.Error(), "get signed file urls") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestAddSignedURLsCountMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"signedUrls":[]}`))
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	recordMap := map[string]any{
		"block": map[string]any{
			"block-1": map[string]any{"value": map[string]any{
				"id":         "block-1",
				"type":       "file",
				"properties": map[string]any{"source": []any{[]any{"https://prod-files-secure/x"}}},
			}},
		},
	}
	err := client.AddSignedURLs(context.Background(), recordMap, []string{"block-1"})
	if err == nil {
		t.Fatal("expected count mismatch error")
	}
	if !strings.Contains(err.Error(), "count mismatch") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestAddSignedURLsNoFileInstances(t *testing.T) {
	client := New()
	recordMap := map[string]any{"block": map[string]any{}}
	if err := client.AddSignedURLs(context.Background(), recordMap, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := recordMap["signed_urls"].(map[string]any); !ok {
		t.Fatalf("signed_urls not reset: %#v", recordMap["signed_urls"])
	}
}

func TestGetCollectionDataBuildsGroupedBoardReducers(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("src"); got != "initial_load" {
			t.Fatalf("src query = %q", got)
		}
		if got := r.Header.Get("x-notion-space-id"); got != "space-id" {
			t.Fatalf("space header = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"reducerResults":{}}}`))
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	_, err := client.GetCollectionData(context.Background(), "collection-id", "view-id", map[string]any{
		"type": "board",
		"format": map[string]any{
			"board_columns_by": map[string]any{"property": "K{tS", "type": "multi_select"},
			"board_columns": []any{
				map[string]any{"property": "K{tS", "value": map[string]any{"type": "multi_select", "value": "group 1"}},
				map[string]any{"property": "K{tS", "value": map[string]any{"type": "multi_select"}},
			},
		},
	}, CollectionOptions{Limit: 999, SpaceID: "space-id"})
	if err != nil {
		t.Fatal(err)
	}

	loader := requestBody["loader"].(map[string]any)
	reducers := loader["reducers"].(map[string]any)
	for _, key := range []string{
		"board_columns",
		"board:multi_select:group 1",
		"results:multi_select:group 1",
		"board:multi_select:uncategorized",
		"results:multi_select:uncategorized",
	} {
		if reducers[key] == nil {
			t.Fatalf("missing reducer %q in %#v", key, reducers)
		}
	}
	boardColumns := reducers["board_columns"].(map[string]any)
	groupSortPreference := boardColumns["groupSortPreference"].([]any)
	uncategorized := groupSortPreference[1].(map[string]any)["value"].(map[string]any)
	if _, ok := uncategorized["value"]; ok {
		t.Fatalf("uncategorized groupSortPreference value should be omitted, got %#v", uncategorized)
	}
}

func TestGetPageFetchesCollectionsConcurrently(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	collectionBlock1ID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	collectionBlock2ID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	collection1ID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	collection2ID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	view1ID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	view2ID := "ffffffff-ffff-ffff-ffff-ffffffffffff"

	var activeQueryCollections int32
	var maxActiveQueryCollections int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loadPageChunk":
			_, _ = w.Write([]byte(`{
				"recordMap": {
					"block": {
						"` + rootID + `": {"value": {"id": "` + rootID + `", "type": "page", "content": ["` + collectionBlock1ID + `", "` + collectionBlock2ID + `"]}},
						"` + collectionBlock1ID + `": {"value": {"id": "` + collectionBlock1ID + `", "type": "collection_view", "collection_id": "` + collection1ID + `", "view_ids": ["` + view1ID + `"]}},
						"` + collectionBlock2ID + `": {"value": {"id": "` + collectionBlock2ID + `", "type": "collection_view", "collection_id": "` + collection2ID + `", "view_ids": ["` + view2ID + `"]}}
					},
					"collection": {},
					"collection_view": {
						"` + view1ID + `": {"value": {"id": "` + view1ID + `", "type": "table", "format": {}}},
						"` + view2ID + `": {"value": {"id": "` + view2ID + `", "type": "table", "format": {}}}
					}
				}
			}`))
		case "/queryCollection":
			current := atomic.AddInt32(&activeQueryCollections, 1)
			updateMaxInt32(&maxActiveQueryCollections, current)
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&activeQueryCollections, -1)
			_, _ = w.Write([]byte(`{"recordMap":{"block":{}},"result":{"reducerResults":{"collection_group_results":{"blockIds":[]}}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	if _, err := client.GetPage(context.Background(), rootID, PageOptions{FetchCollections: true, Concurrency: 2}); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&maxActiveQueryCollections); got < 2 {
		t.Fatalf("max concurrent queryCollection calls = %d, want at least 2", got)
	}
}

func TestGetPageRejectsMaxBlocksBeforeFetchingMissingBlocks(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	child1ID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	child2ID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	var syncRecordCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loadPageChunk":
			_, _ = w.Write([]byte(`{
				"recordMap": {
					"block": {
						"` + rootID + `": {"value": {"id": "` + rootID + `", "type": "page", "content": ["` + child1ID + `", "` + child2ID + `"]}}
					}
				}
			}`))
		case "/syncRecordValuesMain":
			atomic.AddInt32(&syncRecordCalls, 1)
			_, _ = w.Write([]byte(`{"recordMap":{"block":{}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	_, err := client.GetPage(context.Background(), rootID, PageOptions{FetchMissingBlocks: true, MaxBlocks: 1})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusRequestEntityTooLarge || httpErr.Code != ErrorCodeMaxBlocksExceeded {
		t.Fatalf("error = %#v", err)
	}
	if got := atomic.LoadInt32(&syncRecordCalls); got != 0 {
		t.Fatalf("syncRecordValuesMain calls = %d, want 0", got)
	}
}

func updateMaxInt32(target *int32, value int32) {
	for {
		current := atomic.LoadInt32(target)
		if value <= current || atomic.CompareAndSwapInt32(target, current, value) {
			return
		}
	}
}
