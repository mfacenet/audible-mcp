package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Registration is the subset of Amazon's device-registration response we persist.
type Registration struct {
	AccessToken               string
	AdpToken                  string
	CustomerInfo              map[string]any
	DeviceInfo                map[string]any
	DevicePrivateKey          string
	ExpiresAt                 int64
	RefreshToken              string
	StoreAuthenticationCookie map[string]any
	WebsiteCookies            map[string]string
}

type registerRequest struct {
	AuthData struct {
		AuthorizationCode string `json:"authorization_code"`
		ClientDomain      string `json:"client_domain"`
		ClientID          string `json:"client_id"`
		CodeAlgorithm     string `json:"code_algorithm"`
		CodeVerifier      string `json:"code_verifier"`
	} `json:"auth_data"`
	Cookies struct {
		Domain         string `json:"domain"`
		WebsiteCookies []any  `json:"website_cookies"`
	} `json:"cookies"`
	RegistrationData struct {
		AppName         string `json:"app_name"`
		AppVersion      string `json:"app_version"`
		DeviceModel     string `json:"device_model"`
		DeviceName      string `json:"device_name"`
		DeviceSerial    string `json:"device_serial"`
		DeviceType      string `json:"device_type"`
		Domain          string `json:"domain"`
		OSVersion       string `json:"os_version"`
		SoftwareVersion string `json:"software_version"`
	} `json:"registration_data"`
	RequestedExtensions []string `json:"requested_extensions"`
	RequestedTokenType  []string `json:"requested_token_type"`
}

type registerResponse struct {
	Response struct {
		Success *struct {
			Extensions struct {
				CustomerInfo map[string]any `json:"customer_info"`
				DeviceInfo   map[string]any `json:"device_info"`
			} `json:"extensions"`
			Tokens struct {
				Bearer *struct {
					AccessToken  string `json:"access_token"`
					ExpiresIn    any    `json:"expires_in"`
					RefreshToken string `json:"refresh_token"`
				} `json:"bearer"`
				MacDMS *struct {
					AdpToken         string `json:"adp_token"`
					DevicePrivateKey string `json:"device_private_key"`
				} `json:"mac_dms"`
				StoreAuthenticationCookie map[string]any  `json:"store_authentication_cookie"`
				WebsiteCookies            json.RawMessage `json:"website_cookies"`
			} `json:"tokens"`
		} `json:"success"`
	} `json:"response"`
}

type namedCookie struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

// RegisterDevice exchanges an authorization code for ADP credentials.
func RegisterDevice(ctx context.Context, httpClient *http.Client, session Session, authorizationCode string) (Registration, error) {
	if httpClient == nil {
		httpClient = defaultHTTPClient()
	}

	var body registerRequest
	body.AuthData.AuthorizationCode = authorizationCode
	body.AuthData.ClientDomain = "DeviceLegacy"
	body.AuthData.ClientID = ClientID(session.Serial)
	body.AuthData.CodeAlgorithm = "SHA-256"
	body.AuthData.CodeVerifier = session.CodeVerifier
	body.Cookies.Domain = ".amazon." + session.Marketplace.Domain
	body.Cookies.WebsiteCookies = []any{}
	body.RegistrationData.AppName = "Audible"
	body.RegistrationData.AppVersion = audibleIOSAppVersion
	body.RegistrationData.DeviceModel = "iPhone"
	body.RegistrationData.DeviceName = "%FIRST_NAME%%FIRST_NAME_POSSESSIVE_STRING%%DUPE_STRATEGY_1ST%Audible for iPhone"
	body.RegistrationData.DeviceSerial = session.Serial
	body.RegistrationData.DeviceType = audibleIOSDeviceType
	body.RegistrationData.Domain = "Device"
	body.RegistrationData.OSVersion = "15.0.0"
	body.RegistrationData.SoftwareVersion = audibleIOSSoftwareVersion
	body.RequestedExtensions = []string{"device_info", "customer_info"}
	body.RequestedTokenType = []string{"bearer", "mac_dms", "website_cookies", "store_authentication_cookie"}

	payload, err := json.Marshal(body)
	if err != nil {
		return Registration{}, err
	}

	target := "amazon"
	if session.WithUsername {
		target = "audible"
	}
	endpoint := fmt.Sprintf("https://api.%s.%s/auth/register", target, session.Marketplace.Domain)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Registration{}, err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return Registration{}, fmt.Errorf("device registration request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Registration{}, fmt.Errorf("read registration response: %w", err)
	}

	var parsed registerResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Registration{}, fmt.Errorf("expected JSON registration response but received: %s", truncate(string(raw), 240))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Registration{}, fmt.Errorf("device registration failed with %d: %s", resp.StatusCode, string(raw))
	}

	success := parsed.Response.Success
	if success == nil || success.Tokens.Bearer == nil || success.Tokens.MacDMS == nil {
		return Registration{}, fmt.Errorf("device registration response was missing required fields")
	}

	expiresIn, err := intFromAny(success.Tokens.Bearer.ExpiresIn)
	if err != nil {
		return Registration{}, fmt.Errorf("registration expires_in: %w", err)
	}

	return Registration{
		AccessToken:               success.Tokens.Bearer.AccessToken,
		AdpToken:                  success.Tokens.MacDMS.AdpToken,
		CustomerInfo:              success.Extensions.CustomerInfo,
		DeviceInfo:                success.Extensions.DeviceInfo,
		DevicePrivateKey:          success.Tokens.MacDMS.DevicePrivateKey,
		ExpiresAt:                 time.Now().UnixMilli() + int64(expiresIn)*1000,
		RefreshToken:              success.Tokens.Bearer.RefreshToken,
		StoreAuthenticationCookie: success.Tokens.StoreAuthenticationCookie,
		WebsiteCookies:            normalizeCookies(success.Tokens.WebsiteCookies),
	}, nil
}

// Bundle builds a persistable auth file from a completed login + registration.
func Bundle(session Session, registration Registration) *File {
	return &File{
		AccessToken:               registration.AccessToken,
		AdpToken:                  registration.AdpToken,
		CustomerInfo:              registration.CustomerInfo,
		DeviceInfo:                registration.DeviceInfo,
		DevicePrivateKey:          registration.DevicePrivateKey,
		Domain:                    session.Marketplace.Domain,
		ExpiresAt:                 registration.ExpiresAt,
		Locale:                    session.Marketplace.Code,
		RefreshToken:              registration.RefreshToken,
		Serial:                    session.Serial,
		StoreAuthenticationCookie: registration.StoreAuthenticationCookie,
		WebsiteCookies:            registration.WebsiteCookies,
		WithUsername:              session.WithUsername,
	}
}

func normalizeCookies(raw json.RawMessage) map[string]string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var list []namedCookie
	if err := json.Unmarshal(raw, &list); err == nil {
		return cookieMap(list)
	}

	var grouped map[string][]namedCookie
	if err := json.Unmarshal(raw, &grouped); err == nil {
		out := make([]namedCookie, 0)
		for _, cookies := range grouped {
			out = append(out, cookies...)
		}
		return cookieMap(out)
	}
	return nil
}

func cookieMap(cookies []namedCookie) map[string]string {
	if len(cookies) == 0 {
		return nil
	}
	out := make(map[string]string, len(cookies))
	for _, c := range cookies {
		out[c.Name] = strings.ReplaceAll(c.Value, `"`, "")
	}
	return out
}

func intFromAny(v any) (int, error) {
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case json.Number:
		i, err := n.Int64()
		return int(i), err
	case string:
		var i int
		_, err := fmt.Sscan(n, &i)
		return i, err
	case nil:
		return 0, fmt.Errorf("missing")
	default:
		return 0, fmt.Errorf("unsupported type %T", v)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}
