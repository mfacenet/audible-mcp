package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/mfacenet/audible-mcp/internal/marketplace"
)

// audibleIOSDeviceType is Amazon's device type for the Audible iOS app.
const audibleIOSDeviceType = "A2CZJZGLK2JJVM"

// audibleIOSAppVersion and audibleIOSSoftwareVersion are the iOS app
// identifiers Amazon's registration endpoint expects.
const (
	audibleIOSAppVersion      = "3.56.2"
	audibleIOSSoftwareVersion = "35602678"
)

// Session is a PKCE login in progress against Amazon's Audible iOS OpenID flow.
type Session struct {
	CodeVerifier string
	LoginURL     string
	Marketplace  marketplace.Marketplace
	Serial       string
	WithUsername bool
}

// SessionOption configures NewSession.
type SessionOption func(*sessionConfig)

type sessionConfig struct {
	serial       string
	withUsername bool
}

// WithSerial pins the virtual-device serial instead of generating one.
func WithSerial(serial string) SessionOption {
	return func(c *sessionConfig) {
		c.serial = serial
	}
}

// WithAudibleUsername uses Audible-username login instead of an Amazon account.
func WithAudibleUsername() SessionOption {
	return func(c *sessionConfig) {
		c.withUsername = true
	}
}

// NewSession builds the Amazon OpenID/PKCE URL used by the Audible iOS app.
func NewSession(m marketplace.Marketplace, opts ...SessionOption) (Session, error) {
	var cfg sessionConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.withUsername && !m.AllowsAudibleUsername() {
		return Session{}, fmt.Errorf("Audible-username login is only supported for US, UK, and DE marketplaces")
	}

	serial := cfg.serial
	if serial == "" {
		var err error
		serial, err = newDeviceSerial()
		if err != nil {
			return Session{}, err
		}
	}

	verifier, err := newCodeVerifier()
	if err != nil {
		return Session{}, err
	}
	challenge := s256Challenge(verifier)
	clientID := ClientID(serial)

	hostKind := "amazon"
	assocHandle := "amzn_audible_ios_" + m.Code
	pageID := "amzn_audible_ios"
	if cfg.withUsername {
		hostKind = "audible"
		assocHandle = "amzn_audible_ios_lap_" + m.Code
		pageID = "amzn_audible_ios_privatepool"
	}
	baseHost := fmt.Sprintf("https://www.%s.%s", hostKind, m.Domain)
	returnTo := baseHost + "/ap/maplanding"

	q := url.Values{}
	q.Set("accountStatusPolicy", "P1")
	q.Set("forceMobileLayout", "true")
	q.Set("marketPlaceId", m.MarketPlaceID)
	q.Set("openid.assoc_handle", assocHandle)
	q.Set("openid.claimed_id", "http://specs.openid.net/auth/2.0/identifier_select")
	q.Set("openid.identity", "http://specs.openid.net/auth/2.0/identifier_select")
	q.Set("openid.mode", "checkid_setup")
	q.Set("openid.ns", "http://specs.openid.net/auth/2.0")
	q.Set("openid.ns.oa2", "http://www.amazon.com/ap/ext/oauth/2")
	q.Set("openid.ns.pape", "http://specs.openid.net/extensions/pape/1.0")
	q.Set("openid.oa2.client_id", "device:"+clientID)
	q.Set("openid.oa2.code_challenge", challenge)
	q.Set("openid.oa2.code_challenge_method", "S256")
	q.Set("openid.oa2.response_type", "code")
	q.Set("openid.oa2.scope", "device_auth_access")
	q.Set("openid.pape.max_auth_age", "0")
	q.Set("openid.return_to", returnTo)
	q.Set("pageId", pageID)

	return Session{
		CodeVerifier: verifier,
		LoginURL:     baseHost + "/ap/signin?" + q.Encode(),
		Marketplace:  m,
		Serial:       serial,
		WithUsername: cfg.withUsername,
	}, nil
}

// ClientID is the hex-encoded Audible iOS client id derived from a device serial.
func ClientID(serial string) string {
	return hex.EncodeToString([]byte(serial + "#" + audibleIOSDeviceType))
}

// AuthorizationCode extracts openid.oa2.authorization_code from a maplanding URL.
func AuthorizationCode(responseURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(responseURL))
	if err != nil {
		return "", fmt.Errorf("parse maplanding URL: %w", err)
	}
	code := u.Query().Get("openid.oa2.authorization_code")
	if code == "" {
		return "", fmt.Errorf("response URL does not contain openid.oa2.authorization_code")
	}
	return code, nil
}

func newDeviceSerial() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate device serial: %w", err)
	}
	// UUID v4 bits, then hex without dashes, matching the iOS serial shape.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return strings.ToUpper(hex.EncodeToString(b[:])), nil
}

func newCodeVerifier() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate PKCE verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func s256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
