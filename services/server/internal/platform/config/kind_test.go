// SPDX-License-Identifier: Apache-2.0

package config

import (
	"math"
	"net/netip"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// parseCase is one input of a parser table: want and canon are checked when ok is true; when ok
// is false the error must not contain the input (02 "Konfigurationsladen" item 4: messages never
// show a value).
type parseCase struct {
	in    string
	ok    bool
	want  any
	canon string
}

func runParseCases(t *testing.T, k Kind, enum []string, cases []parseCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(k.String()+"/"+c.in, func(t *testing.T) {
			t.Parallel()
			got, canon, err := parseValue(k, enum, c.in)
			if !c.ok {
				if err == nil {
					t.Fatalf("parseValue(%s, %q) = %v, want an error", k, c.in, got)
				}
				// Blank inputs occur in every text; they carry nothing to leak.
				if strings.TrimSpace(c.in) != "" && strings.Contains(err.Error(), c.in) {
					t.Fatalf("error %q echoes the input", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseValue(%s, %q) error = %v", k, c.in, err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("parseValue(%s, %q) = %#v, want %#v", k, c.in, got, c.want)
			}
			if canon != c.canon {
				t.Errorf("canonical text of %q = %q, want %q", c.in, canon, c.canon)
			}
			// The canonical text parses to the same value: print output can be pasted back.
			again, canon2, err := parseValue(k, enum, canon)
			if err != nil || !reflect.DeepEqual(again, got) || canon2 != canon {
				t.Errorf("canonical text %q does not round-trip: %#v, %q, %v", canon, again, canon2, err)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	t.Parallel()
	want := map[Kind]string{
		KindString: "string", KindEnum: "enum", KindBool: "bool", KindInt: "int", KindFloat: "float",
		KindDuration: "duration", KindStringList: "list(string)", KindCIDRList: "list(cidr)",
		KindListenAddr: "listen address", KindCron: "cron", KindSemVer: "semver",
		KindSecretRef: "secret_ref", KindURL: "url",
	}
	for k, s := range want {
		if k.String() != s {
			t.Errorf("Kind(%d).String() = %q, want %q", int(k), k.String(), s)
		}
	}
	if got := Kind(0).String(); got != "Kind(0)" {
		t.Errorf("Kind(0).String() = %q, want %q", got, "Kind(0)")
	}
	if Kind(0).valid() || Kind(99).valid() || !KindURL.valid() {
		t.Error("valid() must accept exactly the declared kinds")
	}
}

func TestDurationParse(t *testing.T) {
	t.Parallel()
	runParseCases(t, KindDuration, nil, []parseCase{
		{in: "250ms", ok: true, want: 250 * time.Millisecond, canon: "250ms"},
		{in: "5s", ok: true, want: 5 * time.Second, canon: "5s"},
		{in: "2m", ok: true, want: 2 * time.Minute, canon: "2m"},
		{in: "24h", ok: true, want: 24 * time.Hour, canon: "1d"},
		{in: "7d", ok: true, want: 168 * time.Hour, canon: "7d"},
		{in: "0s", ok: true, want: time.Duration(0), canon: "0s"},
		{in: "0ms", ok: true, want: time.Duration(0), canon: "0s"},
		{in: "120s", ok: true, want: 2 * time.Minute, canon: "2m"},
		{in: "90s", ok: true, want: 90 * time.Second, canon: "90s"},
		{in: "1500ms", ok: true, want: 1500 * time.Millisecond, canon: "1500ms"},
		{in: "106751d", ok: true, want: 106751 * 24 * time.Hour, canon: "106751d"},
		// The limit is the largest time.Duration, so it differs per unit.
		{in: "2562047h", ok: true, want: 2562047 * time.Hour, canon: "2562047h"},
		{in: "2562048h"},
		{in: "1.5h"},
		{in: "1h30m"},
		{in: "5"},
		{in: "-1s"},
		{in: "5S"},
		{in: "99999999999999d"},
		{in: "106752d"},
		{in: "05s"},
		{in: "+5s"},
		{in: " 5s"},
		{in: "5us"},
		{in: ""},
	})
}

func TestScalarKinds(t *testing.T) {
	t.Parallel()
	runParseCases(t, KindString, nil, []parseCase{
		{in: "", ok: true, want: "", canon: ""},
		{in: " any text # with = signs ", ok: true, want: " any text # with = signs ", canon: " any text # with = signs "},
	})
	runParseCases(t, KindBool, nil, []parseCase{
		{in: "true", ok: true, want: true, canon: "true"},
		{in: "false", ok: true, want: false, canon: "false"},
		{in: "yes"}, {in: "True"}, {in: "1"}, {in: "on"}, {in: ""},
	})
	// The int64 limits are built at run time: a long digit string in the source would read like an
	// instance ID to the white-label rules.
	maxInt := strconv.FormatInt(math.MaxInt64, 10)
	tooBig := strconv.FormatUint(uint64(math.MaxInt64)+1, 10)
	runParseCases(t, KindInt, nil, []parseCase{
		{in: "1048576", ok: true, want: int64(1048576), canon: "1048576"},
		{in: "-12", ok: true, want: int64(-12), canon: "-12"},
		{in: "0", ok: true, want: int64(0), canon: "0"},
		{in: "007", ok: true, want: int64(7), canon: "7"},
		{in: maxInt, ok: true, want: int64(math.MaxInt64), canon: maxInt},
		{in: tooBig},
		{in: "+5"}, {in: "0x10"}, {in: "1e3"}, {in: "1_000"}, {in: "1.0"}, {in: " 1"}, {in: ""},
	})
	runParseCases(t, KindFloat, nil, []parseCase{
		{in: "0.05", ok: true, want: 0.05, canon: "0.05"},
		{in: "5", ok: true, want: 5.0, canon: "5"},
		{in: "-2.5", ok: true, want: -2.5, canon: "-2.5"},
		{in: "1e-3", ok: true, want: 0.001, canon: "0.001"},
		{in: ".5", ok: true, want: 0.5, canon: "0.5"},
		{in: "1e400"}, {in: "inf"}, {in: "NaN"}, {in: "0x1p-2"}, {in: "+1"}, {in: "5."}, {in: "1_0"}, {in: ""},
	})
	enum := []string{"api", "worker", "all"}
	runParseCases(t, KindEnum, enum, []parseCase{
		{in: "worker", ok: true, want: "worker", canon: "worker"},
		{in: "Worker"}, {in: "workers"}, {in: ""},
	})
	runParseCases(t, KindSemVer, nil, []parseCase{
		{in: "1.4.0", ok: true, want: "1.4.0", canon: "1.4.0"},
	})
}

func TestEnumErrorNamesAllowedValues(t *testing.T) {
	t.Parallel()
	_, _, err := parseValue(KindEnum, []string{"api", "worker", "all"}, "apii")
	if err == nil || err.Error() != "must be one of api, worker, all" {
		t.Fatalf("error = %v, want the list of allowed values", err)
	}
}

func TestListFromEnvCommaAndJSON(t *testing.T) {
	t.Parallel()
	runParseCases(t, KindStringList, nil, []parseCase{
		{in: "a, b ,c", ok: true, want: []string{"a", "b", "c"}, canon: "a, b, c"},
		{in: `["a","b"]`, ok: true, want: []string{"a", "b"}, canon: "a, b"},
		{in: "", ok: true, want: []string{}, canon: ""},
		{in: "[]", ok: true, want: []string{}, canon: ""},
		{in: "single", ok: true, want: []string{"single"}, canon: "single"},
		{in: `["x,y"," z"]`, ok: true, want: []string{"x,y", " z"}, canon: `["x,y"," z"]`},
		{in: `["[a"]`, ok: true, want: []string{"[a"}, canon: `["[a"]`},
		{in: `["<&>"]`, ok: true, want: []string{"<&>"}, canon: "<&>"},
		{in: "a,,b"},
		{in: "a,"},
		{in: " "},
		{in: `["a",""]`},
		{in: `["a",null]`},
		{in: `["a",1]`},
		{in: `["a"] trailing`},
		{in: `[broken`},
	})
}

func TestCIDRList(t *testing.T) {
	t.Parallel()
	p := netip.MustParsePrefix
	runParseCases(t, KindCIDRList, nil, []parseCase{
		{in: "127.0.0.1/32, ::1/128", ok: true, want: []netip.Prefix{p("127.0.0.1/32"), p("::1/128")}, canon: "127.0.0.1/32, ::1/128"},
		{in: `["192.0.2.0/24","2001:DB8::/32"]`, ok: true, want: []netip.Prefix{p("192.0.2.0/24"), p("2001:db8::/32")}, canon: "192.0.2.0/24, 2001:db8::/32"},
		{in: "", ok: true, want: []netip.Prefix{}, canon: ""},
		{in: "192.0.2.10/24"},
		{in: "2001:db8::1/32"},
		{in: "192.0.2.10"},
		{in: "198.51.100.0/33"},
		{in: "192.0.2.0/24,,198.51.100.0/24"},
	})
}

func TestListenAddr(t *testing.T) {
	t.Parallel()
	runParseCases(t, KindListenAddr, nil, []parseCase{
		{in: "127.0.0.1:8080", ok: true, want: "127.0.0.1:8080", canon: "127.0.0.1:8080"},
		{in: ":8080", ok: true, want: ":8080", canon: ":8080"},
		{in: "[::]:0", ok: true, want: "[::]:0", canon: "[::]:0"},
		{in: "[0:0::0]:65535", ok: true, want: "[::]:65535", canon: "[::]:65535"},
		{in: "[::1]:443", ok: true, want: "[::1]:443", canon: "[::1]:443"},
		{in: "localhost:8080"},
		{in: "example.com:8080"},
		{in: "127.0.0.1"},
		{in: "127.0.0.1:65536"},
		{in: "127.0.0.1:-1"},
		{in: "127.0.0.1:http"},
		{in: "127.0.0.1:08080"},
		{in: "127.0.0.1:"},
		{in: "::1:80"},
		{in: ""},
	})
}

func TestCron(t *testing.T) {
	t.Parallel()
	runParseCases(t, KindCron, nil, []parseCase{
		{in: "0 2 * * 0", ok: true, want: "0 2 * * 0", canon: "0 2 * * 0"},
		{in: "0 2 * * 1-6", ok: true, want: "0 2 * * 1-6", canon: "0 2 * * 1-6"},
		{in: "*/15  0-23/2 1,15 1-12 *", ok: true, want: "*/15 0-23/2 1,15 1-12 *", canon: "*/15 0-23/2 1,15 1-12 *"},
		{in: "59 23 31 12 6", ok: true, want: "59 23 31 12 6", canon: "59 23 31 12 6"},
		{in: "0,30 * * * *", ok: true, want: "0,30 * * * *", canon: "0,30 * * * *"},
		{in: "0 2 * *"},
		{in: "0 2 * * 0 0"},
		{in: "60 * * * *"},
		{in: "* 24 * * *"},
		{in: "* * 0 * *"},
		{in: "* * 32 * *"},
		{in: "* * * 0 *"},
		{in: "* * * 13 *"},
		{in: "* * * * 7"},
		{in: "* * * JAN *"},
		{in: "* * * * MON"},
		{in: "5-1 * * * *"},
		{in: "*/0 * * * *"},
		{in: "*/60 * * * *"},
		{in: "5/2 * * * *"},
		{in: "1,,2 * * * *"},
		{in: "05 * * * *"},
		{in: "@daily"},
		{in: "? * * * *"},
		{in: ""},
	})
}

func TestSemVer(t *testing.T) {
	t.Parallel()
	runParseCases(t, KindSemVer, nil, []parseCase{
		{in: "0.0.0", ok: true, want: "0.0.0", canon: "0.0.0"},
		{in: "1.4.0-rc.1+build.5", ok: true, want: "1.4.0-rc.1+build.5", canon: "1.4.0-rc.1+build.5"},
		{in: "10.20.30-alpha-1", ok: true, want: "10.20.30-alpha-1", canon: "10.20.30-alpha-1"},
		{in: "v1.4.0"},
		{in: "1.4"},
		{in: "01.4.0"},
		{in: "1.4.0-01"},
		{in: "1.4.0-"},
		{in: "1.4.0+"},
		{in: ""},
	})
}

func TestURL(t *testing.T) {
	t.Parallel()
	runParseCases(t, KindURL, nil, []parseCase{
		{in: "https://chat.example.com", ok: true, want: "https://chat.example.com", canon: "https://chat.example.com"},
		{in: "http://127.0.0.1:8080/api", ok: true, want: "http://127.0.0.1:8080/api", canon: "http://127.0.0.1:8080/api"},
		{in: "https://[2001:db8::1]:8443", ok: true, want: "https://[2001:db8::1]:8443", canon: "https://[2001:db8::1]:8443"},
		{in: "ftp://example.com"},
		{in: "https://user@example.com"},
		{in: "https://example.com/#frag"},
		{in: "https://example.com#"},
		{in: "https://"},
		{in: "https://:443"},
		{in: "https:example.com"},
		{in: "example.com"},
		{in: "https://example.com:0"},
		{in: "https://example.com:65536"},
		{in: "https://example.com:0443"},
		{in: "https://exa mple.com"},
		{in: ""},
	})
}

func TestSecretRefSchemes(t *testing.T) {
	t.Parallel()
	runParseCases(t, KindSecretRef, nil, []parseCase{
		{in: "file:/run/bothy/secrets/db_app_password", ok: true, want: "file:/run/bothy/secrets/db_app_password", canon: "file:/run/bothy/secrets/db_app_password"},
		{in: "env:BOTHY_SECRET_DB_APP_PASSWORD", ok: true, want: "env:BOTHY_SECRET_DB_APP_PASSWORD", canon: "env:BOTHY_SECRET_DB_APP_PASSWORD"},
		{in: "env:_X1", ok: true, want: "env:_X1", canon: "env:_X1"},
		{in: "env:DB_PASSWORD", ok: true, want: "env:DB_PASSWORD", canon: "env:DB_PASSWORD"},
		{in: "env:BOTHY_CONFIG_FILE"},
		{in: "env:BOTHY_SERVER__ROLE"},
		{in: "env:BOTHY_SECRET__DB"},
		{in: "file:run/bothy/secrets/x"},
		{in: "file:"},
		{in: "env:bothy_secret"},
		{in: "env:1X"},
		{in: "env:"},
		{in: "/run/bothy/secrets/x"},
		{in: "hunter2"},
		{in: ":x"},
		{in: "vault:secret/x"},
		{in: "FILE:/x"},
		{in: ""},
	})
	for _, ref := range []string{"azkv://vault-name/secret-name", "awssm://eu-central-1/bothy/db_app_password"} {
		_, _, err := parseValue(KindSecretRef, nil, ref)
		if err == nil || err.Error() != "scheme not supported by this server version" {
			t.Errorf("parseValue(%q) error = %v, want %q", ref, err, "scheme not supported by this server version")
		}
	}
}

func TestParseElementsFromSequence(t *testing.T) {
	t.Parallel()
	got, canon, err := parseElements(KindCIDRList, []string{"192.0.2.0/24", "::1/128"})
	if err != nil || canon != "192.0.2.0/24, ::1/128" || len(got.([]netip.Prefix)) != 2 {
		t.Fatalf("parseElements = %v, %q, %v", got, canon, err)
	}
	if _, _, err := parseElements(KindStringList, []string{"a", ""}); err == nil {
		t.Fatal("an empty element must be an error")
	}
	if _, _, err := parseElements(KindCIDRList, []string{"192.0.2.10/24"}); err == nil || strings.Contains(err.Error(), "192.0.2.10") {
		t.Fatalf("error = %v, want an error without the element", err)
	}
}

func TestEnvReference(t *testing.T) {
	t.Parallel()
	if v, ok := envReference("env:BOTHY_SECRET_X"); !ok || v != "BOTHY_SECRET_X" {
		t.Errorf("envReference(env:BOTHY_SECRET_X) = %q, %v", v, ok)
	}
	for _, ref := range []string{"file:/x", "env:", "env:lower", "BOTHY_SECRET_X"} {
		if v, ok := envReference(ref); ok {
			t.Errorf("envReference(%q) = %q, true; want false", ref, v)
		}
	}
}
