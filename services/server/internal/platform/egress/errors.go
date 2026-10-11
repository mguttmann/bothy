// SPDX-License-Identifier: Apache-2.0

package egress

import "errors"

// ErrBlocked is the sentinel of every refused attempt: errors.Is(err, ErrBlocked) holds for
// each *BlockedError, also when net/http wraps it. A caller maps it to its own error kind,
// usually 503 unavailable with the internal cause dependency_unavailable
// (docs/spec/02-architecture.md 14.1); this package knows no kinds of other modules.
var ErrBlocked = errors.New("egress: blocked")

// ErrResponseTooLarge is returned by Read of a response body that holds more bytes than the
// limit of the client (egress.max_response_bytes or Options.MaxResponseBytes). A body is never
// cut short silently: Read returns the bytes up to the limit and then this error, and keeps
// returning it.
var ErrResponseTooLarge = errors.New("egress: response too large")

// Reason says why an attempt was refused. The values are the closed set of the attribute
// bothy.reason of the counter bothy.egress.blocked (docs/observability/signals.md).
type Reason string

// The reasons of a refusal.
const (
	// ReasonURLNotAllowed: the URL carries user info, has no host or port in the accepted
	// form, or names the host in a form the client does not accept (zone, non-ASCII name).
	ReasonURLNotAllowed Reason = "url_not_allowed"
	// ReasonSchemeNotAllowed: the scheme is neither https nor http, or it is http without a
	// matching rule with allow_private.
	ReasonSchemeNotAllowed Reason = "scheme_not_allowed"
	// ReasonHostNotAllowed: neither a rule of egress.allowed_hosts nor the consumer policy
	// allows host and port; no DNS lookup happened.
	ReasonHostNotAllowed Reason = "host_not_allowed"
	// ReasonAddressBlocked: no address of the host passed the address check at connection
	// time; no connection was opened.
	ReasonAddressBlocked Reason = "address_blocked"
	// ReasonRedirectNotAllowed: a redirect target failed the checks before the connection
	// (RedirectChecked).
	ReasonRedirectNotAllowed Reason = "redirect_not_allowed"
)

// BlockedError is a refused attempt. Its text names the reason and the host, never the path,
// the query or user info of the URL.
type BlockedError struct {
	// Reason says which check refused the attempt.
	Reason Reason
	// Host is the host of the refused URL in canonical form (lower case, IP literal as
	// net/netip prints it), or "" when the URL had no usable host.
	Host string
}

// Error returns "egress: blocked: <reason> (host <host>)".
func (e *BlockedError) Error() string {
	if e.Host == "" {
		return "egress: blocked: " + string(e.Reason)
	}
	return "egress: blocked: " + string(e.Reason) + " (host " + e.Host + ")"
}

// Is reports whether target is ErrBlocked, so that errors.Is(err, ErrBlocked) finds every
// refusal.
func (e *BlockedError) Is(target error) bool { return target == ErrBlocked }
