package notionapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hwasub/unofficial-notion-go/internal/notionid"
	"github.com/hwasub/unofficial-notion-go/internal/notionrecordmap"
)

// defaultMaxResponseBytes caps how many bytes Fetch will read from a single
// Notion response body. It guards against unbounded memory growth from a
// malicious or malfunctioning upstream.
const defaultMaxResponseBytes = 15 << 20

// maxErrorMessageBytes bounds how much of an upstream error body is retained in
// an HTTPError.Message, so a large non-JSON error response cannot bloat the
// returned error (and any logs that include it).
const maxErrorMessageBytes = 4 << 10

// truncateErrorMessage bounds an upstream error message to maxErrorMessageBytes,
// trimming any partial trailing UTF-8 rune introduced by the cut.
func truncateErrorMessage(message string) string {
	if len(message) <= maxErrorMessageBytes {
		return message
	}
	return strings.ToValidUTF8(message[:maxErrorMessageBytes], "") + "…(truncated)"
}

// Structural limits Fetch enforces on decoded responses. Real Notion record
// maps nest well under 30 levels and the largest arrays (blockIds) stay far
// below the hard block caps, so these bounds only reject hostile or corrupt
// payloads before they reach recursive consumers downstream.
const (
	maxDecodedDepth    = 128
	maxDecodedArrayLen = 100_000
)

// rejectRedirects refuses to follow any redirect. Fetch surfaces the 3xx
// response as an *HTTPError instead, so the token_v2 cookie is never re-sent
// to a redirect target. Notion's /api/v3 POST endpoints do not legitimately
// redirect.
func rejectRedirects(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

// Client talks to the private Notion API. Construct it with New; the zero value
// is usable but New is preferred because it installs sane defaults. All fields
// are private: configure the client through the With* Option functions.
type Client struct {
	apiBaseURL       string
	authToken        string
	activeUser       string
	userTimeZone     string
	httpClient       *http.Client
	maxResponseBytes int64
}

// Option configures a Client. Pass any number of options to New.
type Option func(*Client)

// WithAPIBaseURL sets the Notion API base URL (default
// "https://www.notion.so/api/v3"). A trailing "/" is trimmed.
func WithAPIBaseURL(value string) Option {
	return func(c *Client) { c.apiBaseURL = value }
}

// WithAuthToken sets the token_v2 cookie value used to authenticate requests.
func WithAuthToken(value string) Option {
	return func(c *Client) { c.authToken = value }
}

// WithActiveUser sets the x-notion-active-user-header value.
func WithActiveUser(value string) Option {
	return func(c *Client) { c.activeUser = value }
}

// WithUserTimeZone sets the default user time zone used for collection queries
// (default "America/New_York").
func WithUserTimeZone(value string) Option {
	return func(c *Client) { c.userTimeZone = value }
}

// WithHTTPClient overrides the *http.Client used for requests. A nil value is
// ignored, leaving the default client in place. The client is used as-is,
// including its redirect policy: set CheckRedirect (for example to return
// http.ErrUseLastResponse, as the default client does) so the token_v2 cookie
// is never re-sent to a redirect target.
func WithHTTPClient(value *http.Client) Option {
	return func(c *Client) {
		if value != nil {
			c.httpClient = value
		}
	}
}

// WithMaxResponseBytes caps how many bytes are read from a single response body
// before Fetch returns an error. Non-positive values fall back to
// defaultMaxResponseBytes.
func WithMaxResponseBytes(value int64) Option {
	return func(c *Client) { c.maxResponseBytes = value }
}

// PageOptions controls how GetPage assembles a page's record map. The zero value
// is valid: GetPage fills in sensible defaults for the numeric fields and treats
// the boolean fields as opt-in steps.
type PageOptions struct {
	Concurrency             int  // maximum concurrent upstream requests (defaults to 3)
	FetchMissingBlocks      bool // page through loadPageChunk until every content block is present
	FetchCollections        bool // resolve collection (database) data for embedded collection views
	SignFileURLs            bool // resolve signed download URLs for file-bearing blocks
	ChunkLimit              int  // blocks requested per loadPageChunk call (defaults to 100)
	ChunkNumber             int  // starting chunk index for the initial loadPageChunk call
	ThrowOnCollectionErrors bool // return on the first collection error instead of skipping it
	CollectionReducerLimit  int  // maximum rows requested per collection reducer (defaults to 999)
	FetchRelationPages      bool // reserved: fetch pages referenced by relation properties
	MaxBlocks               int  // optional maximum block records allowed while assembling the page
}

// CollectionOptions controls a single GetCollectionData query. The zero value is
// valid: GetCollectionData fills in defaults for Limit and UserTimeZone and
// enables LoadContentCover.
type CollectionOptions struct {
	Limit            int    // maximum rows requested per reducer (defaults to 999)
	SearchQuery      string // optional full-text search applied to the collection
	UserTimeZone     string // IANA time zone for date filters (defaults to the client's zone)
	LoadContentCover bool   // include row cover images in the results
	SpaceID          string // workspace ID sent via the x-notion-space-id header
}

// SignedURLRequest names a single file URL to sign together with the record that
// grants access to it. GetSignedFileURLs takes a slice of these.
type SignedURLRequest struct {
	PermissionRecord PermissionRecord `json:"permissionRecord"` // record whose permissions authorize the file
	URL              string           `json:"url"`              // raw (unsigned) file URL to resolve
}

// PermissionRecord identifies the Notion record that authorizes access to a file,
// usually the block that embeds it.
type PermissionRecord struct {
	Table string `json:"table"` // record table, typically "block"
	ID    string `json:"id"`    // record ID within the table
}

// SignedURLResponse holds the signed download URLs returned by GetSignedFileURLs,
// in the same order as the requests.
type SignedURLResponse struct {
	SignedURLs []string `json:"signedUrls"`
}

// New returns a Client with sane defaults, then applies opts in order. The
// resulting client is ready to use; defaults are guaranteed even if no options
// are supplied.
func New(opts ...Option) *Client {
	c := &Client{
		apiBaseURL:       "https://www.notion.so/api/v3",
		userTimeZone:     "America/New_York",
		httpClient:       &http.Client{Timeout: 60 * time.Second, CheckRedirect: rejectRedirects},
		maxResponseBytes: defaultMaxResponseBytes,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.apiBaseURL = strings.TrimRight(c.apiBaseURL, "/")
	return c
}

// normalized returns a copy of c with any zero-value field filled in, so that a
// manually constructed, zero-value, or nil Client behaves like one returned by
// New. It never mutates the receiver, keeping a shared *Client safe for
// concurrent use. Each request method calls it and operates on the result.
func (c *Client) normalized() *Client {
	if c == nil {
		return New()
	}
	out := *c
	if strings.TrimSpace(out.apiBaseURL) == "" {
		out.apiBaseURL = "https://www.notion.so/api/v3"
	}
	out.apiBaseURL = strings.TrimRight(out.apiBaseURL, "/")
	if strings.TrimSpace(out.userTimeZone) == "" {
		out.userTimeZone = "America/New_York"
	}
	if out.httpClient == nil {
		out.httpClient = &http.Client{Timeout: 60 * time.Second, CheckRedirect: rejectRedirects}
	}
	if out.maxResponseBytes <= 0 {
		out.maxResponseBytes = defaultMaxResponseBytes
	}
	return &out
}

// GetPage fetches a page by ID and returns its assembled record map. It calls
// GetPageRaw for the initial chunk, then, according to opts, pages through any
// missing content blocks, resolves embedded collection data, and signs file
// URLs. The returned map always contains the "block", "collection",
// "collection_view", "notion_user", "collection_query", and "signed_urls" keys.
// It returns an error if the page is not found or any required upstream call
// fails.
func (c *Client) GetPage(ctx context.Context, pageID string, opts PageOptions) (map[string]any, error) {
	c = c.normalized()
	rootPageID := notionid.ParsePageIDForAPI(pageID, true)
	if rootPageID == "" {
		rootPageID = pageID
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 3
	}
	if opts.ChunkLimit <= 0 {
		opts.ChunkLimit = 100
	}
	if opts.CollectionReducerLimit <= 0 {
		opts.CollectionReducerLimit = 999
	}
	page, err := c.GetPageRaw(ctx, pageID, opts.ChunkLimit, opts.ChunkNumber)
	if err != nil {
		return nil, err
	}
	recordMap := notionrecordmap.AsMap(page["recordMap"])
	if notionrecordmap.AsMap(recordMap["block"]) == nil {
		return nil, fmt.Errorf("notion page not found: %q", notionid.UUIDToID(pageID))
	}
	ensureMap(recordMap, "collection")
	ensureMap(recordMap, "collection_view")
	ensureMap(recordMap, "notion_user")
	recordMap["collection_query"] = map[string]any{}
	recordMap["signed_urls"] = map[string]any{}
	if err := enforceMaxBlocks(recordMap, opts.MaxBlocks); err != nil {
		return nil, err
	}

	contentBlockIDs := notionrecordmap.GetPageContentBlockIDs(recordMap, rootPageID)
	if opts.FetchMissingBlocks {
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			blocks := notionrecordmap.AsMap(recordMap["block"])
			pending := []string{}
			for _, id := range contentBlockIDs {
				if _, ok := blocks[id]; !ok {
					pending = append(pending, id)
				}
			}
			if len(pending) == 0 {
				break
			}
			if opts.MaxBlocks > 0 && len(blocks)+len(pending) > opts.MaxBlocks {
				return nil, maxBlocksExceededError(opts.MaxBlocks)
			}
			chunk, err := c.GetBlocks(ctx, pending)
			if err != nil {
				return nil, err
			}
			before := len(blocks)
			newBlocks := notionrecordmap.AsMap(notionrecordmap.AsMap(chunk["recordMap"])["block"])
			if err := enforceMapMergeMaxBlocks(blocks, newBlocks, opts.MaxBlocks); err != nil {
				return nil, err
			}
			mergeMap(blocks, newBlocks)
			if len(blocks) == before {
				// The upstream returned none of the pending blocks (not found,
				// access revoked, or keyed differently). Stop rather than
				// re-requesting the same set forever.
				break
			}
			// New blocks may reference further content; the traversal is only
			// recomputed here, after a merge actually changed the block table.
			contentBlockIDs = notionrecordmap.GetPageContentBlockIDs(recordMap, rootPageID)
		}
	}

	if opts.FetchCollections {
		instances := collectionInstances(recordMap, contentBlockIDs)
		results := c.fetchCollections(ctx, recordMap, instances, opts)
		for _, result := range results {
			if result.Err != nil {
				if opts.ThrowOnCollectionErrors {
					return nil, result.Err
				}
				continue
			}
			sourceRecordMap := notionrecordmap.AsMap(result.Data["recordMap"])
			if err := enforceRecordMapMergeMaxBlocks(recordMap, sourceRecordMap, opts.MaxBlocks); err != nil {
				return nil, err
			}
			mergeRecordMap(recordMap, sourceRecordMap)
			resultMap := notionrecordmap.AsMap(result.Data["result"])
			reducerResults := resultMap["reducerResults"]
			if reducerResults != nil {
				collectionQuery := ensureMap(recordMap, "collection_query")
				byCollection := notionrecordmap.AsMap(collectionQuery[result.Instance.CollectionID])
				if byCollection == nil {
					byCollection = map[string]any{}
					collectionQuery[result.Instance.CollectionID] = byCollection
				}
				byCollection[result.Instance.ViewID] = reducerResults
			}
		}
	}

	if opts.SignFileURLs {
		if err := c.AddSignedURLs(ctx, recordMap, contentBlockIDs); err != nil {
			return nil, err
		}
	}
	return recordMap, nil
}

// GetPageRaw fetches a single loadPageChunk for the given page and returns the
// raw decoded response, including its "recordMap" and cursor. chunkLimit bounds
// the number of blocks returned and chunkNumber selects the starting chunk. It
// returns an error if pageID cannot be parsed or the upstream call fails.
func (c *Client) GetPageRaw(ctx context.Context, pageID string, chunkLimit int, chunkNumber int) (map[string]any, error) {
	c = c.normalized()
	parsed := notionid.ParsePageIDForAPI(pageID, true)
	if parsed == "" {
		return nil, fmt.Errorf("invalid notion pageId %q", pageID)
	}
	body := map[string]any{
		"pageId":          parsed,
		"limit":           chunkLimit,
		"chunkNumber":     chunkNumber,
		"cursor":          map[string]any{"stack": []any{}},
		"verticalColumns": false,
	}
	return c.Fetch(ctx, "loadPageChunk", body, nil, nil)
}

// GetCollectionData queries a collection (database) through the queryCollection
// endpoint and returns the raw decoded response. collectionView is the view's
// record value; GetCollectionData inspects its type, format, and query2 fields to
// build the reducer that fetches results, applying any grouping (board columns or
// group-by) and filters defined by the view. opts tunes the row limit, search
// query, time zone, cover loading, and workspace ID. It returns an error if the
// upstream call fails.
func (c *Client) GetCollectionData(ctx context.Context, collectionID string, collectionViewID string, collectionView any, opts CollectionOptions) (map[string]any, error) {
	c = c.normalized()
	if opts.Limit <= 0 {
		opts.Limit = 999
	}
	if opts.UserTimeZone == "" {
		opts.UserTimeZone = c.userTimeZone
	}
	if !opts.LoadContentCover {
		// The TS default is true; callers can still pass false by constructing
		// reducers manually in the future. v1 always uses the default.
		opts.LoadContentCover = true
	}
	view := notionrecordmap.AsMap(collectionView)
	viewType := notionrecordmap.StringValue(view["type"])
	isBoardType := viewType == "board"
	format := notionrecordmap.AsMap(view["format"])
	query2 := notionrecordmap.AsMap(view["query2"])
	var groupBy any
	if isBoardType {
		groupBy = format["board_columns_by"]
	} else {
		groupBy = format["collection_group_by"]
	}

	filters := []any{}
	if propertyFilters := notionrecordmap.AsSlice(format["property_filters"]); len(propertyFilters) > 0 {
		for _, filterValue := range propertyFilters {
			filterObject := notionrecordmap.AsMap(filterValue)
			filter := notionrecordmap.AsMap(filterObject["filter"])
			filters = append(filters, map[string]any{
				"filter":   filter["filter"],
				"property": filter["property"],
			})
		}
	}
	if filter := notionrecordmap.AsMap(query2["filter"]); filter != nil {
		if queryFilters := notionrecordmap.AsSlice(filter["filters"]); len(queryFilters) > 0 {
			filters = append(filters, queryFilters...)
		}
	}

	sorts := collectionViewSorts(view, query2)
	loader := map[string]any{
		"type": "reducer",
		"reducers": map[string]any{
			"collection_group_results": map[string]any{
				"type":             "results",
				"limit":            opts.Limit,
				"loadContentCover": opts.LoadContentCover,
			},
		},
		"filter":       map[string]any{"filters": filters, "operator": "and"},
		"searchQuery":  opts.SearchQuery,
		"userTimeZone": opts.UserTimeZone,
	}
	// query2 may overwrite "filter" here; that mirrors the upstream TS
	// reference implementation and is intentionally left unchanged.
	mergeMap(loader, query2)
	loader["sort"] = sorts

	if groupBy != nil {
		groups := notionrecordmap.AsSlice(format["board_columns"])
		if len(groups) == 0 {
			groups = notionrecordmap.AsSlice(format["collection_groups"])
		}
		reducers := map[string]any{}
		for _, groupValue := range groups {
			group := notionrecordmap.AsMap(groupValue)
			valueObject := notionrecordmap.AsMap(group["value"])
			property := group["property"]
			value := valueObject["value"]
			valueType := notionrecordmap.StringValue(valueObject["type"])
			for _, iterator := range []string{boardIterator(isBoardType), "results"} {
				iteratorProps := map[string]any{}
				if iterator == "results" {
					iteratorProps["type"] = iterator
					iteratorProps["limit"] = opts.Limit
				} else {
					iteratorProps["type"] = "aggregation"
					iteratorProps["aggregation"] = map[string]any{"aggregator": "count"}
				}
				isUncategorized := value == nil
				isDate := notionrecordmap.AsMap(value) != nil && notionrecordmap.AsMap(notionrecordmap.AsMap(value)["range"]) != nil
				queryLabel := "uncategorized"
				var queryValue any
				if !isUncategorized {
					queryValue = value
					if isDate {
						rng := notionrecordmap.AsMap(notionrecordmap.AsMap(value)["range"])
						queryLabel = firstNonEmpty(notionrecordmap.StringValue(rng["start_date"]), notionrecordmap.StringValue(rng["end_date"]))
					} else if valueMap := notionrecordmap.AsMap(value); valueMap != nil && valueMap["value"] != nil {
						queryLabel = notionrecordmap.LooseStringValue(valueMap["value"])
						queryValue = valueMap["value"]
					} else {
						queryLabel = notionrecordmap.LooseStringValue(value)
					}
				}
				operator := "is_empty"
				if !isUncategorized {
					operator = collectionOperator(valueType)
				}
				filter := map[string]any{
					"operator": operator,
				}
				if !isUncategorized {
					filter["value"] = map[string]any{"type": "exact", "value": queryValue}
				}
				reducers[iterator+":"+valueType+":"+queryLabel] = mergeCopy(iteratorProps, map[string]any{
					"filter": map[string]any{
						"operator": "and",
						"filters": []any{map[string]any{
							"property": property,
							"filter":   filter,
						}},
					},
				})
			}
		}
		reducerLabel := viewType + "_groups"
		if isBoardType {
			reducerLabel = "board_columns"
		}
		groupSortPreference := []any{}
		for _, groupValue := range groups {
			group := notionrecordmap.AsMap(groupValue)
			valueObject := notionrecordmap.AsMap(group["value"])
			sortValue := map[string]any{}
			if value, ok := valueObject["type"]; ok {
				sortValue["type"] = value
			}
			if value, ok := valueObject["value"]; ok {
				sortValue["value"] = value
			}
			groupSortPreference = append(groupSortPreference, map[string]any{
				"property": group["property"],
				"value":    sortValue,
			})
		}
		reducers[reducerLabel] = map[string]any{
			"type":                "groups",
			"version":             "v2",
			"groupBy":             groupBy,
			"groupSortPreference": groupSortPreference,
			"limit":               opts.Limit,
		}
		if filter := notionrecordmap.AsMap(query2["filter"]); filter != nil {
			notionrecordmap.AsMap(reducers[reducerLabel])["filter"] = filter
		}
		loader = map[string]any{
			"type":         "reducer",
			"reducers":     reducers,
			"searchQuery":  opts.SearchQuery,
			"userTimeZone": opts.UserTimeZone,
			"filter":       map[string]any{"filters": filters, "operator": "and"},
		}
		mergeMap(loader, query2)
		loader["sort"] = sorts
	}

	headers := map[string]string{}
	if opts.SpaceID != "" {
		headers["x-notion-space-id"] = opts.SpaceID
	}
	query := url.Values{"src": []string{"initial_load"}}
	return c.Fetch(ctx, "queryCollection", map[string]any{
		"collection":     map[string]any{"id": collectionID},
		"collectionView": map[string]any{"id": collectionViewID},
		"source":         map[string]any{"type": "collection", "id": collectionID},
		"loader":         loader,
	}, headers, query)
}

// GetBlocks fetches the current values of the given block IDs through the
// syncRecordValuesMain endpoint and returns the raw decoded response, whose
// "recordMap" contains the requested blocks. It returns an error if the upstream
// call fails.
func (c *Client) GetBlocks(ctx context.Context, blockIDs []string) (map[string]any, error) {
	c = c.normalized()
	requests := make([]any, 0, len(blockIDs))
	for _, id := range blockIDs {
		requests = append(requests, map[string]any{"table": "block", "id": id, "version": -1})
	}
	return c.Fetch(ctx, "syncRecordValuesMain", map[string]any{"requests": requests}, nil, nil)
}

// GetSignedFileURLs resolves signed download URLs for the given file requests
// through the getSignedFileUrls endpoint. The returned SignedURLResponse lists
// the signed URLs in the same order as urls. It returns an error if the upstream
// call fails.
func (c *Client) GetSignedFileURLs(ctx context.Context, urls []SignedURLRequest) (*SignedURLResponse, error) {
	c = c.normalized()
	requests := make([]any, 0, len(urls))
	for _, item := range urls {
		requests = append(requests, item)
	}
	out, err := c.Fetch(ctx, "getSignedFileUrls", map[string]any{"urls": requests}, nil, nil)
	if err != nil {
		return nil, err
	}
	response := &SignedURLResponse{}
	for _, value := range notionrecordmap.AsSlice(out["signedUrls"]) {
		response.SignedURLs = append(response.SignedURLs, notionrecordmap.StringValue(value))
	}
	return response, nil
}

// AddSignedURLs resolves signed download URLs for any file-bearing blocks in
// recordMap and stores them under recordMap["signed_urls"], keyed by block ID.
//
// Contract: the method is honest about failure. If no blocks reference signable
// files it resets recordMap["signed_urls"] to an empty map and returns nil. If
// the upstream getSignedFileUrls call fails, the error is returned wrapped with
// context. If the response count does not match the number of requested files
// the method returns an error rather than silently producing partial results.
// Callers that prefer best-effort behavior should treat the returned error as a
// warning rather than a hard failure.
func (c *Client) AddSignedURLs(ctx context.Context, recordMap map[string]any, contentBlockIDs []string) error {
	c = c.normalized()
	recordMap["signed_urls"] = map[string]any{}
	if len(contentBlockIDs) == 0 {
		contentBlockIDs = notionrecordmap.GetPageContentBlockIDs(recordMap, "")
	}
	blocks := notionrecordmap.AsMap(recordMap["block"])
	allFileInstances := []SignedURLRequest{}
	for _, blockID := range contentBlockIDs {
		block := notionrecordmap.GetBlockValue(blocks[blockID])
		if block == nil {
			continue
		}
		blockType := notionrecordmap.StringValue(block["type"])
		if !signedBlockType(blockType, block) {
			continue
		}
		source := ""
		if blockType == "page" {
			source = notionrecordmap.StringValue(notionrecordmap.AsMap(block["format"])["page_cover"])
		} else {
			source = firstPropertyText(notionrecordmap.AsMap(block["properties"])["source"])
		}
		if source == "" {
			continue
		}
		if strings.Contains(source, "secure.notion-static.com") ||
			strings.Contains(source, "prod-files-secure") ||
			strings.Contains(source, "attachment:") {
			allFileInstances = append(allFileInstances, SignedURLRequest{
				PermissionRecord: PermissionRecord{Table: "block", ID: notionrecordmap.StringValue(block["id"])},
				URL:              source,
			})
		}
	}
	if len(allFileInstances) == 0 {
		return nil
	}
	response, err := c.GetSignedFileURLs(ctx, allFileInstances)
	if err != nil {
		return fmt.Errorf("get signed file urls: %w", err)
	}
	if len(response.SignedURLs) != len(allFileInstances) {
		return fmt.Errorf("notion signed url count mismatch: got %d, want %d", len(response.SignedURLs), len(allFileInstances))
	}
	signed := notionrecordmap.AsMap(recordMap["signed_urls"])
	for i, file := range allFileInstances {
		if response.SignedURLs[i] == "" || file.PermissionRecord.ID == "" {
			continue
		}
		signed[file.PermissionRecord.ID] = response.SignedURLs[i]
	}
	return nil
}

// Fetch performs a POST to the named Notion API endpoint with the given JSON
// body and returns the decoded response. extraHeaders and query are optional.
// Fetch attaches the configured auth token and active-user headers, caps the
// response body at the client's maxResponseBytes, and decodes JSON numbers
// losslessly via json.Number. A non-2xx status, an oversized body, or a decode
// failure is returned as an error; non-2xx responses are wrapped in *HTTPError
// with any parsed upstream message and Retry-After delay. Successful responses
// must carry a JSON Content-Type and stay within structural limits
// (maxDecodedDepth nesting levels, maxDecodedArrayLen items per array);
// violations are returned as *HTTPError with a machine-readable Code.
func (c *Client) Fetch(ctx context.Context, endpoint string, body map[string]any, extraHeaders map[string]string, query url.Values) (map[string]any, error) {
	c = c.normalized()
	endpointURL := c.apiBaseURL + "/" + strings.TrimLeft(endpoint, "/")
	if len(query) > 0 {
		endpointURL += "?" + query.Encode()
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range extraHeaders {
		req.Header.Set(key, value)
	}
	if c.authToken != "" {
		req.Header.Set("Cookie", "token_v2="+c.authToken)
	}
	if c.activeUser != "" {
		req.Header.Set("x-notion-active-user-header", c.activeUser)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// normalized guarantees maxResponseBytes > 0. Read one extra byte so we can
	// distinguish "exactly at the limit" from "over the limit".
	data, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > c.maxResponseBytes {
		return nil, &HTTPError{
			StatusCode: http.StatusRequestEntityTooLarge,
			Code:       ErrorCodeMaxResponseBytesExceeded,
			Message:    "notion response exceeded max response bytes",
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(data))
		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err == nil {
			for _, key := range []string{"message", "error", "name"} {
				if value := notionrecordmap.StringValue(parsed[key]); value != "" {
					message = value
					break
				}
			}
		}
		return nil, &HTTPError{
			StatusCode:   resp.StatusCode,
			Message:      truncateErrorMessage(message),
			RetryAfterMS: RetryAfterToMS(resp.Header.Get("Retry-After"), time.Now()),
		}
	}
	// Checked after the non-2xx branch on purpose: upstream error bodies are
	// parsed for messages regardless of their declared type. A missing header
	// is tolerated (some proxies strip it); only an explicit non-JSON type is
	// rejected — the JSON decode below still guards the headerless case.
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		mediaType, _, _ := mime.ParseMediaType(contentType)
		if mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json") {
			return nil, &HTTPError{
				StatusCode: http.StatusBadGateway,
				Code:       ErrorCodeUnexpectedContentType,
				Message:    fmt.Sprintf("notion response content-type %q is not JSON", mediaType),
			}
		}
	}
	var out map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&out); err != nil {
		return nil, err
	}
	// Reject responses with extra data after the first JSON value (e.g.
	// `{"ok":true} trailing`). A well-formed body decodes exactly one value, so
	// the next Decode must report EOF; anything else is a malformed response.
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return nil, &HTTPError{
			StatusCode: http.StatusBadGateway,
			Code:       ErrorCodeMalformedResponse,
			Message:    "notion response contains trailing data after the JSON body",
		}
	}
	if err := validateDecodedStructure(out); err != nil {
		return nil, err
	}
	return out, nil
}

// validateDecodedStructure rejects decoded responses that nest deeper than
// maxDecodedDepth or carry arrays longer than maxDecodedArrayLen. Downstream
// consumers (Scrub, GetPageContentBlockIDs) walk record maps recursively, so a
// hostile payload could otherwise overflow their stacks. The walk itself is
// iterative for the same reason.
func validateDecodedStructure(value any) error {
	type frame struct {
		value any
		depth int
	}
	stack := []frame{{value: value, depth: 1}}
	for len(stack) > 0 {
		next := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		var children []any
		switch typed := next.value.(type) {
		case map[string]any:
			children = make([]any, 0, len(typed))
			for _, child := range typed {
				children = append(children, child)
			}
		case []any:
			if len(typed) > maxDecodedArrayLen {
				return &HTTPError{
					StatusCode: http.StatusBadGateway,
					Code:       ErrorCodeMalformedResponse,
					Message:    fmt.Sprintf("notion response array exceeds %d items", maxDecodedArrayLen),
				}
			}
			children = typed
		default:
			continue
		}
		if len(children) > 0 && next.depth >= maxDecodedDepth {
			return &HTTPError{
				StatusCode: http.StatusBadGateway,
				Code:       ErrorCodeMalformedResponse,
				Message:    fmt.Sprintf("notion response nests deeper than %d levels", maxDecodedDepth),
			}
		}
		for _, child := range children {
			stack = append(stack, frame{value: child, depth: next.depth + 1})
		}
	}
	return nil
}

type collectionInstance struct {
	CollectionID string
	ViewID       string
	SpaceID      string
}

type collectionFetchResult struct {
	Instance collectionInstance
	Data     map[string]any
	Err      error
}

func (c *Client) fetchCollections(ctx context.Context, recordMap map[string]any, instances []collectionInstance, opts PageOptions) []collectionFetchResult {
	results := make([]collectionFetchResult, len(instances))
	if len(instances) == 0 {
		return results
	}
	workerCount := opts.Concurrency
	if workerCount <= 0 {
		workerCount = 1
	}
	if workerCount > len(instances) {
		workerCount = len(instances)
	}

	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				instance := instances[index]
				// Keep draining the channel (an early return would deadlock the
				// unbuffered producer below) but stop issuing upstream calls once
				// the caller's context is done.
				if err := ctx.Err(); err != nil {
					results[index] = collectionFetchResult{Instance: instance, Err: err}
					continue
				}
				collectionViews := notionrecordmap.AsMap(recordMap["collection_view"])
				collectionView := mapValue(collectionViews[instance.ViewID], "value")
				data, err := c.GetCollectionData(ctx, instance.CollectionID, instance.ViewID, collectionView, CollectionOptions{
					Limit:   opts.CollectionReducerLimit,
					SpaceID: instance.SpaceID,
				})
				results[index] = collectionFetchResult{
					Instance: instance,
					Data:     data,
					Err:      err,
				}
			}
		}()
	}
	for index := range instances {
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	return results
}

// collectionViewSorts extracts the view's sort specs in the shape the
// queryCollection loader expects ([{property, direction}], passed through
// verbatim). Modern views store them in query2.sort; legacy views in
// query.sort. Setting them explicitly after the query2 merge covers both
// shapes and the grouped branch, whose loader otherwise carries no sort.
func collectionViewSorts(view map[string]any, query2 map[string]any) []any {
	sorts := notionrecordmap.AsSlice(query2["sort"])
	if len(sorts) == 0 {
		sorts = notionrecordmap.AsSlice(notionrecordmap.AsMap(view["query"])["sort"])
	}
	if sorts == nil {
		sorts = []any{}
	}
	return sorts
}

func collectionInstances(recordMap map[string]any, contentBlockIDs []string) []collectionInstance {
	blocks := notionrecordmap.AsMap(recordMap["block"])
	out := []collectionInstance{}
	for _, id := range contentBlockIDs {
		block := notionrecordmap.GetBlockValue(blocks[id])
		if block == nil {
			continue
		}
		blockType := notionrecordmap.StringValue(block["type"])
		if blockType != "collection_view" && blockType != "collection_view_page" {
			continue
		}
		collectionID := notionrecordmap.GetBlockCollectionID(block, recordMap)
		if collectionID == "" {
			continue
		}
		for _, viewID := range notionrecordmap.AsSlice(block["view_ids"]) {
			if viewID := notionrecordmap.StringValue(viewID); viewID != "" {
				out = append(out, collectionInstance{
					CollectionID: collectionID,
					ViewID:       viewID,
					SpaceID:      notionrecordmap.StringValue(block["space_id"]),
				})
			}
		}
	}
	return out
}

func ensureMap(parent map[string]any, key string) map[string]any {
	current := notionrecordmap.AsMap(parent[key])
	if current != nil {
		return current
	}
	current = map[string]any{}
	parent[key] = current
	return current
}

func mapValue(value any, key string) any {
	record := notionrecordmap.AsMap(value)
	if record == nil {
		return nil
	}
	return record[key]
}

func mergeMap(target map[string]any, source map[string]any) {
	for key, value := range source {
		target[key] = value
	}
}

func mergeCopy(a map[string]any, b map[string]any) map[string]any {
	out := map[string]any{}
	mergeMap(out, a)
	mergeMap(out, b)
	return out
}

func mergeRecordMap(target map[string]any, source map[string]any) {
	if source == nil {
		return
	}
	for _, key := range []string{"block", "collection", "collection_view", "notion_user"} {
		sourceMap := notionrecordmap.AsMap(source[key])
		if sourceMap == nil {
			continue
		}
		targetMap := ensureMap(target, key)
		mergeMap(targetMap, sourceMap)
	}
}

func enforceMaxBlocks(recordMap map[string]any, maxBlocks int) error {
	if maxBlocks <= 0 {
		return nil
	}
	if len(notionrecordmap.AsMap(recordMap["block"])) > maxBlocks {
		return maxBlocksExceededError(maxBlocks)
	}
	return nil
}

func enforceRecordMapMergeMaxBlocks(target map[string]any, source map[string]any, maxBlocks int) error {
	if maxBlocks <= 0 || source == nil {
		return nil
	}
	return enforceMapMergeMaxBlocks(notionrecordmap.AsMap(target["block"]), notionrecordmap.AsMap(source["block"]), maxBlocks)
}

func enforceMapMergeMaxBlocks(target map[string]any, source map[string]any, maxBlocks int) error {
	if maxBlocks <= 0 || source == nil {
		return nil
	}
	count := len(target)
	for key := range source {
		if target == nil || target[key] == nil {
			count++
		}
		if count > maxBlocks {
			return maxBlocksExceededError(maxBlocks)
		}
	}
	return nil
}

func maxBlocksExceededError(maxBlocks int) *HTTPError {
	return &HTTPError{
		StatusCode: http.StatusRequestEntityTooLarge,
		Code:       ErrorCodeMaxBlocksExceeded,
		Message:    fmt.Sprintf("max block limit exceeded (%d)", maxBlocks),
	}
}

func boardIterator(isBoardType bool) string {
	if isBoardType {
		return "board"
	}
	return "group_aggregation"
}

func collectionOperator(valueType string) string {
	switch valueType {
	case "checkbox":
		return "checkbox_is"
	case "url", "text":
		return "string_starts_with"
	case "select":
		return "enum_is"
	case "multi_select":
		return "enum_contains"
	case "created_time":
		return "date_is_within"
	default:
		return "is_empty"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func signedBlockType(blockType string, block map[string]any) bool {
	switch blockType {
	case "pdf", "audio", "video", "file", "page":
		return true
	case "image":
		return len(notionrecordmap.AsSlice(block["file_ids"])) > 0
	default:
		return false
	}
}

func firstPropertyText(value any) string {
	parts := notionrecordmap.AsSlice(value)
	if len(parts) == 0 {
		return ""
	}
	first := notionrecordmap.AsSlice(parts[0])
	if len(first) == 0 {
		return ""
	}
	return notionrecordmap.StringValue(first[0])
}
