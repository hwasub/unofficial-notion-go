package notionapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
