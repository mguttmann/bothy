// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// TestSameSite: the site is the scheme plus the registrable domain of the public suffix list,
// never the last two labels; a host without one (IP, localhost) is its own site; ports and case
// do not count (brief M6-17 Design note 2).
func TestSameSite(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b string
		want bool
	}{
		{"https://chat.example.com", "https://grafana.example.com/d/ops", true},
		{"https://chat.example.com", "https://CHAT.example.com:3443/d/ops", true},
		{"https://chat.example.co.uk", "https://grafana.example.co.uk/d/ops", true},
		{"https://a.example.co.uk", "https://b.other.co.uk/d/ops", false},
		{"https://chat.example.com", "https://grafana.example.net/d/ops", false},
		{"http://chat.example.com", "https://grafana.example.com/d/ops", false},
		{"https://localhost:8443", "https://grafana.localhost:9444/d/ops", false},
		{"https://localhost:8443", "https://localhost:9444/d/ops", true},
		{"https://192.0.2.1", "https://192.0.2.2/d/ops", false},
		{"https://chat.example.com", "", false},
		{"", "https://grafana.example.com", false},
	}
	for _, c := range cases {
		if got := httpserver.SameSite(c.a, c.b); got != c.want {
			t.Errorf("SameSite(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
