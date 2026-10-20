// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// ExternalOptions describe a server that the harness did not start (M2-48): the Compose stack of
// the terminal conformance suite, reached through its TLS front.
type ExternalOptions struct {
	// PublicURL is server.public_url of the server (the origin every DPoP proof names).
	PublicURL string
	// HTTPClient reaches PublicURL (for example a transport that maps the instance origin to a
	// published port and trusts a test CA).
	HTTPClient *http.Client
	// OwnerEnrollmentCode is the enrollment code of the first owner (bothy setup bootstrap-owner);
	// External redeems it once with fresh device keys.
	OwnerEnrollmentCode string
	// ProofObserver and TokenObserver are Options.ProofObserver and Options.TokenObserver.
	ProofObserver func(string)
	TokenObserver func(access, refresh string)
}

// External returns a Server for a server the harness did not start: the owner is enrolled with
// opts.OwnerEnrollmentCode and fresh device keys (redeemEnrollmentCode), so NewUser, NewAgent,
// NewTokenClient and the clients work as with Start. SecretsDir and SigningKeyDir stay empty,
// AdminDB fails the test (the database of such a server is read only through its own tools),
// and OwnerSession and UserSession are not available. Errors fail tb.
//
// Example:
//
//	s := harness.External(t, harness.ExternalOptions{PublicURL: "https://chat.example.com:8443",
//		HTTPClient: client, OwnerEnrollmentCode: code})
func External(tb testing.TB, opts ExternalOptions) *Server {
	tb.Helper()
	public := strings.TrimSuffix(opts.PublicURL, "/")
	s := &Server{PublicURL: public, APIBase: public + apiPrefix, ListenURL: public,
		GatewayURL: "wss" + strings.TrimPrefix(public, "https") + apiPrefix + "/gateway",
		tb:         tb, clk: clock.System(), http: opts.HTTPClient, external: true, proofs: opts.ProofObserver, tokens: opts.TokenObserver}
	owner, err := s.redeem(tb.Context(), opts.OwnerEnrollmentCode, "Owner device")
	if err != nil {
		tb.Fatalf("harness: external owner: %v", err)
	}
	s.Owner = owner
	return s
}

// errExternal reports a setup service call on a server of External.
var errExternal = errors.New("harness: the setup service of an external server is not available")
