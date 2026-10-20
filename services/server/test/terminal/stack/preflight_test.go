// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeDocker writes an executable docker into a new folder that answers the calls of the
// preflight from the variables FAKE_* of its environment and returns the folder.
func fakeDocker(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
case "$1 $2" in
"buildx version") [ "${FAKE_BUILDX:-yes}" = yes ] || { echo "docker: unknown command: docker buildx" >&2; exit 1; }; echo "github.com/docker/buildx v0.38.0"; exit 0 ;;
"compose version") echo "${FAKE_COMPOSE:-2.40.0}"; exit 0 ;;
esac
case "$1" in
version) echo "${FAKE_ENGINE:-29.5.2}"; exit 0 ;;
info) echo "${FAKE_CGROUP:-2}"; exit 0 ;;
run) echo "${FAKE_PTRACE:-1}"; exit 0 ;;
esac
echo "fake docker: unexpected call $*" >&2
exit 3
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o700); err != nil { //nolint:gosec // G306: the fake must be executable
		t.Fatal(err)
	}
	return dir
}

// TestStackPreflightMessages proves requirement 18: a missing docker, a missing buildx, an old
// Engine, an old Compose, cgroup version 1 and a ptrace_scope below 1 each fail with the fix in the
// message; a machine that meets everything passes.
func TestStackPreflightMessages(t *testing.T) {
	bin := fakeDocker(t)
	cases := []struct {
		name string
		env  []string
		path string
		want []string
	}{
		{name: "passes", path: bin},
		{name: "missing_docker", path: t.TempDir(), want: []string{"docker is not on PATH", "install Docker Engine 26.0.0"}},
		{name: "missing_buildx", path: bin, env: []string{"FAKE_BUILDX=no"}, want: []string{"docker buildx is missing", "install the buildx plugin"}},
		{name: "old_engine", path: bin, env: []string{"FAKE_ENGINE=25.0.3"}, want: []string{"Docker Engine 25.0.3 is too old", "26.0.0 or newer"}},
		{name: "old_compose", path: bin, env: []string{"FAKE_COMPOSE=2.20.1"}, want: []string{"Docker Compose 2.20.1 is too old", "2.26.0 or newer"}},
		{name: "cgroup_v1", path: bin, env: []string{"FAKE_CGROUP=1"}, want: []string{"cgroup version 1", "cgroup version 2"}},
		{name: "ptrace_scope_0", path: bin, env: []string{"FAKE_PTRACE=0"}, want: []string{"kernel.yama.ptrace_scope is 0", "ptrace_scope_low", "at least 1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &Docker{Bin: lookIn(tc.path), Env: append([]string{"PATH=" + tc.path}, tc.env...)}
			err := Preflight(t.Context(), d)
			if err == nil {
				err = PtraceScope(t.Context(), d, "termsuite-host:test")
			}
			if len(tc.want) == 0 {
				if err != nil {
					t.Fatalf("preflight failed on a machine that meets everything: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("preflight passed, want a failure naming %q", tc.want)
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("message %q does not name %q", err, w)
				}
			}
		})
	}
}

// lookIn returns the docker executable in dir, or a name that does not exist there.
func lookIn(dir string) string { return filepath.Join(dir, "docker") }
