// SPDX-License-Identifier: Apache-2.0

// Package smoke is the probe of the Compose smoke test (brief M1-73, plan P-06, exit criterion 8
// of milestone M1): a black-box client that drives a server started from the real bothy-server
// image through its public listener, and the scanner that proves afterwards that no canary,
// token, enrollment code or principal ID left the stack through the collector export or the
// container logs (docs/observability/privacy.md section 6).
//
// infra/compose/smoke.sh runs the command cmd/bothy-smoke in the service smoke-probe of the test
// overlay infra/compose/test/compose.smoke.yaml. The probe shares the network namespace of the
// api container and dials http://127.0.0.1:8080, so the server sees a loopback peer as SEC-TLS-1
// requires, and no host port is published (P-10). DPoP proofs name the public URL
// (server.public_url), not the dialled loopback address (docs/spec/08-identity.md 7.3).
//
// The package imports only the generated api/rest and github.com/coder/websocket besides the
// standard library and never a package below internal/ (TestBlackBoxImports): it sees the server
// only as an installation's client would. It has no JOSE library either; dpop.go builds the
// compact JWS with crypto/ecdsa, like any client of the contract (ADR 0067).
//
// Files:
//   - dpop.go: Key, NewProof, StepUpSignature.
//   - htu.go: GatewayHTU, RESTHTU.
//   - rest.go: the REST interface of the operations the scenario calls and HTTPClient.
//   - gateway.go: the Gateway interface and WSGateway.
//   - scenario.go: Precheck and Run, the steps of the smoke test.
//   - enroll.go: ReadEnrollmentCode.
//   - scan.go: Canaries, Scan, Roots, CheckTelemetry.
//
// Example (what the command does for the subcommand run):
//
//	api, err := smoke.NewHTTPClient("http://127.0.0.1:8080", "https://chat.example.com")
//	canaries, err := smoke.Run(ctx, smoke.Config{PublicURL: "https://chat.example.com", Canary: c,
//		SetupStdout: setupOut, Log: os.Stderr}, api, func() smoke.Gateway { return smoke.NewWSGateway(base) })
package smoke
