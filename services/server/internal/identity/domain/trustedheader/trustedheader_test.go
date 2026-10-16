// SPDX-License-Identifier: Apache-2.0

package trustedheader_test

import (
	"encoding/json"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/trustedheader"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

func TestIsLoopbackListener(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true}, {"127.5.5.5", true}, {"::1", true},
		{"0.0.0.0", false}, {"::", false}, {"192.0.2.10", false}, {"2001:db8::1", false},
	}
	for _, c := range cases {
		addr := &net.TCPAddr{IP: net.ParseIP(c.ip), Port: 8443}
		if got := trustedheader.IsLoopbackListener(addr); got != c.want {
			t.Errorf("IsLoopbackListener(%s) = %v, want %v", c.ip, got, c.want)
		}
	}
	if trustedheader.IsLoopbackListener(nil) {
		t.Error("a missing address counts as loopback")
	}
	if trustedheader.IsLoopbackListener(&net.UnixAddr{Name: "/tmp/sock", Net: "unix"}) {
		t.Error("a unix socket counts as loopback")
	}
}

func TestDecodeHeaderValue(t *testing.T) {
	t.Parallel()
	ok := []struct{ in, want string }{
		{"alex@example.com", "alex@example.com"},
		{"=?utf-8?q?R=C3=B6bin?=", "Röbin"},
		{"=?UTF-8?B?UsO2Ymlu?=", "Röbin"},
		{"=?us-ascii?q?sam=40example.com?=", "sam@example.com"},
		{"Alex =?utf-8?q?R=C3=B6bin?=", "Alex Röbin"},
	}
	for _, c := range ok {
		got, valid := trustedheader.DecodeValue(c.in)
		if !valid || got != c.want {
			t.Errorf("DecodeValue(%q) = %q, %v; want %q, true", c.in, got, valid, c.want)
		}
	}
	absent := []string{
		"",
		"=?iso-8859-1?q?R=F6bin?=",
		"=?iso-8859-1?q?R=F6bin?x?=",
		"=?windows-1252?q?R=F6bin?=",
		"=?utf-8?q?bad=ZZ?=",
		"alex\x01@example.com",
		"=?utf-8?q?alex=0A@example.com?=",
		"alex\u202e@example.com",
		"alex\u200b@example.com",
		"=?utf-8?q?=E2=80=AE?=",
		"\xff\xfe",
		"=?utf-8?q??=",
	}
	for _, in := range absent {
		if got, valid := trustedheader.DecodeValue(in); valid {
			t.Errorf("DecodeValue(%q) = %q, true; want absent", in, got)
		}
	}
}

func TestLoginFromValues(t *testing.T) {
	t.Parallel()
	trusted := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	local := netip.MustParseAddr("127.0.0.1")
	cases := []struct {
		name    string
		peer    netip.Addr
		values  []string
		want    string
		outcome trustedheader.Outcome
	}{
		{"one value", local, []string{"alex@example.com"}, "alex@example.com", trustedheader.Present},
		{"untrusted peer", netip.MustParseAddr("192.0.2.7"), []string{"alex@example.com"}, "", trustedheader.UntrustedPeer},
		{"no peer", netip.Addr{}, []string{"alex@example.com"}, "", trustedheader.UntrustedPeer},
		{"missing", local, nil, "", trustedheader.Missing},
		{"two values", local, []string{"alex@example.com", "sam@example.com"}, "", trustedheader.Multiple},
		{"invalid", local, []string{"alex\u202e"}, "", trustedheader.Invalid},
	}
	for _, c := range cases {
		got, outcome := trustedheader.Evaluate(c.peer, trusted, c.values)
		if got != c.want || outcome != c.outcome {
			t.Errorf("%s: Evaluate = %q, %v; want %q, %v", c.name, got, outcome, c.want, c.outcome)
		}
	}
}

func TestTrustedHeaderConfigValidation(t *testing.T) {
	t.Parallel()
	cfg, err := trustedheader.ParseConfig(json.RawMessage(`{"login_header":"tailscale-user-login","name_header":"Tailscale-User-Name"}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LoginHeader != "Tailscale-User-Login" || cfg.NameHeader != "Tailscale-User-Name" {
		t.Fatalf("config = %+v, want canonical header names", cfg)
	}
	if cfg, err = trustedheader.ParseConfig(json.RawMessage(`{"login_header":"X-Login"}`)); err != nil || cfg.NameHeader != "" {
		t.Fatalf("login header only: %+v, %v", cfg, err)
	}
	invalid := map[string]string{
		`{"login_header":"X Login"}`:                         "/config/login_header",
		`{"login_header":"X-Login","name_header":"N(ame)"}`:  "/config/name_header",
		`{"login_header":"X-Login","extra":"x"}`:             "/config/extra",
		`{}`:                                                 "/config/login_header",
		`{"login_header":"X-Login","name_header":"x-login"}`: "/config/name_header",
	}
	for raw, pointer := range invalid {
		_, err := trustedheader.ParseConfig(json.RawMessage(raw))
		e, ok := errs.As(err)
		if !ok || e.PublicCode() != errs.CodeValidationFailed {
			t.Errorf("%s: err = %v, want validation_failed", raw, err)
			continue
		}
		found := false
		for _, f := range e.Fields {
			found = found || f.Pointer == pointer
		}
		if !found {
			t.Errorf("%s: fields %+v, want %s", raw, e.Fields, pointer)
		}
	}
}

func TestHandleFromLogin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		login string
		max   int
		want  string
	}{
		{"alex@example.com", 32, "alex"},
		{"Sam.Doe@example.com", 32, "sam.doe"},
		{"Röbin@example.com", 32, "rbin"},
		{"__x@example.com", 32, "x"},
		{"@example.com", 32, "user"},
		{"ÄÖÜ", 32, "user"},
		{"averyveryverylonglogin@example.com", 8, "averyver"},
		{"ab-@example.com", 3, "ab-"},
	}
	for _, c := range cases {
		if got := trustedheader.HandleFromLogin(c.login, c.max); got != c.want {
			t.Errorf("HandleFromLogin(%q, %d) = %q, want %q", c.login, c.max, got, c.want)
		}
	}
}

func TestHandleWithSuffix(t *testing.T) {
	t.Parallel()
	got := trustedheader.HandleWithSuffix("alex", "k3x9", 32)
	if got != "alex-k3x9" {
		t.Fatalf("suffix = %q", got)
	}
	got = trustedheader.HandleWithSuffix("averyverylong", "k3x9", 8)
	if got != "ave-k3x9" || len(got) > 8 || strings.HasPrefix(got, "-") {
		t.Fatalf("cut suffix = %q", got)
	}
}

// TestParseConfigRefusesAndShortHandles: an invalid configuration is an error, and a suffix longer
// than the room for a handle is cut to the room.
func TestParseConfigRefusesAndShortHandles(t *testing.T) {
	if _, err := trustedheader.ParseConfig([]byte(`{"login_header":""}`)); err == nil {
		t.Error("an empty login header passed")
	}
	if got := trustedheader.HandleWithSuffix("alex", "abcdef", 3); got != "abc" {
		t.Errorf("HandleWithSuffix = %q, want abc", got)
	}
}
