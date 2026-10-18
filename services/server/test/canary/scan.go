// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

// AllowRules are the run-specific parts of the allow matrix (README.md "Allow matrix").
type AllowRules struct {
	// Readable maps an observer of the foreign sink to the scopes (channel and space IDs) it may
	// read; a content or admin_reason value of such a scope may appear in its records.
	Readable map[string]map[string]bool
}

// Finding is one value found where its class may not appear. It never holds the full value.
type Finding struct {
	// Sink is the output, Location the record and field (line, span, JSON pointer, header).
	Sink, Location string
	// Class and Label name the value.
	Class, Label string
	// Prefix is the first eight characters of the matched value.
	Prefix string
}

// String renders the finding without the value.
func (f Finding) String() string {
	return fmt.Sprintf("%s at %s: %s %q (%s...)", f.Sink, f.Location, f.Class, f.Label, f.Prefix)
}

// Report renders findings one per line, sorted, for a test failure; it never prints a value.
func Report(findings []Finding) string {
	lines := make([]string, 0, len(findings))
	for _, f := range findings {
		lines = append(lines, f.String())
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// minNeedle is the shortest text Scan looks for: shorter forms (six-digit TOTP codes) would match
// unrelated digits of timestamps and counters. README.md "Not covered" names them.
const minNeedle = 8

// indexLen is the length of the window the needle index is keyed by; it is minNeedle, so every
// needle has a key.
const indexLen = minNeedle

// needle is one searchable form of a canary.
type needle struct {
	text string // lowercase
	c    *Canary
}

// Scan matches every registered value, in every form of README.md "Matching", against every
// field of every record and returns the matches the allow matrix forbids. Matching is
// case-insensitive. A value is reported once per record field.
func Scan(reg *Registry, sinks *Sinks, rules AllowRules) []Finding {
	canaries := reg.Canaries()
	index := map[string][]needle{}
	for i := range canaries {
		c := &canaries[i]
		for _, n := range forms(*c) {
			n = strings.ToLower(n)
			if len(n) < minNeedle {
				continue
			}
			index[n[:indexLen]] = append(index[n[:indexLen]], needle{text: n, c: c})
		}
	}
	var out []Finding
	for _, r := range sinks.Records() {
		for _, f := range r.Fields {
			for _, c := range match(index, strings.ToLower(f.Text)) {
				if allowed(*c, r, f.Location, rules) {
					continue
				}
				prefix := c.Value
				if len(prefix) > minNeedle {
					prefix = prefix[:minNeedle]
				}
				loc := strings.TrimSpace(r.Source + " " + f.Location)
				out = append(out, Finding{Sink: r.Sink, Location: loc, Class: string(c.Class), Label: c.Label, Prefix: prefix})
			}
		}
	}
	return out
}

// match returns the canaries with a needle in text (lowercase), each once.
func match(index map[string][]needle, text string) []*Canary {
	var out []*Canary
	seen := map[*Canary]bool{}
	for i := 0; i+indexLen <= len(text); i++ {
		cands, ok := index[text[i:i+indexLen]]
		if !ok {
			continue
		}
		for _, n := range cands {
			if !seen[n.c] && strings.HasPrefix(text[i:], n.text) {
				seen[n.c] = true
				out = append(out, n.c)
			}
		}
	}
	return out
}

// allowed applies the allow matrix to canary c found in field location of record r.
func allowed(c Canary, r Record, location string, rules AllowRules) bool {
	switch c.Class {
	case ClassSecret, ClassNetwork:
		return false
	case ClassIdentity:
		return r.Sink != SinkLog && r.Sink != SinkTrace && r.Sink != SinkMetric
	case ClassAdminReason:
		if r.Sink == SinkAudit && location == "/details/reason" {
			return true
		}
		return contentAllowed(c, r, rules)
	default:
		return contentAllowed(c, r, rules)
	}
}

// contentAllowed is the content row: only in records of the foreign sink, for an instance-wide
// value or one whose scope the viewer may read.
func contentAllowed(c Canary, r Record, rules AllowRules) bool {
	if r.Sink != SinkForeign {
		return false
	}
	return c.Scope == "" || rules.Readable[r.Viewer][c.Scope]
}

// forms returns the texts Scan looks for for c (README.md "Matching").
func forms(c Canary) []string {
	switch c.Class {
	case ClassNetwork:
		return ipForms(c.Value)
	case ClassIdentity:
		return []string{c.Value, strings.ReplaceAll(c.Value, "-", "")}
	}
	if payload, sig, ok := jwsSegments(c.Value); ok {
		return []string{payload, sig}
	}
	return append(encodedForms([]byte(c.Value)), c.Value)
}

// ipForms returns the dotted IPv4 form, or for IPv6 the RFC 5952 compressed and the fully
// expanded form.
func ipForms(v string) []string {
	a, err := netip.ParseAddr(v)
	if err != nil {
		return []string{v}
	}
	if a.Is4() {
		return []string{a.String()}
	}
	return []string{a.String(), a.StringExpanded()}
}

// encodedForms returns the lowercase hex of raw and the parts of its base64 encodings (standard
// and URL alphabet, with or without padding) that stay the same at each of the three byte
// alignments raw can have within a longer encoded text.
func encodedForms(raw []byte) []string {
	out := []string{hex.EncodeToString(raw)}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding} {
		for k := range 3 {
			buf := append(make([]byte, k), raw...)
			s := enc.EncodeToString(buf)
			// The first characters mix prefix bits; the last group mixes the bits that follow.
			skip := (k*8 + 5) / 6
			end := (len(buf) / 3) * 4
			if end > skip {
				out = append(out, s[skip:end])
			}
		}
	}
	return out
}

// jwsSegments returns the payload and signature segments of a compact JWS whose header decodes
// to a JSON object with alg; the header segment repeats across tokens and is no secret.
func jwsSegments(v string) (payload, sig string, ok bool) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	h, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", "", false
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if json.Unmarshal(h, &header) != nil || header.Alg == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}
