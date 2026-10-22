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
// docs/spec/19-security.md 5 rule 6).
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
// the reports of type csp-violation and skips every other report; members it does not read are
// ignored. The body follows the contract's shape: errors are ErrCSPReport for a body that is not
// an array of objects (Reporting API) or not an object with exactly the object csp-report
// (legacy).
//
// Example:
//
//	v, err := domain.ParseCSPReports(domain.CSPLegacy, []byte(`{"csp-report":{"blocked-uri":"inline"}}`))
//	// v[0].BlockedOrigin == "inline"
func ParseCSPReports(format CSPReportFormat, body []byte) ([]CSPViolation, error) {
	if format == CSPLegacy {
		return parseLegacyReport(body)
	}
	return parseReportingAPIReports(body)
}

// parseLegacyReport reads the body of report-uri: exactly the one member csp-report
// (CspReportLegacy of the contract); the members of the report itself that the server does not
// read are ignored.
func parseLegacyReport(body []byte) ([]CSPViolation, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil || len(top) != 1 {
		return nil, ErrCSPReport
	}
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

// parseReportingAPIReports reads an array of reports of the Reporting API; every item is an
// object (CspReportList of the contract), null and other values are no report.
func parseReportingAPIReports(body []byte) ([]CSPViolation, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil || items == nil {
		return nil, ErrCSPReport
	}
	out := []CSPViolation{}
	for _, raw := range items {
		var object map[string]json.RawMessage
		if json.Unmarshal(raw, &object) != nil || object == nil {
			return nil, ErrCSPReport
		}
		if v, ok := cspViolationOf(raw); ok {
			out = append(out, v)
		}
	}
	return out, nil
}

// cspViolationOf returns the violation of one report object, false for a report of another type
// or a body that is missing, null or no object.
func cspViolationOf(raw json.RawMessage) (CSPViolation, bool) {
	var r reportingAPIReport
	if json.Unmarshal(raw, &r) != nil || r.Type != "csp-violation" {
		return CSPViolation{}, false
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(r.Body, &object) != nil || object == nil {
		return CSPViolation{}, false
	}
	var b reportingAPIBody
	if json.Unmarshal(r.Body, &b) != nil {
		return CSPViolation{}, false
	}
	return violation(b.EffectiveDirective, b.Disposition, b.BlockedURL), true
}

// CSPViolationCount is one distinct violation of a report body and how often it occurs there.
type CSPViolationCount struct {
	CSPViolation
	// Count is at least 1.
	Count int64
}

// SummarizeCSPViolations folds identical violations into one entry with their count, in the
// order of their first occurrence; a nil or empty input gives an empty result. A report body of
// many equal reports so costs one log record, not one per report (R-AZ-33).
func SummarizeCSPViolations(vs []CSPViolation) []CSPViolationCount {
	out := []CSPViolationCount{}
	index := map[CSPViolation]int{}
	for _, v := range vs {
		if i, ok := index[v]; ok {
			out[i].Count++
			continue
		}
		index[v] = len(out)
		out = append(out, CSPViolationCount{CSPViolation: v, Count: 1})
	}
	return out
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
