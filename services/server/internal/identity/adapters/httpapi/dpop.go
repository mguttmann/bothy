// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"net/http"
	"net/url"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// ExpectedHTU returns the htu a DPoP proof of r must name (08 "7.3"): the origin of
// server.public_url (scheme, host and port of publicURL) plus the escaped request path, without
// query and fragment. It never reads the Host header of the client, so a proof for another host
// never matches.
func ExpectedHTU(publicURL *url.URL, r *http.Request) string {
	u := url.URL{Scheme: publicURL.Scheme, Host: publicURL.Host, Path: r.URL.Path, RawPath: r.URL.RawPath}
	return u.Scheme + "://" + u.Host + u.EscapedPath()
}

// Cursors is the cursor codec of platform/httpserver (M1-18, 06 "6. Paginierung") behind the
// port app.Cursors, so the application layer stays free of platform packages.
type Cursors struct{}

// Compile-time check: the codec is the outgoing port of the device lists.
var _ app.Cursors = Cursors{}

// Encode implements app.Cursors with httpserver.EncodeCursor.
func (Cursors) Encode(list string, binding []byte, key any) (string, error) {
	return httpserver.EncodeCursor(list, binding, key)
}

// Decode implements app.Cursors with httpserver.DecodeCursor (400 cursor_invalid on a mismatch).
func (Cursors) Decode(list string, binding []byte, s string, key any) error {
	return httpserver.DecodeCursor(list, binding, s, key)
}
