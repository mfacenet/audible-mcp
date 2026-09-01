package audible

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mfacenet/audible-mcp/internal/auth"
)

func TestListCollectionItemsURL(t *testing.T) {
	t.Parallel()
	api, seen := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"url": r.URL.String()})
	})
	raw, err := api.ListCollectionItems(context.Background(), "__FAVORITES")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "/1.0/collections/__FAVORITES/items") {
		t.Fatalf("body = %s", raw)
	}
	if !strings.Contains(seen.lastQuery, "response_groups=always-returned") && !strings.Contains(string(raw), "response_groups=always-returned") {
		if !strings.Contains(seen.lastURL, "response_groups=always-returned") {
			t.Fatalf("url = %s", seen.lastURL)
		}
	}
}

func TestGetLibraryItemResponseGroups(t *testing.T) {
	t.Parallel()
	api, seen := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"url": r.URL.String()})
	})
	if _, err := api.GetLibraryItem(context.Background(), "B0FVBC49CX"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen.lastURL, "/1.0/library/B0FVBC49CX") {
		t.Fatalf("url = %s", seen.lastURL)
	}
	if !strings.Contains(seen.lastURL, "contributors") || !strings.Contains(seen.lastURL, "media") {
		t.Fatalf("missing response groups: %s", seen.lastURL)
	}
}

func TestGetListeningStats(t *testing.T) {
	t.Parallel()
	api, seen := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"search": r.URL.RawQuery})
	})
	if _, err := api.GetListeningStats(context.Background(), ListeningStatsOptions{
		Months:     2,
		StartMonth: "2026-02",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen.lastURL, "monthly_listening_interval_duration=2") {
		t.Fatalf("url = %s", seen.lastURL)
	}
	if !strings.Contains(seen.lastURL, "monthly_listening_interval_start_date=2026-02") {
		t.Fatalf("url = %s", seen.lastURL)
	}
	if !strings.Contains(seen.lastURL, "store=Audible") {
		t.Fatalf("url = %s", seen.lastURL)
	}
}

func TestGetChapters(t *testing.T) {
	t.Parallel()
	api, _ := testAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content_metadata": map[string]any{
				"chapter_info": map[string]any{
					"chapters": []any{map[string]any{"title": "Chapter 1"}},
				},
			},
		})
	})
	got, err := api.GetChapters(context.Background(), "B0FVBC49CX")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	if !strings.Contains(string(raw), `"asin":"B0FVBC49CX"`) {
		t.Fatalf("got = %s", raw)
	}
	if !strings.Contains(string(raw), "Chapter 1") {
		t.Fatalf("got = %s", raw)
	}
}

func TestSearchLibrary(t *testing.T) {
	t.Parallel()
	api, seen := testAPI(t, func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "1" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{
					map[string]any{
						"asin":    "A1",
						"title":   "Nothing Relevant",
						"authors": []any{map[string]any{"name": "Someone Else"}},
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{
					"asin":    "A2",
					"title":   "The Mathematician's Mind",
					"authors": []any{map[string]any{"name": "Rebecca Goldstein"}},
				},
			},
		})
	})
	got, err := api.SearchLibrary(context.Background(), SearchLibraryOptions{
		Query:             "goldstein",
		MaxPages:          2,
		NumResultsPerPage: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen.calls != 2 {
		t.Fatalf("calls = %d", seen.calls)
	}
	raw, _ := json.Marshal(got)
	var parsed struct {
		Items []struct {
			ASIN string `json:"asin"`
		}
		TotalMatches int `json:"totalMatches"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.TotalMatches != 1 || parsed.Items[0].ASIN != "A2" {
		t.Fatalf("got = %s", raw)
	}
}

func TestListInProgressTitles(t *testing.T) {
	t.Parallel()
	api, _ := testAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"asin": "A1", "is_finished": false, "percent_complete": 42.5, "title": "In Progress"},
				map[string]any{"asin": "A2", "is_finished": true, "percent_complete": 100, "title": "Finished"},
			},
		})
	})
	got, err := api.ListInProgressTitles(context.Background(), InProgressOptions{MaxPages: 1, NumResultsPerPage: 25})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	var parsed struct {
		Items []struct {
			ASIN string `json:"asin"`
		}
		TotalMatches int `json:"totalMatches"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.TotalMatches != 1 || parsed.Items[0].ASIN != "A1" {
		t.Fatalf("got = %s", raw)
	}
}

func TestFromAuthFileRefreshesExpiredToken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "audible-auth.json")
	pemKey := testPEMKey(t)
	bundle := &auth.File{
		AccessToken:      "expired-access",
		AdpToken:         "adp-token",
		DevicePrivateKey: pemKey,
		Domain:           "com",
		ExpiresAt:        time.Now().Add(-time.Second).UnixMilli(),
		Locale:           "us",
		RefreshToken:     "refresh-token",
		Serial:           "SERIAL123",
	}
	if err := auth.Save(path, bundle); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/auth/token"):
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "source_token=refresh-token") {
				t.Errorf("refresh body = %s", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "fresh-access",
				"expires_in":   3600,
			})
		default:
			if r.Header.Get("x-adp-token") != "adp-token" {
				t.Errorf("x-adp-token = %s", r.Header.Get("x-adp-token"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{map[string]any{"asin": "A1", "title": "Fresh Title"}},
			})
		}
	}))
	t.Cleanup(server.Close)

	client := server.Client()
	client.Transport = rewriteHost(server.URL, client.Transport)

	api, err := FromAuthFile(Options{
		AuthFile:   path,
		BaseURL:    "https://api.audible.com",
		HTTPClient: client,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := api.ValidateAuth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	if !strings.Contains(string(raw), `"valid":true`) {
		t.Fatalf("got = %s", raw)
	}

	saved, err := auth.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.AccessToken != "fresh-access" {
		t.Fatalf("saved token = %q", saved.AccessToken)
	}
}

func TestDefaultStartMonth(t *testing.T) {
	t.Parallel()
	got := defaultStartMonth(time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC))
	if got != "2026-01" {
		t.Fatalf("got %q", got)
	}
}

type seenURLs struct {
	calls     int
	lastURL   string
	lastQuery string
}

func testAPI(t *testing.T, h http.HandlerFunc) (*API, *seenURLs) {
	t.Helper()
	seen := &seenURLs{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.calls++
		seen.lastURL = r.URL.String()
		seen.lastQuery = r.URL.RawQuery
		h(w, r)
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	httpClient.Transport = rewriteHost(server.URL, httpClient.Transport)
	client, err := NewClient("https://api.audible.com", "adp", testPEMKey(t), WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	return New(client), seen
}

func testPEMKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
}

type hostRewrite struct {
	target *url.URL
	base   http.RoundTripper
}

func rewriteHost(serverURL string, base http.RoundTripper) http.RoundTripper {
	u, err := url.Parse(serverURL)
	if err != nil {
		panic(err)
	}
	if base == nil {
		base = http.DefaultTransport
	}
	return hostRewrite{target: u, base: base}
}

func (h hostRewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = h.target.Scheme
	clone.URL.Host = h.target.Host
	clone.Host = h.target.Host
	return h.base.RoundTrip(clone)
}

func TestAuthFileRoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "audible-auth.json")
	f := &auth.File{
		AccessToken:      "a",
		AdpToken:         "adp",
		DevicePrivateKey: testPEMKey(t),
		Domain:           "com",
		ExpiresAt:        1,
		Locale:           "us",
		RefreshToken:     "r",
		Serial:           "S",
	}
	if err := auth.Save(path, f); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("auth file mode = %v", info.Mode())
	}
}
