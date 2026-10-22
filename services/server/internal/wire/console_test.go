// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// repoRoot is the repository root relative to this package directory.
const repoRoot = "../../../.."

// consoleBuildDir writes index.html (with the nonce placeholder the server requires since M6-03)
// and the version.json of the console build step (apps/console/tool/version_json.sh, the same
// script services/server/Dockerfile runs) into a temporary directory.
func consoleBuildDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	index := `<!DOCTYPE html><script src="/console/flutter_bootstrap.js" async nonce="` + httpserver.NoncePlaceholder + `"></script>`
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(index), 0o600); err != nil {
		t.Fatal(err)
	}
	// The arguments are fixed paths of this repository and a temporary directory of the test.
	script := filepath.Join(repoRoot, "apps/console/tool/version_json.sh")
	args := []string{
		script, filepath.Join(repoRoot, "docs/api/openapi.yaml"),
		filepath.Join(repoRoot, "services/server/VERSION"), filepath.Join(dir, "version.json"),
	}
	cmd := exec.CommandContext(t.Context(), "sh", args...) //nolint:gosec // G204: no input from outside the test
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("version_json.sh: %v: %s", err, out)
	}
	return dir
}

// TestConsoleVersionJSON: the version.json the server serves under /console/ carries the
// spec_version that getDiscovery reports (routes.SpecVersion, wired into the instance contract in
// settings.go) and the server version of the binary (brief M6-02 Design note 5).
func TestConsoleVersionJSON(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	if err := MountConsole(mux, loadValues(t, "BOTHY_CONSOLE__ASSETS_DIR="+consoleBuildDir(t))); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/console/version.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /console/version.json = %d", rec.Code)
	}
	var v struct {
		SpecVersion   string `json:"spec_version"`
		ServerVersion string `json:"server_version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v.SpecVersion != routes.SpecVersion {
		t.Errorf("version.json spec_version = %q, discovery reports %q", v.SpecVersion, routes.SpecVersion)
	}
	if v.ServerVersion != buildinfo.Version() {
		t.Errorf("version.json server_version = %q, the binary is %q", v.ServerVersion, buildinfo.Version())
	}
}

// TestMountConsole: an empty console.assets_dir mounts nothing, a directory without index.html
// ends the start with an error naming the key.
func TestMountConsole(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	if err := MountConsole(mux, loadValues(t)); err != nil {
		t.Fatal(err)
	}
	if _, pattern := mux.Handler(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/console/", nil)); pattern != "" {
		t.Errorf("empty console.assets_dir mounted %q", pattern)
	}
	err := MountConsole(http.NewServeMux(), loadValues(t, "BOTHY_CONSOLE__ASSETS_DIR="+t.TempDir()))
	if err == nil || !strings.Contains(err.Error(), "console.assets_dir") {
		t.Errorf("directory without index.html: err = %v, want one naming console.assets_dir", err)
	}
}
