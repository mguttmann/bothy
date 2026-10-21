// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// TestParseOffsiteTarget holds the form s3:https://<host>[:<port>]/<bucket>[/<prefix>] of
// backup.offsite.target (docs/DECISIONS.md D-M7-02-4): the valid forms with their parts, Key and
// Endpoint, and the rejected ones (no s3: prefix, http, a bad bucket, a bad port, credentials or
// a query in the URL, a prefix segment of dots). Errors never echo the value.
func TestParseOffsiteTarget(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in       string
		want     domain.OffsiteTarget
		endpoint string
		key      string
	}{
		{"s3:https://s3.example.com/bothy-backup", domain.OffsiteTarget{Host: "s3.example.com", Port: 443, Bucket: "bothy-backup"},
			"https://s3.example.com:443", "restic"},
		{"s3:https://s3.example.com/bothy-backup/", domain.OffsiteTarget{Host: "s3.example.com", Port: 443, Bucket: "bothy-backup"},
			"https://s3.example.com:443", "restic"},
		{"s3:https://s3.example.com:9000/bothy-backup/instance-1", domain.OffsiteTarget{Host: "s3.example.com", Port: 9000, Bucket: "bothy-backup", Prefix: "instance-1"},
			"https://s3.example.com:9000", "instance-1/restic"},
		{"s3:https://192.0.2.10:8333/bothy.backup/a/b_c.d", domain.OffsiteTarget{Host: "192.0.2.10", Port: 8333, Bucket: "bothy.backup", Prefix: "a/b_c.d"},
			"https://192.0.2.10:8333", "a/b_c.d/restic"},
		{"s3:https://[2001:db8::1]:8333/bkt", domain.OffsiteTarget{Host: "2001:db8::1", Port: 8333, Bucket: "bkt"},
			"https://[2001:db8::1]:8333", "restic"},
	} {
		got, err := domain.ParseOffsiteTarget(c.in)
		if err != nil {
			t.Errorf("ParseOffsiteTarget(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseOffsiteTarget(%q) = %+v, want %+v", c.in, got, c.want)
		}
		if e := got.Endpoint(); e != c.endpoint {
			t.Errorf("%q: Endpoint = %q, want %q", c.in, e, c.endpoint)
		}
		if k := got.Key("restic"); k != c.key {
			t.Errorf("%q: Key = %q, want %q", c.in, k, c.key)
		}
	}

	for _, in := range []string{
		"",
		"https://s3.example.com/bothy-backup",
		"s3:http://s3.example.com/bothy-backup",
		"s3:ftp://s3.example.com/bothy-backup",
		"s3:https://s3.example.com",
		"s3:https://s3.example.com/",
		"s3:https://s3.example.com/Upper_Case",
		"s3:https://s3.example.com/ab",
		"s3:https://s3.example.com/-bucket",
		"s3:https://s3.example.com/" + strings.Repeat("b", 64),
		"s3:https://s3.example.com:0/bothy-backup",
		"s3:https://s3.example.com:65536/bothy-backup",
		"s3:https://s3.example.com:port/bothy-backup",
		"s3:https://alex:pw@s3.example.com/bothy-backup",
		"s3:https://s3.example.com/bothy-backup?x=1",
		"s3:https://s3.example.com/bothy-backup#frag",
		"s3:https://s3.example.com/bothy-backup/../other",
		"s3:https://s3.example.com/bothy-backup/a//b",
		"s3:https://s3.example.com/bothy-backup/a b",
		"s3:https://s3_example.com/bothy-backup",
		"s3:https:///bothy-backup",
	} {
		_, err := domain.ParseOffsiteTarget(in)
		if !errors.Is(err, domain.ErrInvalidOffsiteTarget) {
			t.Errorf("ParseOffsiteTarget(%q) = %v, want ErrInvalidOffsiteTarget", in, err)
			continue
		}
		if in != "" && strings.Contains(err.Error(), in) || strings.Contains(err.Error(), "alex") {
			t.Errorf("ParseOffsiteTarget(%q) = %q echoes the value", in, err)
		}
	}
}
