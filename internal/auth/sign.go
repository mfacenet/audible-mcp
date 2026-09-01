package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Signature is the ADP request-signing headers Amazon expects.
type Signature struct {
	Algorithm string
	Token     string
	Value     string
}

// SignRequest produces x-adp-* headers for an Audible API call.
// The signed payload is:
//
//	METHOD\nPATH?QUERY\nISO8601\nBODY\nADP_TOKEN
func SignRequest(method string, u *url.URL, body string, adpToken string, privateKeyPEM string, now time.Time) (Signature, error) {
	key, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return Signature{}, err
	}

	if now.IsZero() {
		now = time.Now()
	}
	date := now.UTC().Format("2006-01-02T15:04:05.000Z")
	path := u.EscapedPath()
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	payload := method + "\n" + path + "\n" + date + "\n" + body + "\n" + adpToken

	sum := sha256.Sum256([]byte(payload))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return Signature{}, fmt.Errorf("sign request: %w", err)
	}

	return Signature{
		Algorithm: "SHA256withRSA:1.0",
		Token:     adpToken,
		Value:     base64.StdEncoding.EncodeToString(sig) + ":" + date,
	}, nil
}

func parseRSAPrivateKey(pemText string) (*rsa.PrivateKey, error) {
	trimmed := strings.TrimSpace(pemText)
	block, _ := pem.Decode([]byte(trimmed))
	if block == nil {
		return tryParseRSAKey([]byte(trimmed))
	}
	return tryParseRSAKey(block.Bytes)
}

func tryParseRSAKey(der []byte) (*rsa.PrivateKey, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse device private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("device private key is not RSA")
	}
	return key, nil
}
