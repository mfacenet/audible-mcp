package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mfacenet/audible-mcp/internal/marketplace"
)

func TestClientID(t *testing.T) {
	t.Parallel()
	want := hex.EncodeToString([]byte("ABC123#" + audibleIOSDeviceType))
	if got := ClientID("ABC123"); got != want {
		t.Fatalf("ClientID = %q, want %q", got, want)
	}
}

func TestNewSessionUS(t *testing.T) {
	t.Parallel()
	m, err := marketplace.Lookup("us")
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewSession(m, WithSerial("ABC123"))
	if err != nil {
		t.Fatal(err)
	}
	if session.Serial != "ABC123" {
		t.Fatalf("serial = %q", session.Serial)
	}
	if !strings.Contains(session.LoginURL, "openid.oa2.code_challenge_method=S256") {
		t.Fatalf("missing PKCE method: %s", session.LoginURL)
	}
	if !strings.Contains(session.LoginURL, "marketPlaceId=AF2M0KC94RCEA") {
		t.Fatalf("missing marketplace: %s", session.LoginURL)
	}
	if !strings.Contains(session.LoginURL, "device%3A"+ClientID("ABC123")) {
		t.Fatalf("missing client id: %s", session.LoginURL)
	}
}

func TestNewSessionUsernameRestricted(t *testing.T) {
	t.Parallel()
	m, err := marketplace.Lookup("fr")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewSession(m, WithAudibleUsername()); err == nil {
		t.Fatal("expected username restriction error")
	}
}

func TestAuthorizationCode(t *testing.T) {
	t.Parallel()
	code, err := AuthorizationCode("https://www.amazon.com/ap/maplanding?openid.oa2.authorization_code=abc123")
	if err != nil {
		t.Fatal(err)
	}
	if code != "abc123" {
		t.Fatalf("code = %q", code)
	}
	if _, err := AuthorizationCode("https://www.amazon.com/ap/maplanding"); err == nil {
		t.Fatal("expected missing-code error")
	}
}

func TestRegisterDeviceNormalizesCookies(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/register" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if ct := r.Header.Get("content-type"); ct != "application/json" {
			t.Errorf("content-type = %s", ct)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = io.WriteString(w, `{
			"response": {
				"success": {
					"extensions": {
						"customer_info": {"id": "customer"},
						"device_info": {"serial": "ABC123"}
					},
					"tokens": {
						"bearer": {
							"access_token": "access",
							"expires_in": 3600,
							"refresh_token": "refresh"
						},
						"mac_dms": {
							"adp_token": "adp",
							"device_private_key": "pem"
						},
						"website_cookies": [{"Name": "at-main", "Value": "\"cookie\""}]
					}
				}
			}
		}`)
	}))
	t.Cleanup(server.Close)

	m, _ := marketplace.Lookup("us")
	session, err := NewSession(m, WithSerial("ABC123"))
	if err != nil {
		t.Fatal(err)
	}

	client := server.Client()
	client.Transport = rewriteHost(server.URL, client.Transport)

	reg, err := RegisterDevice(context.Background(), client, session, "code-123")
	if err != nil {
		t.Fatal(err)
	}
	if reg.AccessToken != "access" {
		t.Fatalf("access = %q", reg.AccessToken)
	}
	if reg.WebsiteCookies["at-main"] != "cookie" {
		t.Fatalf("cookie = %#v", reg.WebsiteCookies)
	}

	file := Bundle(session, reg)
	if file.Serial != "ABC123" || file.Locale != "us" {
		t.Fatalf("bundle = %#v", file)
	}
}

func TestRefreshAccessTokenRequestShape(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/token" {
			t.Errorf("path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		if form.Get("requested_token_type") != "access_token" {
			t.Errorf("form = %s", body)
		}
		if form.Get("source_token") != "refresh" {
			t.Errorf("source_token = %s", form.Get("source_token"))
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-access",
			"expires_in":   1200,
		})
	}))
	t.Cleanup(server.Close)

	client := server.Client()
	client.Transport = rewriteHost(server.URL, client.Transport)

	token, expiresAt, err := RefreshAccessToken(context.Background(), client, &File{
		AccessToken:      "old",
		AdpToken:         "adp",
		DevicePrivateKey: "pem",
		Domain:           "com",
		RefreshToken:     "refresh",
		Serial:           "SERIAL",
	})
	if err != nil {
		t.Fatal(err)
	}
	if token != "new-access" {
		t.Fatalf("token = %q", token)
	}
	if expiresAt <= time.Now().UnixMilli() {
		t.Fatalf("expiresAt = %d", expiresAt)
	}
}

func TestRefreshWebsiteCookies(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ap/exchangetoken/cookies" {
			t.Errorf("path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		if form.Get("requested_token_type") != "auth_cookies" {
			t.Errorf("form = %s", body)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = io.WriteString(w, `{
			"response": {
				"tokens": {
					"cookies": {
						".amazon.com": [{"Name": "sess-at-main", "Value": "\"cookie-value\""}]
					}
				}
			}
		}`)
	}))
	t.Cleanup(server.Close)

	client := server.Client()
	client.Transport = rewriteHost(server.URL, client.Transport)

	cookies, err := RefreshWebsiteCookies(context.Background(), client, &File{
		AccessToken:      "a",
		AdpToken:         "adp",
		DevicePrivateKey: "pem",
		Domain:           "com",
		RefreshToken:     "refresh",
		Serial:           "SERIAL",
	}, "com")
	if err != nil {
		t.Fatal(err)
	}
	if cookies["sess-at-main"] != "cookie-value" {
		t.Fatalf("cookies = %#v", cookies)
	}
}

func TestSignRequestRSA(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	u, err := url.Parse("https://api.audible.com/1.0/library?page=2")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 9, 1, 2, 3, 0, time.UTC)
	sig, err := SignRequest(http.MethodPost, u, `{"hello":"world"}`, "adp-token", string(pemKey), now)
	if err != nil {
		t.Fatal(err)
	}
	if sig.Algorithm != "SHA256withRSA:1.0" {
		t.Fatalf("alg = %q", sig.Algorithm)
	}
	if sig.Token != "adp-token" {
		t.Fatalf("token = %q", sig.Token)
	}

	encoded, date, ok := strings.Cut(sig.Value, ":")
	if !ok {
		t.Fatalf("signature header = %q", sig.Value)
	}
	if date != "2026-03-09T01:02:03.000Z" {
		t.Fatalf("date = %q", date)
	}
	rawSig, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	payload := "POST\n/1.0/library?page=2\n" + date + "\n{\"hello\":\"world\"}\nadp-token"
	sum := sha256.Sum256([]byte(payload))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, sum[:], rawSig); err != nil {
		t.Fatalf("verify: %v", err)
	}
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
