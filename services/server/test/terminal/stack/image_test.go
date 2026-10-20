// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// recordingDocker writes a docker that appends every call to calls.log in its folder, answers
// the server architecture and writes the image ID into the --iidfile of a build.
func recordingDocker(t *testing.T) (bin, log string) {
	t.Helper()
	dir := t.TempDir()
	log = filepath.Join(dir, "calls.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "` + log + `"
case "$1" in
version) echo arm64; exit 0 ;;
image) exit 1 ;;
tag) exit 0 ;;
buildx)
  while [ $# -gt 0 ]; do
    if [ "$1" = --iidfile ]; then printf 'sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' > "$2"; fi
    shift
  done
  exit 0 ;;
esac
exit 0
`
	bin = filepath.Join(dir, "docker")
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // G306: the fake must be executable
		t.Fatal(err)
	}
	return bin, log
}

// TestServerImageBuildArgs proves the build command of Design note 3: the Dockerfile of the server,
// the platform of the Docker host, VERSION from services/server/VERSION, REVISION from HEAD, the
// repository root as context and the tag termsuite-server: with the first 12 hex characters of
// the image ID.
func TestServerImageBuildArgs(t *testing.T) {
	root, err := RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	bin, log := recordingDocker(t)
	d := &Docker{Bin: bin, Env: []string{"PATH=/usr/bin:/bin"}}
	tag, err := ServerImage(t.Context(), d, root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if tag != "termsuite-server:0123456789ab" {
		t.Errorf("tag = %q, want termsuite-server:0123456789ab", tag)
	}
	version, err := os.ReadFile(filepath.Join(root, "services/server/VERSION")) //nolint:gosec // G304: a fixed file of the repository
	if err != nil {
		t.Fatal(err)
	}
	head, err := exec.CommandContext(t.Context(), "git", "-C", root, "rev-parse", "HEAD").Output() //nolint:gosec // G204: fixed program, the repository root
	if err != nil {
		t.Fatal(err)
	}
	calls, err := os.ReadFile(log) //nolint:gosec // G304: the log of the fake in a temp dir
	if err != nil {
		t.Fatal(err)
	}
	var build string
	for _, l := range strings.Split(string(calls), "\n") {
		if strings.HasPrefix(l, "buildx build") {
			build = l
		}
	}
	for _, want := range []string{
		"buildx build --load --platform linux/arm64 --file services/server/Dockerfile",
		"--build-arg VERSION=" + strings.TrimSpace(string(version)),
		"--build-arg REVISION=" + strings.TrimSpace(string(head)),
		"--iidfile ",
	} {
		if !strings.Contains(build, want) {
			t.Errorf("build call %q lacks %q", build, want)
		}
	}
	if !strings.HasSuffix(build, " .") {
		t.Errorf("build call %q does not build the repository root as context", build)
	}
	if !strings.Contains(string(calls), "tag sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef termsuite-server:0123456789ab") {
		t.Errorf("the image is not tagged with its content address:\n%s", calls)
	}
}

// TestHostImageTag proves the tag of the host image: termsuite-host: with the first 12 hex of the
// SHA-256 of infra/compose/test/host/Dockerfile, built when missing.
func TestHostImageTag(t *testing.T) {
	root, err := RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	bin, log := recordingDocker(t)
	d := &Docker{Bin: bin, Env: []string{"PATH=/usr/bin:/bin"}}
	tag, err := HostImage(t.Context(), d, root)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := fileSHA256(filepath.Join(root, "infra/compose/test/host/Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "termsuite-host:"+sum[:12] {
		t.Errorf("tag = %q, want termsuite-host:%s", tag, sum[:12])
	}
	calls, err := os.ReadFile(log) //nolint:gosec // G304: the log of the fake in a temp dir
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(calls), "buildx build --load --tag "+tag) {
		t.Errorf("the missing host image is not built:\n%s", calls)
	}
}
