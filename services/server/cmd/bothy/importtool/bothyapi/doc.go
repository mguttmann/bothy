// SPDX-License-Identifier: Apache-2.0

// Package bothyapi is the authenticated client of the server API for the import tool
// (docs/spec/22-import-migration.md 2, 8 and 10; docs/spec/08-identity.md 5.3, 6.4, 7.3 and
// 7.4). The tool logs in only as a device with platform cli through device pairing, without a
// step-up key (Login), keeps device ID, DPoP key and tokens in a session file that only the user
// can read (SessionFile), and talks to the server only through the client generated from
// docs/api/openapi.yaml (api/restclient), wrapped in a transport that signs every request with
// a DPoP proof (RFC 9449), retries once on use_dpop_nonce, refreshes an expired access token once
// under the lock of the session file and stores the rotated refresh token before it uses the new
// access token, honours 429 with Retry-After within the tool's limits and never follows a
// redirect. Every connection goes through platform/egress with the server host as the only
// allowed host. No token, key or session file content reaches output, log or error text.
//
// Example:
//
//	c, err := bothyapi.New(ctx, bothyapi.Options{SessionFile: path, Limits: bothyapi.DefaultLimits()})
//	if errors.Is(err, bothyapi.ErrNotLoggedIn) {
//		// exit 2: run bothy import login
//	}
//	me, err := c.Me(ctx)
package bothyapi
