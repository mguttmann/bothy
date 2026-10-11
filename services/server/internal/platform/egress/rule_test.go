// SPDX-License-Identifier: Apache-2.0

package egress_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

func TestParseRule(t *testing.T) {
	t.Parallel()
	valid := []struct {
		in   string
		want egress.Rule
		text string
	}{
		{"api.example.com", egress.Rule{Host: "api.example.com"}, "api.example.com"},
		{"api.example.com:443", egress.Rule{Host: "api.example.com", Port: 443}, "api.example.com:443"},
		{"API.Example.COM:8443", egress.Rule{Host: "api.example.com", Port: 8443}, "api.example.com:8443"},
		{"clamd;allow_private", egress.Rule{Host: "clamd", AllowPrivate: true}, "clamd;allow_private"},
		{"stt_worker.example.com:65535", egress.Rule{Host: "stt_worker.example.com", Port: 65535}, "stt_worker.example.com:65535"},
		{"192.0.2.10:8443;allow_private", egress.Rule{Host: "192.0.2.10", Port: 8443, AllowPrivate: true}, "192.0.2.10:8443;allow_private"},
		{"[2001:db8::10]:8443;allow_private", egress.Rule{Host: "2001:db8::10", Port: 8443, AllowPrivate: true}, "[2001:db8::10]:8443;allow_private"},
		{"2001:DB8::10", egress.Rule{Host: "2001:db8::10"}, "2001:db8::10"},
		{"[2001:db8::10]", egress.Rule{Host: "2001:db8::10"}, "2001:db8::10"},
		{"[::ffff:192.0.2.1]:1", egress.Rule{Host: "192.0.2.1", Port: 1}, "192.0.2.1:1"},
		{"::1;allow_private", egress.Rule{Host: "::1", AllowPrivate: true}, "::1;allow_private"},
		{"svc.0xg1", egress.Rule{Host: "svc.0xg1"}, "svc.0xg1"},
	}
	for _, tc := range valid {
		got, err := egress.ParseRule(tc.in)
		if err != nil {
			t.Errorf("ParseRule(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseRule(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
		if s := got.String(); s != tc.text {
			t.Errorf("ParseRule(%q).String() = %q, want %q", tc.in, s, tc.text)
		}
		if again, err := egress.ParseRule(got.String()); err != nil || again != got {
			t.Errorf("ParseRule(%q) does not round-trip: %+v, %v", got.String(), again, err)
		}
	}

	invalid := []string{
		"",
		"*.example.com",
		"https://api.example.com",
		"api.example.com/path",
		"user@api.example.com",
		"api.example.com:0",
		"api.example.com:65536",
		"api.example.com:0443",
		"api.example.com:",
		"api.example.com:+443",
		"api.example.com;allow_public",
		"api.example.com;allow_private;allow_private",
		"api.example.com;",
		"api.example.com;ALLOW_PRIVATE",
		"api.example.com.",
		".example.com",
		"api..example.com",
		"-api.example.com",
		"api-.example.com",
		"bücher.example.com",
		" api.example.com",
		"api.example.com ",
		"fe80::1%en0",
		"[fe80::1%en0]:443",
		"[2001:db8::10",
		"[2001:db8::10]443",
		"[2001:db8::10]:0",
		"[2001:db8::10]:",
		"api.example.com:44a3",
		"[192.0.2.10]:443",
		"[api.example.com]:443",
		"192.0.2.10:443:1",
		"1.2.3",
		"api.example.0x1f",
		"0x7f000001",
		strings.Repeat("a", 64) + ".example.com",
		strings.Repeat("abcdefghi.", 26) + "com",
	}
	for _, in := range invalid {
		got, err := egress.ParseRule(in)
		if err == nil {
			t.Errorf("ParseRule(%q) = %+v, want an error", in, got)
			continue
		}
		// A value can be a secret in the wrong key; the message must never repeat it.
		if trimmed := strings.TrimSpace(in); len(trimmed) > 3 && strings.Contains(err.Error(), trimmed) {
			t.Errorf("ParseRule(%q): error %q repeats the input", in, err)
		}
		if !strings.HasPrefix(err.Error(), "egress: invalid rule: ") {
			t.Errorf("ParseRule(%q): error %q lacks the prefix", in, err)
		}
	}
}
