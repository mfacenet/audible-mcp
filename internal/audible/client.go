package audible

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mfacenet/audible-mcp/internal/auth"
)

const defaultUserAgent = "audible-mcp/2.0.0"

// Client is an HTTP client that ADP-signs requests to api.audible.com.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	adpToken   string
	privateKey string
	userAgent  string
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithHTTPClient replaces the default HTTP client (useful in tests).
func WithHTTPClient(c *http.Client) ClientOption {
	return func(client *Client) {
		client.httpClient = c
	}
}

// WithUserAgent overrides the default User-Agent.
func WithUserAgent(ua string) ClientOption {
	return func(client *Client) {
		client.userAgent = ua
	}
}

// NewClient builds a signed Audible API client.
func NewClient(baseURL, adpToken, privateKeyPEM string, opts ...ClientOption) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Audible base URL: %w", err)
	}
	c := &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		baseURL:    parsed,
		adpToken:   adpToken,
		privateKey: privateKeyPEM,
		userAgent:  defaultUserAgent,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.httpClient == nil {
		return nil, fmt.Errorf("HTTP client is nil")
	}
	return c, nil
}

// Response is a raw Audible HTTP response.
type Response struct {
	Body       []byte
	StatusCode int
	URL        string
}

// OK reports whether the status is 2xx.
func (r Response) OK() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// Get issues a signed GET.
func (c *Client) Get(ctx context.Context, pathname string, query url.Values) (Response, error) {
	return c.do(ctx, http.MethodGet, pathname, query, nil)
}

func (c *Client) do(ctx context.Context, method, pathname string, query url.Values, body []byte) (Response, error) {
	u, err := c.resolve(pathname, query)
	if err != nil {
		return Response{}, err
	}

	var bodyText string
	var reader io.Reader
	if body != nil {
		bodyText = string(body)
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("user-agent", c.userAgent)
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}

	sig, err := auth.SignRequest(method, u, bodyText, c.adpToken, c.privateKey, time.Time{})
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("x-adp-alg", sig.Algorithm)
	req.Header.Set("x-adp-signature", sig.Value)
	req.Header.Set("x-adp-token", sig.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("audible request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read audible response: %w", err)
	}
	return Response{Body: raw, StatusCode: resp.StatusCode, URL: resp.Request.URL.String()}, nil
}

func (c *Client) resolve(pathname string, query url.Values) (*url.URL, error) {
	joined, err := url.JoinPath(strings.TrimRight(c.baseURL.String(), "/"), pathname)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(joined)
	if err != nil {
		return nil, err
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	return u, nil
}
