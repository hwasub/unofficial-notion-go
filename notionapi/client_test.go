package notionapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hwasub/unofficial-notion-go/internal/notionrecordmap"
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

func TestAddSignedURLsSignsRoleAndRichTextAssetsAcrossAllBlocks(t *testing.T) {
	pageID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	imageID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	cover := "attachment:page-cover.png"
	icon := "attachment:page-icon.png"
	propertyFile := "attachment:property-file.pdf"
	image := "attachment:image.png"
	var requested []SignedURLRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URLs []SignedURLRequest `json:"urls"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requested = append(requested, body.URLs...)
		signed := make([]string, len(body.URLs))
		for i := range body.URLs {
			signed[i] = fmt.Sprintf("https://file.notion.so/signed/%d", i)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"signedUrls": signed})
	}))
	defer server.Close()

	recordMap := map[string]any{"block": map[string]any{
		pageID: map[string]any{"value": map[string]any{
			"id":   pageID,
			"type": "page",
			"format": map[string]any{
				"page_cover": cover,
				"page_icon":  icon,
			},
			"properties": map[string]any{
				"title": []any{[]any{"Root"}},
				"files": []any{[]any{"File", []any{[]any{"a", propertyFile}}}},
			},
		}},
		imageID: map[string]any{"value": map[string]any{
			"id":         imageID,
			"type":       "image",
			"properties": map[string]any{"source": []any{[]any{image}}},
		}},
	}}
	client := New(WithAPIBaseURL(server.URL))
	if err := client.AddSignedURLs(context.Background(), recordMap, nil); err != nil {
		t.Fatal(err)
	}
	if len(requested) != 4 {
		t.Fatalf("signed URL requests = %#v, want four distinct assets", requested)
	}
	signed := notionrecordmap.AsMap(recordMap["signed_urls"])
	for _, key := range []string{
		pageID,
		pageID + ":cover",
		pageID + ":icon",
		cover,
		icon,
		propertyFile,
		imageID,
		image,
	} {
		if notionrecordmap.StringValue(signed[key]) == "" {
			t.Errorf("signed_urls missing key %q: %#v", key, signed)
		}
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

func TestGetCollectionDataForwardsViewSorts(t *testing.T) {
	sortSpec := []any{map[string]any{"property": "K{tS", "direction": "descending"}}
	cases := []struct {
		name string
		view map[string]any
		want []any
	}{
		{
			name: "query2 sort",
			view: map[string]any{"type": "table", "format": map[string]any{}, "query2": map[string]any{"sort": sortSpec}},
			want: sortSpec,
		},
		{
			name: "legacy query sort",
			view: map[string]any{"type": "table", "format": map[string]any{}, "query": map[string]any{"sort": sortSpec}},
			want: sortSpec,
		},
		{
			name: "grouped board keeps sort",
			view: map[string]any{
				"type": "board",
				"format": map[string]any{
					"board_columns_by": map[string]any{"property": "K{tS", "type": "select"},
					"board_columns":    []any{map[string]any{"property": "K{tS", "value": map[string]any{"type": "select", "value": "Ready"}}},
				},
				"query2": map[string]any{"sort": sortSpec},
			},
			want: sortSpec,
		},
		{
			name: "no sorts defaults to empty",
			view: map[string]any{"type": "table", "format": map[string]any{}},
			want: []any{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requestBody map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
					t.Error(err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"result":{"reducerResults":{}}}`))
			}))
			defer server.Close()

			client := New(WithAPIBaseURL(server.URL))
			if _, err := client.GetCollectionData(context.Background(), "collection-id", "view-id", tc.view, CollectionOptions{}); err != nil {
				t.Fatal(err)
			}
			loader := requestBody["loader"].(map[string]any)
			if !reflect.DeepEqual(loader["sort"], tc.want) {
				t.Fatalf("loader sort = %#v, want %#v", loader["sort"], tc.want)
			}
		})
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

func TestGetPageUnboxesNestedCollectionViewSettings(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	collectionBlockID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	collectionID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	viewID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	sortSpec := []any{map[string]any{"property": "priority", "direction": "descending"}}

	var queryBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loadPageChunk":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"recordMap": map[string]any{
					"block": map[string]any{
						rootID: map[string]any{"value": map[string]any{
							"id": rootID, "type": "page", "content": []any{collectionBlockID},
						}},
						collectionBlockID: map[string]any{"value": map[string]any{
							"id": collectionBlockID, "type": "collection_view",
							"collection_id": collectionID, "view_ids": []any{viewID},
						}},
					},
					"collection": map[string]any{},
					"collection_view": map[string]any{
						viewID: map[string]any{"value": map[string]any{"value": map[string]any{
							"id": viewID, "type": "board",
							"format": map[string]any{
								"board_columns_by": "status",
								"board_columns": []any{
									map[string]any{
										"property": "status",
										"value":    map[string]any{"type": "select", "value": "Todo"},
									},
								},
							},
							"query2": map[string]any{"sort": sortSpec},
						}}},
					},
				},
			})
		case "/queryCollection":
			if err := json.NewDecoder(r.Body).Decode(&queryBody); err != nil {
				t.Error(err)
			}
			_, _ = w.Write([]byte(`{"recordMap":{"block":{}},"result":{"reducerResults":{}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	if _, err := client.GetPage(context.Background(), rootID, PageOptions{FetchCollections: true}); err != nil {
		t.Fatal(err)
	}
	loader := notionrecordmap.AsMap(queryBody["loader"])
	if !reflect.DeepEqual(loader["sort"], sortSpec) {
		t.Fatalf("loader sort = %#v, want %#v", loader["sort"], sortSpec)
	}
	reducers := notionrecordmap.AsMap(loader["reducers"])
	if _, ok := reducers["board_columns"]; !ok {
		t.Fatalf("loader reducers = %#v, want board_columns reducer", reducers)
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

func TestGetPageBatchesMissingBlockRequests(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	children := []string{
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1",
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2",
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa3",
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa4",
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa5",
	}
	var batchSizes []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loadPageChunk":
			_ = json.NewEncoder(w).Encode(map[string]any{"recordMap": map[string]any{"block": map[string]any{
				rootID: map[string]any{"value": map[string]any{"id": rootID, "type": "page", "content": children}},
			}}})
		case "/syncRecordValuesMain":
			var request struct {
				Requests []struct {
					ID string `json:"id"`
				} `json:"requests"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			batchSizes = append(batchSizes, len(request.Requests))
			blocks := map[string]any{}
			for _, item := range request.Requests {
				blocks[item.ID] = map[string]any{"value": map[string]any{"id": item.ID, "type": "text"}}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"recordMap": map[string]any{"block": blocks}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	recordMap, err := client.GetPage(context.Background(), rootID, PageOptions{FetchMissingBlocks: true, ChunkLimit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(notionrecordmap.AsMap(recordMap["block"])); got != 1+len(children) {
		t.Fatalf("block count = %d, want %d", got, 1+len(children))
	}
	if !reflect.DeepEqual(batchSizes, []int{2, 2, 1}) {
		t.Fatalf("batch sizes = %#v, want [2 2 1]", batchSizes)
	}
}

func TestGetPageRejectsUnresolvedMissingBlocks(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	childID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loadPageChunk":
			_ = json.NewEncoder(w).Encode(map[string]any{"recordMap": map[string]any{"block": map[string]any{
				rootID: map[string]any{"value": map[string]any{"id": rootID, "type": "page", "content": []string{childID}}},
			}}})
		case "/syncRecordValuesMain":
			_, _ = w.Write([]byte(`{"recordMap":{"block":{}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	_, err := client.GetPage(context.Background(), rootID, PageOptions{FetchMissingBlocks: true})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadGateway || httpErr.Code != ErrorCodeMissingBlocks {
		t.Fatalf("error = %#v, want %s HTTPError", err, ErrorCodeMissingBlocks)
	}
}

func TestGetPageRequiresRequestedRootBlock(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	otherID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"recordMap": map[string]any{"block": map[string]any{
			otherID: map[string]any{"value": map[string]any{"id": otherID, "type": "page"}},
		}}})
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	_, err := client.GetPage(context.Background(), rootID, PageOptions{})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusNotFound || httpErr.Code != ErrorCodePageNotFound {
		t.Fatalf("error = %#v, want %s HTTPError", err, ErrorCodePageNotFound)
	}
}

// Redirect responses must surface as errors instead of being followed, so the
// token_v2 cookie is never re-sent to a redirect target.
func TestFetchDoesNotFollowRedirects(t *testing.T) {
	var redirectTargetHits int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&redirectTargetHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/loadPageChunk", http.StatusFound)
	}))
	defer origin.Close()

	client := New(WithAPIBaseURL(origin.URL), WithAuthToken("secret"))
	_, err := client.Fetch(context.Background(), "loadPageChunk", map[string]any{}, nil, nil)
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error = %#v, want *HTTPError", err)
	}
	if httpErr.StatusCode != http.StatusFound {
		t.Fatalf("status code = %d, want %d", httpErr.StatusCode, http.StatusFound)
	}
	if got := atomic.LoadInt32(&redirectTargetHits); got != 0 {
		t.Fatalf("redirect target hits = %d, want 0", got)
	}
}

func TestFetchRejectsUnsafeAPIBaseURL(t *testing.T) {
	for _, raw := range []string{
		"http://example.com/api/v3",
		"ftp://example.com/api/v3",
		"https://user:pass@example.com/api/v3",
		"https://example.com/api/v3?target=other",
	} {
		client := New(WithAPIBaseURL(raw))
		if _, err := client.Fetch(context.Background(), "loadPageChunk", map[string]any{}, nil, nil); err == nil {
			t.Errorf("Fetch accepted unsafe API base URL %q", raw)
		}
	}
}

func TestCustomHTTPClientCannotEnableRedirects(t *testing.T) {
	var targetHits int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&targetHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer origin.Close()

	client := New(WithAPIBaseURL(origin.URL), WithAuthToken("secret"), WithHTTPClient(&http.Client{}))
	_, _ = client.Fetch(context.Background(), "loadPageChunk", map[string]any{}, nil, nil)
	if atomic.LoadInt32(&targetHits) != 0 {
		t.Fatal("custom HTTP client followed a redirect")
	}
}

func TestDefaultHTTPClientIgnoresAmbientProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:9999")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:9999")
	transport, ok := defaultHTTPClient().Transport.(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Fatalf("default Notion client uses ambient proxy: %#v", transport)
	}
}

func TestFetchRejectsNonJSONContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html>not json</html>`))
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	_, err := client.Fetch(context.Background(), "loadPageChunk", map[string]any{}, nil, nil)
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error = %#v, want *HTTPError", err)
	}
	if httpErr.Code != ErrorCodeUnexpectedContentType {
		t.Fatalf("error code = %q", httpErr.Code)
	}
}

// A missing Content-Type header is tolerated (some proxies strip it); only an
// explicit non-JSON type is rejected.
func TestFetchAllowsMissingContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header()["Content-Type"] = nil // suppress Go's automatic detection
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	out, err := client.Fetch(context.Background(), "loadPageChunk", map[string]any{}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("out = %#v", out)
	}
}

func TestFetchRejectsDeeplyNestedResponse(t *testing.T) {
	body := `{"a":` + strings.Repeat("[", 200) + "1" + strings.Repeat("]", 200) + `}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	_, err := client.Fetch(context.Background(), "loadPageChunk", map[string]any{}, nil, nil)
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error = %#v, want *HTTPError", err)
	}
	if httpErr.Code != ErrorCodeMalformedResponse {
		t.Fatalf("error code = %q", httpErr.Code)
	}
}

func TestFetchRejectsOversizedArrayResponse(t *testing.T) {
	body := `{"a":[` + strings.TrimRight(strings.Repeat("0,", maxDecodedArrayLen+1), ",") + `]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	_, err := client.Fetch(context.Background(), "loadPageChunk", map[string]any{}, nil, nil)
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error = %#v, want *HTTPError", err)
	}
	if httpErr.Code != ErrorCodeMalformedResponse {
		t.Fatalf("error code = %q", httpErr.Code)
	}
}

func TestFetchTruncatesLargeErrorMessage(t *testing.T) {
	body := strings.Repeat("x", maxErrorMessageBytes*2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	_, err := client.Fetch(context.Background(), "loadPageChunk", map[string]any{}, nil, nil)
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error = %#v, want *HTTPError", err)
	}
	if httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", httpErr.StatusCode)
	}
	if !strings.HasSuffix(httpErr.Message, "(truncated)") || len(httpErr.Message) > maxErrorMessageBytes+len("…(truncated)") {
		t.Fatalf("error message not truncated as expected: %d bytes", len(httpErr.Message))
	}
}

func TestFetchRejectsTrailingData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true} trailing`))
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	_, err := client.Fetch(context.Background(), "loadPageChunk", map[string]any{}, nil, nil)
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error = %#v, want *HTTPError", err)
	}
	if httpErr.Code != ErrorCodeMalformedResponse {
		t.Fatalf("error code = %q", httpErr.Code)
	}
}

// A trailing newline or whitespace after the JSON body is tolerated; only extra
// non-whitespace data is rejected.
func TestFetchAllowsTrailingWhitespace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"ok\":true}\n\t "))
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	out, err := client.Fetch(context.Background(), "loadPageChunk", map[string]any{}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("out = %#v", out)
	}
}

func TestFetchAllowsTypicalNesting(t *testing.T) {
	body := `{"a":` + strings.Repeat(`{"b":`, 30) + "1" + strings.Repeat("}", 30) + `}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	if _, err := client.Fetch(context.Background(), "loadPageChunk", map[string]any{}, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Cancellation between pagination iterations must stop GetPage before it asks
// the upstream for the next batch of missing blocks.
func TestGetPageStopsFetchingMissingBlocksOnCancel(t *testing.T) {
	rootID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	child1ID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	child2ID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var syncRecordCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loadPageChunk":
			_, _ = w.Write([]byte(`{
				"recordMap": {
					"block": {
						"` + rootID + `": {"value": {"id": "` + rootID + `", "type": "page", "content": ["` + child1ID + `"]}}
					}
				}
			}`))
		case "/syncRecordValuesMain":
			atomic.AddInt32(&syncRecordCalls, 1)
			// Cancel before answering: the returned block references another
			// missing child, so without the loop-top ctx check GetPage would
			// issue a second syncRecordValuesMain call.
			cancel()
			_, _ = w.Write([]byte(`{"recordMap":{"block":{
				"` + child1ID + `": {"value": {"id": "` + child1ID + `", "type": "text", "content": ["` + child2ID + `"]}}
			}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL))
	_, err := client.GetPage(ctx, rootID, PageOptions{FetchMissingBlocks: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %#v, want context.Canceled", err)
	}
	if got := atomic.LoadInt32(&syncRecordCalls); got != 1 {
		t.Fatalf("syncRecordValuesMain calls = %d, want 1", got)
	}
}

// Once the context is done, collection workers must drain their remaining jobs
// without issuing further upstream calls.
func TestFetchCollectionsSkipsJobsAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var queryCollectionCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&queryCollectionCalls, 1)
		cancel()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"reducerResults":{}}}`))
	}))
	defer server.Close()

	client := New(WithAPIBaseURL(server.URL)).normalized()
	recordMap := map[string]any{"collection_view": map[string]any{}}
	instances := []collectionInstance{
		{CollectionID: "c1", ViewID: "v1"},
		{CollectionID: "c2", ViewID: "v2"},
		{CollectionID: "c3", ViewID: "v3"},
	}
	results := client.fetchCollections(ctx, recordMap, instances, PageOptions{Concurrency: 1})
	if got := atomic.LoadInt32(&queryCollectionCalls); got != 1 {
		t.Fatalf("queryCollection calls = %d, want 1", got)
	}
	for i, result := range results[1:] {
		if !errors.Is(result.Err, context.Canceled) {
			t.Fatalf("results[%d].Err = %#v, want context.Canceled", i+1, result.Err)
		}
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
