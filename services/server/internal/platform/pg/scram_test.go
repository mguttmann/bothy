// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"regexp"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// TestSCRAMVerifierVector compares with a verifier computed independently with Python's
// hashlib.pbkdf2_hmac and hmac (RFC 5802, RFC 7677) for the same password, salt and count.
func TestSCRAMVerifierVector(t *testing.T) {
	t.Parallel()
	salt := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	got, err := scramVerifier([]byte("pencil-EXAMPLE"), salt, 4096)
	if err != nil {
		t.Fatal(err)
	}
	const want = "SCRAM-SHA-256$4096:AAECAwQFBgcICQoLDA0ODw==$CKII7TVSugX4Cjfl/GRP7CEVxTD369FMrhRTkMrwMmo=:4CgUUvPeejESfRQ07dmAK78D0cro3myNiN1n2DUDEBk="
	if got != want {
		t.Errorf("verifier = %s\nwant       %s", got, want)
	}
}

func TestNewSCRAMVerifierSaltsEveryCall(t *testing.T) {
	t.Parallel()
	pattern := regexp.MustCompile(`^SCRAM-SHA-256\$4096:[A-Za-z0-9+/]{22}==\$[A-Za-z0-9+/]{43}=:[A-Za-z0-9+/]{43}=$`)
	a, err := newSCRAMVerifier([]byte("same-EXAMPLE"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := newSCRAMVerifier([]byte("same-EXAMPLE"))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{a, b} {
		if !pattern.MatchString(v) {
			t.Errorf("verifier %q has not the form of PostgreSQL", v)
		}
		if strings.Contains(v, "same-EXAMPLE") {
			t.Error("verifier contains the password")
		}
	}
	if a == b {
		t.Error("two verifiers of one password are equal: the salt is not random")
	}
}

func TestBootstrapRejectsUnsafePassword(t *testing.T) {
	t.Parallel()
	good := func() map[Role]secrets.Value {
		m := map[Role]secrets.Value{}
		for _, r := range Roles() {
			m[r] = secrets.NewValue([]byte("generated-EXAMPLE-" + string(r)))
		}
		return m
	}
	if err := checkPasswords(good()); err != nil {
		t.Fatalf("valid passwords rejected: %v", err)
	}
	cases := []struct {
		name  string
		edit  func(map[Role]secrets.Value)
		canon string // a value that must never appear in the error
	}{
		{"missing role", func(m map[Role]secrets.Value) { delete(m, RoleMaint) }, ""},
		{"unknown role", func(m map[Role]secrets.Value) { m[Role("postgres")] = secrets.NewValue([]byte("x-EXAMPLE")) }, "x-EXAMPLE"},
		{"empty", func(m map[Role]secrets.Value) { m[RoleApp] = secrets.Value{} }, ""},
		{"not ASCII", func(m map[Role]secrets.Value) { m[RoleApp] = secrets.NewValue([]byte("pæss-EXAMPLE")) }, "pæss-EXAMPLE"},
		{"control character", func(m map[Role]secrets.Value) { m[RoleApp] = secrets.NewValue([]byte("line\nbreak-EXAMPLE")) }, "break-EXAMPLE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := good()
			tc.edit(m)
			err := checkPasswords(m)
			if err == nil {
				t.Fatal("accepted")
			}
			if tc.canon != "" && strings.Contains(err.Error(), tc.canon) {
				t.Errorf("error %q repeats the password", err)
			}
		})
	}
}

func TestRoleStatements(t *testing.T) {
	t.Parallel()
	const v = "SCRAM-SHA-256$4096:AAECAwQFBgcICQoLDA0ODw==$CKII7TVSugX4Cjfl/GRP7CEVxTD369FMrhRTkMrwMmo=:4CgUUvPeejESfRQ07dmAK78D0cro3myNiN1n2DUDEBk="
	got, err := roleStatement(true, RoleApp, v)
	if err != nil {
		t.Fatal(err)
	}
	want := `CREATE ROLE "bothy_app" WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD '` + v + `'`
	if got != want {
		t.Errorf("create = %s\nwant     %s", got, want)
	}
	got, err = roleStatement(false, RoleMonitor, v)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, `ALTER ROLE "bothy_monitor" WITH LOGIN NOSUPERUSER`) {
		t.Errorf("alter = %s", got)
	}
	for _, bad := range []string{"plain-EXAMPLE", v + "'; DROP ROLE x; --", "SCRAM-SHA-256$4096:x$y:z"} {
		if _, err := roleStatement(true, RoleApp, bad); err == nil {
			t.Errorf("verifier %q accepted", bad)
		}
	}
	if _, err := roleStatement(true, Role(`x"; DROP ROLE y; --`), v); err == nil {
		t.Error("unknown role accepted")
	}
}

func TestRoles(t *testing.T) {
	t.Parallel()
	got := Roles()
	want := []Role{"bothy_owner", "bothy_app", "bothy_maint", "bothy_monitor"}
	if len(got) != len(want) {
		t.Fatalf("Roles() = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Roles()[%d] = %s, want %s", i, got[i], want[i])
		}
	}
	if RoleOwner != "bothy_owner" || RoleApp != "bothy_app" || RoleMaint != "bothy_maint" || RoleMonitor != "bothy_monitor" {
		t.Error("role constants differ from 16-operations.md 7.1")
	}
}
