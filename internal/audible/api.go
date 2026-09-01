package audible

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mfacenet/audible-mcp/internal/auth"
)

const (
	defaultBaseURL        = "https://api.audible.com"
	authRefreshWindow     = 5 * time.Minute
	libraryGroups         = "contributors,media,product_attrs,product_desc,rating,series"
	libraryItemGroups     = libraryGroups + ",is_downloaded,is_finished,pdf_url,percent_complete"
	contentMetadataGroups = "chapter_info,content_reference"
	catalogGroups         = libraryGroups + ",sku"
)

// API is the authenticated Audible read surface used by the MCP server.
type API struct {
	client       *Client
	httpClient   *http.Client
	authFile     *auth.File
	authFilePath string
	baseURL      string
}

// Options configure FromAuthFile.
type Options struct {
	AuthFile   string
	BaseURL    string
	HTTPClient *http.Client
}

// New wraps an already-configured signed client. Token refresh is disabled
// unless the API was created with FromAuthFile.
func New(client *Client) *API {
	return &API{client: client}
}

// FromAuthFile loads a signed API client from an audible-auth.json bundle.
func FromAuthFile(opts Options) (*API, error) {
	file, err := auth.Load(opts.AuthFile)
	if err != nil {
		return nil, err
	}
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	api := &API{
		httpClient:   opts.HTTPClient,
		authFile:     file,
		authFilePath: opts.AuthFile,
		baseURL:      baseURL,
	}
	if err := api.rebuildClient(); err != nil {
		return nil, err
	}
	return api, nil
}

func (a *API) rebuildClient() error {
	var opts []ClientOption
	if a.httpClient != nil {
		opts = append(opts, WithHTTPClient(a.httpClient))
	}
	client, err := NewClient(a.baseURL, a.authFile.AdpToken, a.authFile.DevicePrivateKey, opts...)
	if err != nil {
		return err
	}
	a.client = client
	return nil
}

func (a *API) refreshAccessToken(ctx context.Context, force bool) error {
	if a.authFile == nil {
		return nil
	}
	if !force && time.Until(time.UnixMilli(a.authFile.ExpiresAt)) > authRefreshWindow {
		return nil
	}
	token, expiresAt, err := auth.RefreshAccessToken(ctx, a.httpClient, a.authFile)
	if err != nil {
		return err
	}
	a.authFile.AccessToken = token
	a.authFile.ExpiresAt = expiresAt
	if err := auth.Save(a.authFilePath, a.authFile); err != nil {
		return err
	}
	return a.rebuildClient()
}

func (a *API) getJSON(ctx context.Context, pathname string, query url.Values) (json.RawMessage, error) {
	if err := a.refreshAccessToken(ctx, false); err != nil {
		return nil, err
	}
	resp, err := a.client.Get(ctx, pathname, query)
	if err != nil {
		return nil, err
	}
	if (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) && a.authFile != nil {
		if err := a.refreshAccessToken(ctx, true); err != nil {
			return nil, err
		}
		resp, err = a.client.Get(ctx, pathname, query)
		if err != nil {
			return nil, err
		}
	}
	if !resp.OK() {
		return nil, fmt.Errorf("Audible API request failed with status %d: %s", resp.StatusCode, truncateBody(resp.Body))
	}
	if !json.Valid(resp.Body) {
		return json.RawMessage([]byte(fmt.Sprintf("%q", string(resp.Body)))), nil
	}
	return json.RawMessage(resp.Body), nil
}

// ListLibraryOptions is the input for ListLibrary.
type ListLibraryOptions struct {
	NumResults int
	Page       int
}

// ListLibrary returns a page of the authenticated library.
func (a *API) ListLibrary(ctx context.Context, opts ListLibraryOptions) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("num_results", fmt.Sprintf("%d", clamp(opts.NumResults, 25, 100)))
	q.Set("page", fmt.Sprintf("%d", clamp(opts.Page, 1, 1000)))
	q.Set("response_groups", libraryGroups)
	return a.getJSON(ctx, "/1.0/library", q)
}

// GetLibraryItem fetches a single library title by ASIN.
func (a *API) GetLibraryItem(ctx context.Context, asin string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("response_groups", libraryItemGroups)
	return a.getJSON(ctx, "/1.0/library/"+url.PathEscape(asin), q)
}

// ListWishlistOptions is the input for ListWishlist.
type ListWishlistOptions struct {
	NumResults int
	Page       int
}

// ListWishlist returns a page of the authenticated wishlist.
func (a *API) ListWishlist(ctx context.Context, opts ListWishlistOptions) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("num_results", fmt.Sprintf("%d", clamp(opts.NumResults, 25, 100)))
	q.Set("page", fmt.Sprintf("%d", clamp(opts.Page, 1, 1000)))
	q.Set("response_groups", libraryGroups)
	return a.getJSON(ctx, "/1.0/wishlist", q)
}

// ListCollections returns the account's collections.
func (a *API) ListCollections(ctx context.Context) (json.RawMessage, error) {
	return a.getJSON(ctx, "/1.0/collections", nil)
}

// ListCollectionItems returns items in a collection.
func (a *API) ListCollectionItems(ctx context.Context, collectionID string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("response_groups", "always-returned")
	return a.getJSON(ctx, "/1.0/collections/"+url.PathEscape(collectionID)+"/items", q)
}

// SearchLibraryOptions is the input for SearchLibrary.
type SearchLibraryOptions struct {
	Query             string
	MaxPages          int
	NumResultsPerPage int
}

// SearchLibrary scans early library pages and filters client-side.
func (a *API) SearchLibrary(ctx context.Context, opts SearchLibraryOptions) (any, error) {
	normalized := strings.ToLower(strings.TrimSpace(opts.Query))
	maxPages := clamp(opts.MaxPages, 3, 20)
	pageSize := clamp(opts.NumResultsPerPage, 25, 100)
	matches := make([]map[string]any, 0)
	seen := map[string]struct{}{}

	for page := 1; page <= maxPages; page++ {
		raw, err := a.ListLibrary(ctx, ListLibraryOptions{NumResults: pageSize, Page: page})
		if err != nil {
			return nil, err
		}
		items := decodeItems(raw)
		for _, item := range items {
			if !itemMatchesQuery(item, normalized) {
				continue
			}
			asin, _ := item["asin"].(string)
			if asin != "" {
				if _, ok := seen[asin]; ok {
					continue
				}
				seen[asin] = struct{}{}
			}
			matches = append(matches, item)
		}
		if len(items) < pageSize {
			break
		}
	}

	return map[string]any{
		"items":         matches,
		"query":         opts.Query,
		"searchedPages": maxPages,
		"totalMatches":  len(matches),
	}, nil
}

// InProgressOptions is the input for ListInProgressTitles.
type InProgressOptions struct {
	MaxPages          int
	NumResultsPerPage int
}

// ListInProgressTitles returns titles that have started and are not finished.
func (a *API) ListInProgressTitles(ctx context.Context, opts InProgressOptions) (any, error) {
	maxPages := clamp(opts.MaxPages, 3, 20)
	pageSize := clamp(opts.NumResultsPerPage, 25, 100)
	found := make([]map[string]any, 0)

	for page := 1; page <= maxPages; page++ {
		q := url.Values{}
		q.Set("num_results", fmt.Sprintf("%d", pageSize))
		q.Set("page", fmt.Sprintf("%d", page))
		q.Set("response_groups", libraryItemGroups)
		raw, err := a.getJSON(ctx, "/1.0/library", q)
		if err != nil {
			return nil, err
		}
		items := decodeItems(raw)
		for _, item := range items {
			if isInProgress(item) {
				found = append(found, item)
			}
		}
		if len(items) < pageSize {
			break
		}
	}

	return map[string]any{
		"items":        found,
		"scannedPages": maxPages,
		"totalMatches": len(found),
	}, nil
}

// GetContentMetadata fetches content metadata including chapters.
func (a *API) GetContentMetadata(ctx context.Context, asin string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("response_groups", contentMetadataGroups)
	return a.getJSON(ctx, "/1.0/content/"+url.PathEscape(asin)+"/metadata", q)
}

// GetChapters extracts chapter_info from content metadata.
func (a *API) GetChapters(ctx context.Context, asin string) (any, error) {
	raw, err := a.GetContentMetadata(ctx, asin)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		ContentMetadata struct {
			ChapterInfo json.RawMessage `json:"chapter_info"`
		} `json:"content_metadata"`
	}
	_ = json.Unmarshal(raw, &parsed)
	var chapterInfo any
	if len(parsed.ContentMetadata.ChapterInfo) > 0 && string(parsed.ContentMetadata.ChapterInfo) != "null" {
		if err := json.Unmarshal(parsed.ContentMetadata.ChapterInfo, &chapterInfo); err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"asin":         asin,
		"chapter_info": chapterInfo,
	}, nil
}

// GetCatalogProduct fetches catalog product metadata.
func (a *API) GetCatalogProduct(ctx context.Context, asin string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("response_groups", catalogGroups)
	return a.getJSON(ctx, "/1.0/catalog/products/"+url.PathEscape(asin), q)
}

// ListeningStatsOptions is the input for GetListeningStats.
type ListeningStatsOptions struct {
	Locale     string
	Months     int
	StartMonth string
	Store      string
}

// GetListeningStats fetches aggregate listening stats.
func (a *API) GetListeningStats(ctx context.Context, opts ListeningStatsOptions) (json.RawMessage, error) {
	locale := opts.Locale
	if locale == "" {
		locale = "en_US"
	}
	store := opts.Store
	if store == "" {
		store = "Audible"
	}
	start := opts.StartMonth
	if start == "" {
		start = defaultStartMonth(time.Now().UTC())
	}
	q := url.Values{}
	q.Set("locale", locale)
	q.Set("monthly_listening_interval_duration", fmt.Sprintf("%d", clamp(opts.Months, 3, 24)))
	q.Set("monthly_listening_interval_start_date", start)
	q.Set("response_groups", "total_listening_stats")
	q.Set("store", store)
	return a.getJSON(ctx, "/1.0/stats/aggregates", q)
}

// AuthStatus is local metadata about the loaded auth bundle.
func (a *API) AuthStatus() map[string]any {
	cookieNames := make([]string, 0, len(a.authFile.WebsiteCookies))
	for name := range a.authFile.WebsiteCookies {
		cookieNames = append(cookieNames, name)
	}
	return map[string]any{
		"authFile":           a.authFilePath,
		"deviceSerial":       a.authFile.Serial,
		"expiresAt":          time.UnixMilli(a.authFile.ExpiresAt).UTC().Format("2006-01-02T15:04:05.000Z"),
		"locale":             a.authFile.Locale,
		"marketplaceDomain":  a.authFile.Domain,
		"websiteCookieNames": cookieNames,
	}
}

// ValidateAuth refreshes tokens if needed and performs a tiny library read.
func (a *API) ValidateAuth(ctx context.Context) (any, error) {
	if err := a.refreshAccessToken(ctx, false); err != nil {
		return nil, err
	}
	raw, err := a.ListLibrary(ctx, ListLibraryOptions{NumResults: 1, Page: 1})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"expiresAt":          time.UnixMilli(a.authFile.ExpiresAt).UTC().Format("2006-01-02T15:04:05.000Z"),
		"sampleLibraryCount": len(decodeItems(raw)),
		"valid":              true,
	}, nil
}

func clamp(value, fallback, max int) int {
	if value == 0 {
		return fallback
	}
	if value < 1 {
		return 1
	}
	if value > max {
		return max
	}
	return value
}

func defaultStartMonth(now time.Time) string {
	shifted := now.AddDate(0, -2, 0)
	return fmt.Sprintf("%04d-%02d", shifted.Year(), shifted.Month())
}

func decodeItems(raw json.RawMessage) []map[string]any {
	var parsed struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil
	}
	return parsed.Items
}

func itemMatchesQuery(item map[string]any, q string) bool {
	if q == "" {
		return false
	}
	if containsFold(item["asin"], q) || containsFold(item["title"], q) {
		return true
	}
	for _, key := range []string{"authors", "narrators", "series"} {
		arr, ok := item[key].([]any)
		if !ok {
			continue
		}
		for _, entry := range arr {
			obj, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			if containsFold(obj["name"], q) {
				return true
			}
		}
	}
	return false
}

func containsFold(v any, q string) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(s)), q)
}

func isInProgress(item map[string]any) bool {
	if percent, ok := asFloat(item["percent_complete"]); ok && percent > 0 && percent < 100 {
		return true
	}
	finished, ok := item["is_finished"].(bool)
	return ok && !finished
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func truncateBody(body []byte) string {
	s := string(body)
	if len(s) > 1200 {
		return s[:1200]
	}
	return s
}
