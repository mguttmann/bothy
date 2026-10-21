// SPDX-License-Identifier: Apache-2.0

package smoke

import "testing"

// TestGatewayHTU: the HTTPS form of the gateway URL of the public origin, as the server builds it
// (gateway HTU of M1-33): the scheme becomes https, a path prefix and a trailing slash do not
// count, a query or fragment is rejected.
func TestGatewayHTU(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in, want string
		fail     bool
	}{
		{in: "https://chat.example.com", want: "https://chat.example.com/api/v1/gateway"},
		{in: "https://chat.example.com/", want: "https://chat.example.com/api/v1/gateway"},
		{in: "https://chat.example.com/bothy", want: "https://chat.example.com/api/v1/gateway"},
		{in: "https://chat.example.com:8443", want: "https://chat.example.com:8443/api/v1/gateway"},
		{in: "http://chat.example.com:80", want: "https://chat.example.com/api/v1/gateway"},
		{in: "https://chat.example.com/?x=1", fail: true},
		{in: "https://chat.example.com/#f", fail: true},
		{in: "chat.example.com", fail: true},
		{in: "", fail: true},
	} {
		got, err := GatewayHTU(tc.in)
		if tc.fail {
			if err == nil {
				t.Errorf("GatewayHTU(%q) = %q, want an error", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("GatewayHTU(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}

// TestRESTHTU: the origin of the public URL plus the request path, never the loopback address
// the probe dials.
func TestRESTHTU(t *testing.T) {
	t.Parallel()
	got, err := RESTHTU("https://chat.example.com/", "/api/v1/threads/0192f000-0000-7000-8000-000000000001/messages")
	if err != nil || got != "https://chat.example.com/api/v1/threads/0192f000-0000-7000-8000-000000000001/messages" {
		t.Errorf("RESTHTU = %q, %v", got, err)
	}
	if _, err := RESTHTU("https://chat.example.com", "api/v1/me"); err == nil {
		t.Error("a relative path must be rejected")
	}
	if _, err := RESTHTU("https://chat.example.com?x=1", "/api/v1/me"); err == nil {
		t.Error("a public URL with a query must be rejected")
	}
}
