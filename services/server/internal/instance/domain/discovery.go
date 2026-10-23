// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"crypto/sha256"
	"encoding/base64"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Paths discovery names (docs/api/openapi.yaml schema Discovery).
const (
	// GatewayURL is the path of the WebSocket gateway.
	GatewayURL = "/api/v1/gateway"
	// JWKSURI is the path of getJwks.
	JWKSURI = "/api/v1/instance/jwks"
	// WebhookKeysURI is the path of getWebhookKeys.
	WebhookKeysURI = "/api/v1/instance/webhook-keys"
	// LogoURL is the path of getBrandingLogo; discovery names it only while branding.logo_file_id
	// is set.
	LogoURL = "/api/v1/instance/branding/logo"
)

// DiscoveryLogoURL returns logo_url of discovery for the logo file id logo: LogoURL while a logo
// is set, "" (member omitted) for the zero id, which LogoFileID returns for NoLogo.
func DiscoveryLogoURL(logo ids.ID) string {
	if logo.IsZero() {
		return ""
	}
	return LogoURL
}

// NoMinVersion is the min_version of a platform that has a recommended version but no minimum:
// the contract requires min_version, and 0.0.0 admits every app version (docs/DECISIONS.md
// M1-46/8).
const NoMinVersion = "0.0.0"

// discoveryFeatures are the feature keys this server implements beyond the base contract of its
// spec version (06 section 13 item 1). None exists yet. A capability enters here when a client
// must detect it at run time because servers of the same spec version may lack it; the key is
// lower case with dots (for example "search.semantic"), and the entry names its ticket.
var discoveryFeatures = []string{}

// DiscoveryFeatures returns the feature list of discovery; a fresh, non-nil slice.
func DiscoveryFeatures() []string { return append([]string{}, discoveryFeatures...) }

// APIVersions returns the API versions this server serves; a fresh slice.
func APIVersions() []string { return []string{"v1"} }

// Platforms are the app platforms of the client version keys, in the order discovery lists them
// (18 section 9 item 2).
var Platforms = []string{"ios", "android", "macos", "windows"}

// ClientVersionConfig are the values of client.min_version.<platform> and
// client.recommended_version.<platform>; a missing or empty entry means not set.
type ClientVersionConfig struct {
	// Min maps a platform to its minimum version.
	Min map[string]string
	// Recommended maps a platform to its recommended version.
	Recommended map[string]string
}

// ClientVersion is one entry of Discovery.min_client_versions.
type ClientVersion struct {
	// Platform is ios, android, macos or windows.
	Platform string
	// MinVersion is the minimum version; NoMinVersion when only a recommended version is set.
	MinVersion string
	// RecommendedVersion is the recommended version; "" for none.
	RecommendedVersion string
}

// MinClientVersions returns one entry per platform with a minimum or a recommended version set,
// in the order of Platforms; an empty, non-nil list when none is set (arrays are always sent).
func MinClientVersions(c ClientVersionConfig) []ClientVersion {
	out := []ClientVersion{}
	for _, p := range Platforms {
		lowest, rec := c.Min[p], c.Recommended[p]
		if lowest == "" && rec == "" {
			continue
		}
		if lowest == "" {
			lowest = NoMinVersion
		}
		out = append(out, ClientVersion{Platform: p, MinVersion: lowest, RecommendedVersion: rec})
	}
	return out
}

// providerKindLocal is the kind of the local account provider.
const providerKindLocal = "local"

// LocalPasswordEnabled reports whether discovery offers the password with TOTP: the
// configuration allows it (auth.local_password_enabled) and an enabled provider of kind local
// exists among kinds (docs/DECISIONS.md M1-46/6).
func LocalPasswordEnabled(configAllows bool, kinds []string) bool {
	return configAllows && slices.Contains(kinds, providerKindLocal)
}

// etagChars is the length of a content ETag: 22 base64url characters carry 132 bits of SHA-256.
const etagChars = 22

// ContentETag returns the strong ETag of a response body that has no version of its own
// (getInstance): the first 22 characters of the base64url SHA-256 of the canonical body, in
// double quotes. The body differs per caller once feature flags exist (M1-47), so a version
// counter cannot serve.
func ContentETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + base64.RawURLEncoding.EncodeToString(sum[:])[:etagChars] + `"`
}
