// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"strings"
	"testing"
	"testing/fstest"
)

const upOnly = "-- +goose Up\nSELECT 1;\n"

// migrationFS returns a map file system with one goose migration per name.
func migrationFS(names ...string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for _, n := range names {
		fsys[n] = &fstest.MapFile{Data: []byte(upOnly + "\n-- +goose Down\nSELECT 1;\n")}
	}
	return fsys
}

func TestValidate(t *testing.T) {
	t.Parallel()
	good := migrationFS("00001_base_privileges.sql", "00002_channels.sql", "00003_messages_2.sql")
	good["README.md"] = &fstest.MapFile{Data: []byte("not a migration")}
	if err := Validate(good); err != nil {
		t.Fatalf("valid set rejected: %v", err)
	}

	cases := []struct {
		name string
		fsys fstest.MapFS
		want string
	}{
		{"empty", fstest.MapFS{}, "no migrations"},
		{"gap", migrationFS("00001_a.sql", "00003_c.sql"), "version 2 is missing"},
		{"not from 1", migrationFS("00002_b.sql"), "version 1 is missing"},
		{"duplicate", migrationFS("00001_a.sql", "00001_b.sql"), "duplicate version 1"},
		{"version zero", migrationFS("00000_zero.sql", "00001_a.sql"), "00000_zero.sql"},
		{"bad name upper case", migrationFS("00001_Base.sql"), "00001_Base.sql"},
		{"bad name four digits", migrationFS("0001_a.sql"), "0001_a.sql"},
		{"bad name no slug", migrationFS("00001.sql"), "00001.sql"},
		{"timestamp version", migrationFS("20260101120000_a.sql"), "20260101120000_a.sql"},
		{"unparsable", fstest.MapFS{
			"00001_a.sql": {Data: []byte("-- +goose Up\n-- +goose StatementBegin\nSELECT 1;\n")},
		}, "00001_a.sql"},
		{"missing up annotation", fstest.MapFS{
			"00001_a.sql": {Data: []byte("SELECT 1;\n")},
		}, "00001_a.sql"},
		{"environment substitution", fstest.MapFS{
			"00001_a.sql": {Data: []byte("-- +goose Up\n-- +goose ENVSUB ON\nSELECT '${HOME}';\n")},
		}, "ENVSUB"},
		// goose compares annotations in any case and strips every leading "--".
		{"environment substitution lower case", fstest.MapFS{
			"00001_a.sql": {Data: []byte("-- +goose envsub on\n-- +goose Up\nSELECT '${HOME}';\n")},
		}, "ENVSUB"},
		{"environment substitution doubled comment", fstest.MapFS{
			"00001_a.sql": {Data: []byte("-- +goose Up\n-- -- +goose Envsub On\nSELECT '${HOME}';\n")},
		}, "ENVSUB"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(tc.fsys)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestRequiredSchemaVersion(t *testing.T) {
	t.Parallel()
	got, err := RequiredSchemaVersion(migrationFS("00002_b.sql", "00001_a.sql", "00010_j.sql",
		"00003_c.sql", "00004_d.sql", "00005_e.sql", "00006_f.sql", "00007_g.sql", "00008_h.sql", "00009_i.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if got != 10 {
		t.Errorf("RequiredSchemaVersion = %d, want 10", got)
	}
	if _, err := RequiredSchemaVersion(migrationFS("00001_a.sql", "00003_c.sql")); err == nil {
		t.Error("an invalid set must not yield a version")
	}
}
