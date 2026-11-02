// SPDX-License-Identifier: Apache-2.0

package config

import "testing"

// TestOwnerAccount covers host_service.owner_account (ADR 0073, D-OA-1, D-OA-3, D-OA-10).
func TestOwnerAccount(t *testing.T) {
	c, err := Parse([]byte("host_service:\n  owner_account: alex\n"), linux)
	if err != nil {
		t.Fatal(err)
	}
	if c.OwnerAccount != "alex" || len(c.SessionUsers) != 1 || c.SessionUsers[0] != "alex" {
		t.Fatalf("empty session users are not filled in: %+v", c)
	}
	if c, err := Parse([]byte("host_service:\n  owner_account: alex\n  session_users: [alex]\n"), Platform{GOOS: "darwin"}); err != nil || c.OwnerAccount != "alex" {
		t.Fatalf("darwin: %v", err)
	}
	if c, err := Parse([]byte("host_service: {}\n"), linux); err != nil || c.OwnerAccount != "" {
		t.Fatalf("default is not empty: %v %+v", err, c)
	}
	for _, tc := range []struct {
		name, body string
		p          Platform
		key        string
	}{
		{"another session user", "host_service:\n  owner_account: alex\n  session_users: [alex, bothy-s-sam]\n", linux, "host_service.session_users"},
		{"a different session user", "host_service:\n  owner_account: alex\n  session_users: [bothy-s-sam]\n", linux, "host_service.session_users"},
		{"local confirmation", "host_service:\n  owner_account: alex\n  require_local_confirm: true\n", linux, "host_service.require_local_confirm"},
		{"windows", "host_service:\n  owner_account: alex\n", Platform{GOOS: "windows"}, "host_service.owner_account"},
		{"empty name", "host_service:\n  owner_account: \"\"\n", linux, "host_service.owner_account"},
		{"not a string", "host_service:\n  owner_account: [alex]\n", linux, "host_service.owner_account"},
	} {
		_, err := Parse([]byte(tc.body), tc.p)
		if err == nil {
			t.Errorf("%s: accepted", tc.name)
			continue
		}
		if k := keyOf(t, err); k != tc.key {
			t.Errorf("%s: key %q, want %q", tc.name, k, tc.key)
		}
	}
}
