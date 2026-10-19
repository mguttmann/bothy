// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"errors"
	"net/url"
	"strings"
)

// apiPrefix is the path prefix of the REST API and the gateway (docs/api/openapi.yaml servers).
const apiPrefix = "/api/v1"

// errPublicURL is the error of a public URL the server would not accept as server.public_url.
var errPublicURL = errors.New("smoke: the public URL must be an absolute http or https URL without query, fragment or user")

// publicOrigin parses publicURL like the server parses server.public_url: scheme http or https, a
// host, no user information, no query and no fragment. A path is allowed and does not count.
func publicOrigin(publicURL string) (*url.URL, error) {
	u, err := url.Parse(publicURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(publicURL, "#") {
		return nil, errPublicURL
	}
	return u, nil
}

// GatewayHTU returns the htu of a gateway proof (docs/spec/04-sync-protocol.md 6.3 step 3): the
// HTTPS form of <origin of publicURL>/api/v1/gateway. It follows the rule of the server (M1-33,
// gateway adapter HTU): the scheme becomes https, an http URL on port 80 loses the port, and a
// path of the public URL does not count, because the gateway path is fixed. The error reports a
// public URL the server would reject.
//
// Example: GatewayHTU("https://chat.example.com") returns "https://chat.example.com/api/v1/gateway".
func GatewayHTU(publicURL string) (string, error) {
	u, err := publicOrigin(publicURL)
	if err != nil {
		return "", err
	}
	host := u.Host
	if u.Scheme == "http" && u.Port() == "80" {
		host = u.Hostname()
	}
	return "https://" + host + apiPrefix + "/gateway", nil
}

// RESTHTU returns the htu a REST proof for path must name (docs/spec/08-identity.md 7.3, M1-33):
// scheme and host of publicURL plus the escaped request path, never the loopback address the
// probe dials. path is the absolute request path including /api/v1; a query in it is dropped. The
// error reports a public URL the server would reject or a relative path.
//
// Example: RESTHTU("https://chat.example.com", "/api/v1/me") returns "https://chat.example.com/api/v1/me".
func RESTHTU(publicURL, path string) (string, error) {
	u, err := publicOrigin(publicURL)
	if err != nil {
		return "", err
	}
	p, _, _ := strings.Cut(path, "?")
	if !strings.HasPrefix(p, "/") {
		return "", errors.New("smoke: the request path must be absolute")
	}
	return u.Scheme + "://" + u.Host + (&url.URL{Path: p}).EscapedPath(), nil
}
