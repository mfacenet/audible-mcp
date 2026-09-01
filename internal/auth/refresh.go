package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type tokenRefreshResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   any    `json:"expires_in"`
}

type cookieRefreshResponse struct {
	Response struct {
		Tokens struct {
			Cookies json.RawMessage `json:"cookies"`
		} `json:"tokens"`
	} `json:"response"`
}

// RefreshAccessToken rotates the bearer access token using the stored refresh token.
func RefreshAccessToken(ctx context.Context, httpClient *http.Client, f *File) (accessToken string, expiresAt int64, err error) {
	if httpClient == nil {
		httpClient = defaultHTTPClient()
	}

	form := url.Values{}
	form.Set("app_name", "Audible")
	form.Set("app_version", audibleIOSAppVersion)
	form.Set("requested_token_type", "access_token")
	form.Set("source_token", f.RefreshToken)
	form.Set("source_token_type", "refresh_token")

	endpoint := fmt.Sprintf("https://api.%s/auth/token", f.apiHost())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("access token refresh request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("read token refresh response: %w", err)
	}

	var parsed tokenRefreshResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", 0, fmt.Errorf("expected JSON token refresh response but received: %s", truncate(string(raw), 240))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, fmt.Errorf("access token refresh failed with %d: %s", resp.StatusCode, string(raw))
	}

	expiresIn, err := intFromAny(parsed.ExpiresIn)
	if err != nil {
		return "", 0, fmt.Errorf("token refresh expires_in: %w", err)
	}
	return parsed.AccessToken, time.Now().UnixMilli() + int64(expiresIn)*1000, nil
}

// RefreshWebsiteCookies rotates Amazon website cookies for the registered device.
func RefreshWebsiteCookies(ctx context.Context, httpClient *http.Client, f *File, cookiesDomain string) (map[string]string, error) {
	if httpClient == nil {
		httpClient = defaultHTTPClient()
	}

	target := "amazon"
	if f.WithUsername {
		target = "audible"
	}

	form := url.Values{}
	form.Set("app_name", "Audible")
	form.Set("app_version", audibleIOSAppVersion)
	form.Set("domain", fmt.Sprintf(".%s.%s", target, cookiesDomain))
	form.Set("requested_token_type", "auth_cookies")
	form.Set("source_token", f.RefreshToken)
	form.Set("source_token_type", "refresh_token")

	endpoint := fmt.Sprintf("https://www.%s/ap/exchangetoken/cookies", f.apiHost())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("website cookie refresh request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read cookie refresh response: %w", err)
	}

	var parsed cookieRefreshResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("expected JSON cookie refresh response but received: %s", truncate(string(raw), 240))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("website cookie refresh failed with %d: %s", resp.StatusCode, string(raw))
	}

	cookies := normalizeCookies(parsed.Response.Tokens.Cookies)
	if cookies == nil {
		cookies = map[string]string{}
	}
	return cookies, nil
}
