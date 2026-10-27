// SPDX-License-Identifier: Apache-2.0

package e2epush_test

import (
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/internal/e2epush"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestApplySetsTheEndpointsOfBothProviders (brief M5-11 Design note 1): the base URL of
// Fake-APNs serves both APNs environments, the one of Fake-FCM the send endpoint and, under
// /token, the token endpoint; the last assignment of a variable wins, as in a process
// environment.
func TestApplySetsTheEndpointsOfBothProviders(t *testing.T) {
	t.Parallel()
	var d wire.Deps
	env := []string{"PATH=/usr/bin", e2epush.EnvAPNs + "=https://other:1", e2epush.EnvAPNs + "=https://fakepush:8443",
		e2epush.EnvFCM + "=https://fakepush:8444"}
	if err := e2epush.Apply(env, &d); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"production": "https://fakepush:8443", "sandbox": "https://fakepush:8443"}
	if !maps.Equal(d.APNsEndpoints, want) {
		t.Fatalf("APNsEndpoints %v, want %v", d.APNsEndpoints, want)
	}
	wantFCM := wire.FCMEndpoints{Send: "https://fakepush:8444", Token: "https://fakepush:8444/token"} // #nosec G101 -- URLs of the fake, no credential
	if d.FCMEndpoints != wantFCM {
		t.Fatalf("FCMEndpoints %+v", d.FCMEndpoints)
	}
	if d.APNsRootCAs != nil {
		t.Fatal("Apply set a root pool; the fakes' CA comes from SSL_CERT_FILE")
	}
}

// TestApplyNeedsBothVariables: the E2E server build fails closed. Without a variable it never
// leaves a provider on the real endpoints of Apple or Google, which a misconfigured run would
// otherwise reach with the fake's keys and the device tokens of the test; the error names the
// missing variable, and Deps stays unchanged.
func TestApplyNeedsBothVariables(t *testing.T) {
	t.Parallel()
	for missing, env := range map[string][]string{
		e2epush.EnvAPNs: {"HOME=/tmp", e2epush.EnvFCM + "=https://fakepush:8444"},
		e2epush.EnvFCM:  {e2epush.EnvAPNs + "=https://fakepush:8443"},
	} {
		var d wire.Deps
		err := e2epush.Apply(env, &d)
		if err == nil || !strings.Contains(err.Error(), missing) {
			t.Errorf("without %s: %v, want an error naming it", missing, err)
		}
		if d.APNsEndpoints != nil || d.FCMEndpoints != (wire.FCMEndpoints{}) {
			t.Errorf("without %s: Deps changed to %v, %+v", missing, d.APNsEndpoints, d.FCMEndpoints)
		}
	}
}

// TestApplyRefusesAMalformedURL: a set variable must be an https URL with a host and nothing
// after the port; the error names the variable, and Deps stays unchanged.
func TestApplyRefusesAMalformedURL(t *testing.T) {
	t.Parallel()
	for _, name := range []string{e2epush.EnvAPNs, e2epush.EnvFCM} {
		for _, value := range []string{"", "http://fakepush:8443", "https://", "https://user@fakepush:8443",
			"https://fakepush:8443/v1", "https://fakepush:8443/", "https://fakepush:8443?x=1", "https://fakepush:8443#x",
			"fakepush:8443", "https://fake push:8443"} {
			env := []string{e2epush.EnvAPNs + "=https://fakepush:8443", e2epush.EnvFCM + "=https://fakepush:8444", name + "=" + value}
			var d wire.Deps
			err := e2epush.Apply(env, &d)
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Errorf("%s=%q: %v, want an error naming the variable", name, value, err)
			}
			if d.APNsEndpoints != nil || d.FCMEndpoints != (wire.FCMEndpoints{}) {
				t.Errorf("%s=%q changed Deps", name, value)
			}
		}
	}
}

// goList runs go list with args in cmd/bothy and returns its output lines.
func goList(t *testing.T, args ...string) []string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "go", append([]string{"list"}, args...)...) // #nosec G204 -- fixed arguments of the test
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list %v: %v\n%s", args, err, out)
	}
	return strings.Fields(string(out))
}

// TestReleaseBuildHasNoE2EPush (brief M5-11 Design note 1): the release build of cmd/bothy (no
// build tags) compiles the no-op file and neither the file with the tag e2e nor this package;
// the build with the tag e2e compiles the E2E file instead and links this package; and no other
// Go file of the module names the variables of this package. So no configuration key, flag or
// environment variable of the release server redirects push.
func TestReleaseBuildHasNoE2EPush(t *testing.T) {
	t.Parallel()
	const files = "{{join .GoFiles \" \"}}"
	release, e2e := goList(t, "-tags=", "-f", files, "."), goList(t, "-tags=e2e", "-f", files, ".")
	if slices.Contains(release, "e2e_push.go") || !slices.Contains(release, "e2e_push_off.go") {
		t.Fatalf("release build of cmd/bothy compiles %v", release)
	}
	if !slices.Contains(e2e, "e2e_push.go") || slices.Contains(e2e, "e2e_push_off.go") {
		t.Fatalf("E2E build of cmd/bothy compiles %v", e2e)
	}
	const pkg = "github.com/OWNER/bothy/services/server/cmd/bothy/internal/e2epush"
	releaseDeps, e2eDeps := goList(t, "-tags=", "-deps", "."), goList(t, "-tags=e2e", "-deps", ".")
	if !slices.Contains(releaseDeps, "github.com/OWNER/bothy/services/server/internal/wire") {
		t.Fatalf("go list -deps does not look like the dependencies of cmd/bothy: %.400v", releaseDeps)
	}
	if slices.Contains(releaseDeps, pkg) || !slices.Contains(e2eDeps, pkg) {
		t.Fatalf("e2epush in the release build: %v, in the E2E build: %v", slices.Contains(releaseDeps, pkg), slices.Contains(e2eDeps, pkg))
	}
	// The module root, walked without following directory links; this package is the one
	// allowed to name the variables.
	module := os.DirFS(filepath.Join("..", "..", "..", ".."))
	const self = "cmd/bothy/internal/e2epush"
	err := fs.WalkDir(module, ".", func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() && (path == self || e.Name() == "testdata") {
			return fs.SkipDir
		}
		if e.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		src, err := fs.ReadFile(module, path)
		if err != nil {
			return err
		}
		for _, name := range []string{e2epush.EnvAPNs, e2epush.EnvFCM} {
			if strings.Contains(string(src), name) {
				t.Errorf("%s names %s; only package e2epush may read it", path, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestE2EBuildTypeChecks: make lint and make test never compile cmd/bothy with the tag e2e, so a
// change to Apply or wire.Deps that breaks e2e_push.go would show only in the image build of a
// device run; go vet with the tag type-checks the E2E build here instead.
func TestE2EBuildTypeChecks(t *testing.T) {
	t.Parallel()
	cmd := exec.CommandContext(t.Context(), "go", "vet", "-tags=e2e", ".") // #nosec G204 -- fixed arguments of the test
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go vet -tags=e2e ./cmd/bothy: %v\n%s", err, out)
	}
}
