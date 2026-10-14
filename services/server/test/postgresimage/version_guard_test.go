// SPDX-License-Identifier: Apache-2.0

//go:build unix

package postgresimage_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// guard is the version guard of the image, relative to this package directory.
const guard = "../../images/bothy-postgres/check-pgbackrest-version.sh"

// fakeVersion writes a pgbackrest stand-in that prints out for "pgbackrest version".
func fakeVersion(t *testing.T, out string, code int) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "pgbackrest")
	script := "#!/bin/sh\n[ \"$1\" = version ] || exit 99\nprintf '%s\\n' '" + out + "'\nexit " + strconv.Itoa(code) + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // G306: the fake must be executable.
		t.Fatal(err)
	}
	return bin
}

// TestVersionGuard runs the guard of the Dockerfile and the entry point against fake binaries
// and reported versions: versions below 2.59.3 (16 8.1 rationale 4) and unreadable output fail.
func TestVersionGuard(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args []string
		ok   bool
	}{
		{"binary at the minimum", []string{fakeVersion(t, "pgBackRest 2.59.3", 0)}, true},
		{"binary above", []string{fakeVersion(t, "pgBackRest 2.60.0", 0)}, true},
		{"binary of a later major", []string{fakeVersion(t, "pgBackRest 3.0", 0)}, true},
		{"binary below", []string{fakeVersion(t, "pgBackRest 2.59.2", 0)}, false},
		{"old minor with a large patch", []string{fakeVersion(t, "pgBackRest 2.58.10", 0)}, false},
		{"old release", []string{fakeVersion(t, "pgBackRest 2.55.1", 0)}, false},
		{"development version", []string{fakeVersion(t, "pgBackRest 2.60dev", 0)}, false},
		{"other output", []string{fakeVersion(t, "something else", 0)}, false},
		{"binary fails", []string{fakeVersion(t, "pgBackRest 2.59.3", 1)}, false},
		{"missing binary", []string{filepath.Join(t.TempDir(), "missing")}, false},
		{"reported at the minimum", []string{"--reported", "pgBackRest 2.59.3"}, true},
		{"reported below", []string{"--reported", "pgBackRest 2.59.1"}, false},
		{"reported empty", []string{"--reported", ""}, false},
	} {
		out, err := exec.CommandContext(t.Context(), guard, tc.args...).CombinedOutput() //nolint:gosec // G204: test inputs.
		if (err == nil) != tc.ok {
			t.Errorf("%s: err %v, want ok %v; output %s", tc.name, err, tc.ok, out)
		}
		if !tc.ok && !strings.Contains(string(out), "2.59.3") {
			t.Errorf("%s: the message does not name the minimum: %s", tc.name, out)
		}
	}
}
