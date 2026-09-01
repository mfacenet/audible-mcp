// Package auth implements Amazon's Audible iOS device registration and
// ADP request signing for a user's own account.
//
// The protocol is the OpenID Authorization Code flow with PKCE used by
// the official Audible iOS app, plus RSA-SHA256 ADP signatures on
// subsequent API calls. This package is a Go implementation of that
// public protocol. It is not a translation of mkb79/Audible (AGPL).
package auth
