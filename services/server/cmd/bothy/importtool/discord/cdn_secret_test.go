// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// cdnSecretForms are the parts of the signed links that must never appear in output.
func cdnSecretForms() []string {
	return []string{staleQuery, freshQuery, staleSig, freshSig, staleExpires, freshExpires, staleIssued, freshIssued,
		"hm=", "ex=", "is=", "error for", testToken}
}

// TestCDNNoSecretInOutput (22 6.5 item 4, 16 "No secret in output"): every error text and every
// debug log line of the fetcher, over refusals, redirects, rate limits, oversize answers,
// expired links and error bodies that echo the request URL, contains neither the query nor the
// signature values nor the echoed body.
func TestCDNNoSecretInOutput(t *testing.T) {
	var out strings.Builder
	record := func(v any, err error) {
		fmt.Fprintf(&out, "%v %+v %#v\n", v, v, v)
		if err != nil {
			fmt.Fprintf(&out, "%v %+v %#v %s\n", err, err, err, err.Error())
		}
	}
	scenarios := map[string]func(h *cdnHarness){
		"echo 400": func(h *cdnHarness) { h.cdn.Always(cdnPath, discordtest.CDNEcho(http.StatusBadRequest)) },
		"echo 403": func(h *cdnHarness) { h.cdn.Always(cdnPath, discordtest.CDNEcho(http.StatusForbidden)) },
		"echo 404": func(h *cdnHarness) { h.cdn.Always(cdnPath, discordtest.CDNEcho(http.StatusNotFound)) },
		"echo 429": func(h *cdnHarness) { h.cdn.Always(cdnPath, discordtest.CDNEcho(http.StatusTooManyRequests)) },
		"echo 500": func(h *cdnHarness) { h.cdn.Always(cdnPath, discordtest.CDNEcho(http.StatusInternalServerError)) },
		"redirect": func(h *cdnHarness) {
			h.cdn.Always(cdnPath, discordtest.CDNRedirect(h.cdn.URL(discordtest.ForeignHost, cdnPath, staleQuery)))
		},
		"too large": func(h *cdnHarness) { h.cdn.AddFile(cdnPath, discordtest.CDNFile{Size: cdnMaxBytes + 1}) },
		"lie":       func(h *cdnHarness) { h.cdn.Always(cdnPath, discordtest.CDNLie(10, cdnMaxBytes+1)) },
		"expired": func(h *cdnHarness) {
			h.cdn.AddFile(cdnPath, discordtest.CDNFile{Size: cdnFileSize, Signature: freshSig})
		},
	}
	for name, setup := range scenarios {
		h := newCDNHarness(t, func(c *CDNConfig) { c.MaxRetries = 1 })
		setup(h)
		for _, host := range []string{discordtest.CDNHost, discordtest.ForeignHost} {
			ref := h.ref(h.staleURL(host))
			record(ref, nil)
			d, err := h.fetcher.Fetch(t.Context(), ref)
			record(d, err)
			if err == nil {
				_, err = io.Copy(io.Discard, d.Body)
				record(nil, err)
				_ = d.Body.Close()
			}
		}
		for _, raw := range []string{"http://" + discordtest.CDNHost + cdnPath + "?" + staleQuery,
			"https://alex:" + staleSig + "@" + discordtest.CDNHost + cdnPath + "?" + staleQuery + "#" + freshSig} {
			_, err := h.fetcher.Fetch(t.Context(), h.ref(raw))
			record(nil, err)
		}
		record(h.fetcher, nil)
		out.WriteString(h.logs.String())
		if h.logs.Len() == 0 {
			t.Fatalf("%s: no debug log captured", name)
		}
	}
	for _, s := range cdnSecretForms() {
		if i := strings.Index(out.String(), s); i >= 0 {
			t.Fatalf("output contains %q near %q", s, out.String()[max(0, i-200):min(out.Len(), i+50)])
		}
	}
}

// TestSafeURL: SafeURL keeps scheme, host and path and drops query, fragment and user info.
func TestSafeURL(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"https://cdn.example.com/attachments/1/2/a.png?ex=1&is=2&hm=3&", "https://cdn.example.com/attachments/1/2/a.png"},
		{"https://cdn.example.com/a.png#frag", "https://cdn.example.com/a.png"},
		{"https://cdn.example.com/a.png?#", "https://cdn.example.com/a.png"},
		{"https://alex:pw@cdn.example.com:8443/a.png?x=1", "https://cdn.example.com:8443/a.png"},
		{"https://[2001:db8::1]:443/a.png?hm=3", "https://[2001:db8::1]:443/a.png"},
		{"https://cdn.example.com/a%20b/c%2Fd.png?hm=3", "https://cdn.example.com/a%20b/c%2Fd.png"},
		{"https://cdn.example.com/%3Fhm=3/x", "https://cdn.example.com/%3Fhm=3/x"},
		{"https://cdn.example.com", "https://cdn.example.com"},
		{"not a url\x7f?hm=3", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := SafeURL(c.raw); got != c.want {
			t.Errorf("SafeURL(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}
