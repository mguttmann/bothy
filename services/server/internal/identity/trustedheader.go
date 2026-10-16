// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"net/netip"
)

// TrustedHeaderLogin is the input of TrustedHeaderLogins.LoginWithTrustedHeader (schema
// TrustedHeaderLoginRequest plus the request facts of the HTTP chain).
type TrustedHeaderLogin struct {
	// Console selects the console form (client = console, M6-01): no Device and no Proof; the
	// login registers a web device and starts a console session with a cookie instead of tokens.
	Console bool
	// Peer is the IP of the TCP connection, never a forwarded address; the identity headers count
	// only when it lies in server.trusted_proxies (08 section 2.4 rule 2).
	Peer netip.Addr
	// Header are the request header fields with canonical names; the driver reads only the names
	// of its configuration. The values are personal data and never logged.
	Header map[string][]string
	// EnrollmentCode is the optional code that links an unknown login (08 section 3.1); "" for none.
	EnrollmentCode string
	// Device is the registration of the logging-in device.
	Device RegisterDevice
	// Proof is the DPoP proof of the request, verified as a login proof without ath.
	Proof ProofRequest
	// ClientIP is the client address of the request; the zero Addr for none.
	ClientIP netip.Addr
}

// TrustedHeaderLogins is the incoming port of the login with the identity header of a trusted
// reverse proxy (M1-43, docs/spec/08-identity.md section 2.4).
type TrustedHeaderLogins interface {
	// LoginWithTrustedHeader authorizes auth.login.complete for the anonymous principal and logs in
	// the person of the header login: a known identity, a link through the enrollment code or a
	// provisioned pending person. Errors: not_found while the driver is off (no enabled
	// trusted_header provider or no loopback listener), unauthenticated for a missing, untrusted or
	// unmapped login and every rejected code (one body), invalid_dpop_proof or use_dpop_nonce,
	// validation_failed or conflict for the device registration.
	LoginWithTrustedHeader(ctx context.Context, in TrustedHeaderLogin) (RedeemResult, error)
}
