// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
)

// NoncePlaceholder is the text in the index.html of the console build that the server replaces
// with the nonce of each response (docs/spec/19-sicherheit.md 5 rule 2, docs/DECISIONS.md
// D-M6-03-2). apps/console/web/index.html carries it in the nonce attribute of the bootstrap
// script tag; flutter_bootstrap.js hands the same value to the loader option nonce.
const NoncePlaceholder = "__BOTHY_CSP_NONCE__"

// CSPReportPath is the path of submitCspReport, the endpoint the console's report-to group csp
// names (19 5 rule 6, R-AZ-33).
const CSPReportPath = "/api/v1/csp-reports"

// TrustedTypesPolicies are the names of the Trusted Types policies the console may create
// (19 5 rule 4): exactly the names the headless test SEC-T05 observes from the Flutter loader and
// engine (tools/console-e2e, test "trusted types"), not an assumption. The loader of Flutter
// creates flutter-js for the URLs of the scripts it injects; the CanvasKit engine creates none.
// Change the list only together with the test.
var TrustedTypesPolicies = []string{"flutter-js"}

// Headers of the console responses (SEC-HDR-3) beside those of console_assets.go.
const (
	headerCSP                = "Content-Security-Policy"
	headerCOEP               = "Cross-Origin-Embedder-Policy"
	headerReportingEndpoints = "Reporting-Endpoints"
	valueRequireCORP         = "require-corp"
	// reportGroup is the name report-to uses and Reporting-Endpoints defines.
	reportGroup = "csp"
)

// nonceBytes is the size of a nonce: 128 bits from crypto/rand, base64 encoded (19 5 rule 2).
const nonceBytes = 16

// NewNonce returns a fresh nonce for one console response: 16 random bytes in standard base64,
// which is the base64-value grammar of a CSP nonce-source. crypto/rand.Read does not fail on a
// supported platform (it ends the program instead), so there is no error to return.
func NewNonce() string {
	b := make([]byte, nonceBytes)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// ContentSecurityPolicy returns the value of the Content-Security-Policy header of a console
// response with the nonce of that response (docs/spec/19-sicherheit.md 5, exactly its
// directives): scripts only from the own origin plus 'wasm-unsafe-eval' for the renderer, styles
// from the own origin and the nonce, Trusted Types required with TrustedTypesPolicies, violation
// reports to the group csp. It never contains 'unsafe-inline', 'unsafe-eval' or frame-src
// (frame-src arrives with console.grafana_embed_url in M6-17).
//
// Example:
//
//	n := httpserver.NewNonce()
//	w.Header().Set("Content-Security-Policy", httpserver.ContentSecurityPolicy(n))
func ContentSecurityPolicy(nonce string) string {
	directives := []string{
		"default-src 'none'",
		"script-src 'self' 'wasm-unsafe-eval'",
		"style-src 'self' 'nonce-" + nonce + "'",
		"img-src 'self' blob: data:",
		"font-src 'self'",
		"connect-src 'self'",
		"worker-src 'self' blob:",
		"manifest-src 'self'",
		"base-uri 'none'",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"object-src 'none'",
		"require-trusted-types-for 'script'",
		"trusted-types " + strings.Join(TrustedTypesPolicies, " "),
		"report-to " + reportGroup,
	}
	return strings.Join(directives, "; ")
}

// consoleHeaders sets the headers of SEC-HDR-3 on a console response with the nonce of that
// response: the CSP, Cross-Origin-Opener-Policy and Cross-Origin-Embedder-Policy (the console is
// cross-origin isolated), Cross-Origin-Resource-Policy: same-origin (no other origin may embed a
// console asset), X-Frame-Options: DENY, Permissions-Policy and Reporting-Endpoints for the group
// csp (docs/DECISIONS.md D-M6-03-1). SEC-HDR-1 comes from the recover step of the chain.
func consoleHeaders(h http.Header, nonce string) {
	h.Set(headerCSP, ContentSecurityPolicy(nonce))
	h.Set(headerCOOP, valueSameOrigin)
	h.Set(headerCOEP, valueRequireCORP)
	h.Set(headerCORP, valueSameOrigin)
	h.Set(headerFrameOptions, valueFrameDeny)
	h.Set(headerPermissionsPolicy, valuePermissionsPolicy)
	h.Set(headerReportingEndpoints, reportGroup+`="`+CSPReportPath+`"`)
}
