// Package marketplace catalogs Audible storefronts used by the iOS app OAuth flow.
package marketplace

import (
	"fmt"
	"strings"
)

// Marketplace is an Audible storefront.
type Marketplace struct {
	Code          string
	Domain        string
	MarketPlaceID string
}

var byCode = map[string]Marketplace{
	"au": {Code: "au", Domain: "com.au", MarketPlaceID: "AN7EY7DTAW63G"},
	"br": {Code: "br", Domain: "com.br", MarketPlaceID: "A10J1VAYUDTYRN"},
	"ca": {Code: "ca", Domain: "ca", MarketPlaceID: "A2CQZ5RBY40XE"},
	"de": {Code: "de", Domain: "de", MarketPlaceID: "AN7V1F1VY261K"},
	"es": {Code: "es", Domain: "es", MarketPlaceID: "ALMIKO4SZCSAR"},
	"fr": {Code: "fr", Domain: "fr", MarketPlaceID: "A2728XDNODOQ8T"},
	"in": {Code: "in", Domain: "in", MarketPlaceID: "AJO3FBRUE6J4S"},
	"it": {Code: "it", Domain: "it", MarketPlaceID: "A2N7FU2W2BU2ZC"},
	"jp": {Code: "jp", Domain: "co.jp", MarketPlaceID: "A1QAP3MOU4173J"},
	"uk": {Code: "uk", Domain: "co.uk", MarketPlaceID: "A2I9A3Q2GNFNGQ"},
	"us": {Code: "us", Domain: "com", MarketPlaceID: "AF2M0KC94RCEA"},
}

// Lookup returns the marketplace for a country code such as "us" or "uk".
func Lookup(code string) (Marketplace, error) {
	m, ok := byCode[strings.ToLower(strings.TrimSpace(code))]
	if !ok {
		return Marketplace{}, fmt.Errorf("unknown Audible marketplace %q", code)
	}
	return m, nil
}

// AllowsAudibleUsername reports whether Amazon's Audible-username login
// (as opposed to an Amazon account) is available for this storefront.
func (m Marketplace) AllowsAudibleUsername() bool {
	switch m.Domain {
	case "com", "co.uk", "de":
		return true
	default:
		return false
	}
}
