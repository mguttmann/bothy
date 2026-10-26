// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// NoncePlaceholder is the text in the index.html of the console build that the server replaces
// with the nonce of each response (docs/spec/19-security.md 5 rule 2, docs/DECISIONS.md
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
// response with the nonce of that response (docs/spec/19-security.md 5, exactly its
// directives): scripts only from the own origin plus 'wasm-unsafe-eval' for the renderer, styles
// from the own origin and the nonce, Trusted Types required with TrustedTypesPolicies, violation
// reports to the group csp. It never contains 'unsafe-inline' or 'unsafe-eval'.
//
// frameOrigin is the origin of console.grafana_embed_url (FrameOrigin), or "" while the key is
// empty. Only with an origin is frame-src appended, naming exactly that origin and nothing else
// (19 5 rule 7, SEC-VZ-2); without one the bytes are those of M6-03 and default-src 'none' blocks
// every frame. frame-ancestors 'none' stays in both cases: the console itself is never embedded.
//
// Example:
//
//	n := httpserver.NewNonce()
//	w.Header().Set("Content-Security-Policy", httpserver.ContentSecurityPolicy(n, ""))
func ContentSecurityPolicy(nonce, frameOrigin string) string {
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
	if frameOrigin != "" {
		directives = append(directives, "frame-src "+frameOrigin)
	}
	return strings.Join(directives, "; ")
}

// dnsName matches a DNS host name in lower case: labels of letters, digits and inner hyphens,
// separated by single dots, with an optional final dot.
var dnsName = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)*[a-z0-9]([a-z0-9-]*[a-z0-9])?\.?$`)

// FrameOrigin returns the origin of console.grafana_embed_url for frame-src: scheme, host in lower
// case and port of an https URL with a host and without user information, the default port 443
// dropped (brief M6-17 Design note 1). The host must be a DNS name or an IP literal, so the
// result is exactly one origin, never a wildcard pattern or text with a CSP delimiter. The path, query and fragment of the dashboard never reach
// the policy. An empty value returns "" and no error (no embedding). The error says what is
// wrong and never repeats the value.
//
// Example:
//
//	o, err := httpserver.FrameOrigin("https://grafana.example.com/d/ops") // "https://grafana.example.com"
func FrameOrigin(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	switch {
	case err != nil || u.Opaque != "":
		return "", errors.New("not a URL")
	case !strings.EqualFold(u.Scheme, schemeHTTPS):
		// The console is served over https and cross-origin isolated; an http frame would be
		// mixed content (25 9.2).
		return "", errors.New("scheme must be https")
	case u.Hostname() == "":
		return "", errors.New("host missing")
	case u.User != nil:
		return "", errors.New("must not contain user information")
	}
	host := strings.ToLower(u.Hostname())
	if net.ParseIP(host) == nil && !dnsName.MatchString(host) {
		// A wildcard or a CSP delimiter (";", ",", a space) would make frame-src a pattern or
		// add a directive; only a DNS name or an IP literal is exactly one origin.
		return "", errors.New("host must be a DNS name or an IP address")
	}
	port := u.Port()
	if port == "443" {
		port = ""
	}
	if port != "" {
		return schemeHTTPS + "://" + net.JoinHostPort(host, port), nil
	}
	if strings.Contains(host, ":") {
		return schemeHTTPS + "://[" + host + "]", nil
	}
	return schemeHTTPS + "://" + host, nil
}

// consoleHeaders sets the headers of SEC-HDR-3 on a console response with the nonce of that
// response: the CSP, Cross-Origin-Opener-Policy and Cross-Origin-Embedder-Policy (the console is
// cross-origin isolated), Cross-Origin-Resource-Policy: same-origin (no other origin may embed a
// console asset), X-Frame-Options: DENY, Permissions-Policy and Reporting-Endpoints for the group
// csp (docs/DECISIONS.md D-M6-03-1). frameOrigin is the origin of console.grafana_embed_url or
// "" (ContentSecurityPolicy). SEC-HDR-1 comes from the recover step of the chain.
func consoleHeaders(h http.Header, nonce, frameOrigin string) {
	h.Set(headerCSP, ContentSecurityPolicy(nonce, frameOrigin))
	h.Set(headerCOOP, valueSameOrigin)
	h.Set(headerCOEP, valueRequireCORP)
	h.Set(headerCORP, valueSameOrigin)
	h.Set(headerFrameOptions, valueFrameDeny)
	h.Set(headerPermissionsPolicy, valuePermissionsPolicy)
	h.Set(headerReportingEndpoints, reportGroup+`="`+CSPReportPath+`"`)
}
