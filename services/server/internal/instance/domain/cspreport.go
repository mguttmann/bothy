// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strings"
)

// CSPReportFormat is the format of a Content Security Policy violation report body (submitCspReport,
// docs/spec/19-sicherheit.md 5 rule 6).
type CSPReportFormat int

// The two formats a browser sends.
const (
	// CSPReportingAPI is application/reports+json: an array of reports of the Reporting API, the
	// format of the directive report-to.
	CSPReportingAPI CSPReportFormat = iota + 1
	// CSPLegacy is application/csp-report: one object {"csp-report": {...}} of the directive
	// report-uri (CSP Level 2).
	CSPLegacy
)

// CSPOther is the value of a field whose content is not on the fixed list.
const CSPOther = "other"

// CSPViolation is what the server keeps of one violation report: the effective directive, the
// disposition and the origin of the blocked URL, each from a fixed list or reduced to an origin,
// never a path, a query, the document URL or the sample (19 5 rule 6, R-AZ-33).
type CSPViolation struct {
	// Directive is a name of cspDirectives or CSPOther.
	Directive string
	// Disposition is enforce, report or CSPOther.
	Disposition string
	// BlockedOrigin is scheme://host[:port] of an http, https, ws or wss URL, a keyword of
	// cspKeywords, data or blob for those schemes, or CSPOther.
	BlockedOrigin string
}

// ErrCSPReport is the error of a body that is not of the declared format.
var ErrCSPReport = errors.New("domain: the body is not a CSP violation report of its media type")

// cspDirectives are the fetch, document, navigation and Trusted Types directives of CSP Level 3
// that a report can name as its effective directive.
var cspDirectives = []string{
	"base-uri", "child-src", "connect-src", "default-src", "fenced-frame-src", "font-src", "form-action",
	"frame-ancestors", "frame-src", "img-src", "manifest-src", "media-src", "object-src",
	"require-trusted-types-for", "sandbox", "script-src", "script-src-attr", "script-src-elem", "style-src",
	"style-src-attr", "style-src-elem", "trusted-types", "webrtc", "worker-src",
}

// CSPDirectives returns the directive names a CSPViolation keeps, a fresh slice; the metric
// bothy.console.csp_violations declares the same set (adapters/otelmetrics holds them equal).
func CSPDirectives() []string { return slices.Clone(cspDirectives) }

// cspKeywords are the values of blockedURL (blocked-uri) that name no URL (CSP Level 3 and Trusted
// Types).
var cspKeywords = []string{"inline", "eval", "wasm-eval", "trusted-types-policy", "trusted-types-sink"}

// maxHostLength is the longest DNS name (RFC 1035 2.3.4); a longer host is no origin to log.
const maxHostLength = 253

// reportingAPIReport is one report of the Reporting API; only the read members are declared.
type reportingAPIReport struct {
	Type string          `json:"type"`
	Body json.RawMessage `json:"body"`
}

// reportingAPIBody is the body of a csp-violation report (CSP Level 3, section 5.2).
type reportingAPIBody struct {
	BlockedURL         string `json:"blockedURL"`
	EffectiveDirective string `json:"effectiveDirective"`
	Disposition        string `json:"disposition"`
}

// legacyReport is the body of report-uri (CSP Level 2, section 4.4).
type legacyReport struct {
	Report *struct {
		BlockedURI         string `json:"blocked-uri"`
		EffectiveDirective string `json:"effective-directive"`
		ViolatedDirective  string `json:"violated-directive"`
		Disposition        string `json:"disposition"`
	} `json:"csp-report"`
}

// ParseCSPReports returns the violations of body in format. In the Reporting API format it keeps
// the reports of type csp-violation and skips every other report and every item that is no
// object; members it does not read are ignored. Errors: ErrCSPReport for a body that is not an
// array (Reporting API) or not an object with the object csp-report (legacy).
//
// Example:
//
//	v, err := domain.ParseCSPReports(domain.CSPLegacy, []byte(`{"csp-report":{"blocked-uri":"inline"}}`))
//	// v[0].BlockedOrigin == "inline"
func ParseCSPReports(format CSPReportFormat, body []byte) ([]CSPViolation, error) {
	if format == CSPLegacy {
		var r legacyReport
		if err := json.Unmarshal(body, &r); err != nil || r.Report == nil {
			return nil, ErrCSPReport
		}
		directive := r.Report.EffectiveDirective
		if directive == "" {
			directive, _, _ = strings.Cut(r.Report.ViolatedDirective, " ")
		}
		return []CSPViolation{violation(directive, r.Report.Disposition, r.Report.BlockedURI)}, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, ErrCSPReport
	}
	out := []CSPViolation{}
	for _, raw := range items {
		var r reportingAPIReport
		if json.Unmarshal(raw, &r) != nil || r.Type != "csp-violation" {
			continue
		}
		var b reportingAPIBody
		if json.Unmarshal(r.Body, &b) != nil {
			continue
		}
		out = append(out, violation(b.EffectiveDirective, b.Disposition, b.BlockedURL))
	}
	return out, nil
}

// violation reduces the three read fields to their fixed lists and the origin.
func violation(directive, disposition, blocked string) CSPViolation {
	v := CSPViolation{Directive: CSPOther, Disposition: CSPOther, BlockedOrigin: blockedOrigin(blocked)}
	if slices.Contains(cspDirectives, directive) {
		v.Directive = directive
	}
	if disposition == "enforce" || disposition == "report" {
		v.Disposition = disposition
	}
	return v
}

// blockedOrigin returns the origin of blocked, a keyword, data, blob or CSPOther.
func blockedOrigin(blocked string) string {
	if slices.Contains(cspKeywords, blocked) {
		return blocked
	}
	u, err := url.Parse(blocked)
	if err != nil {
		return CSPOther
	}
	switch scheme := strings.ToLower(u.Scheme); scheme {
	case "data", "blob":
		return scheme
	case "http", "https", "ws", "wss":
		host := strings.ToLower(u.Host)
		if u.Hostname() == "" || len(host) > maxHostLength {
			return CSPOther
		}
		return (&url.URL{Scheme: scheme, Host: host}).String()
	}
	return CSPOther
}
