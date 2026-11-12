// SPDX-License-Identifier: Apache-2.0

package scan_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"

	serverconfig "github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The fixture release of the wizard runs: install.sh is rendered with this version and download
// address, and the bundle pins this server version and registry, as a product release does
// (docs/spec/18-release.md 2.1).
const (
	wizardVersion       = "1.2.3"
	wizardBaseURL       = "https://github.com/example/bothy/releases/download/v" + wizardVersion + "/"
	wizardServerVersion = "9.8.7"
	wizardRegistry      = "ghcr.io/example"
)

// scanModeKey is the key the wizard's choice of the virus scan must reach (12 5.3).
const scanModeKey = "file.scan.mode"

// The services of compose.yaml that take part in an upload: api accepts it, worker scans it.
var uploadServices = []string{"api", "worker"}

// TestInstallWithoutVirusScan follows the wizard's choice of the virus scan from install.sh to an
// upload (12 5.3, 16 3.1). Nothing on the way is a copy: the real infra/compose/install.sh,
// rendered as a release ships it, writes the .env of an installation from the real .env.example;
// the Docker CLI renders the real compose.yaml with that .env; the configuration loader of the
// server reads the environment the services get. With the scan off the stack has no clamd, so
// file.scan.mode must be off, and a server with the loaded values ends an upload as skipped and
// complete; with required it would stay pending for good (fail closed). With the scan on the mode
// stays required and the stack runs clamav.
func TestInstallWithoutVirusScan(t *testing.T) {
	t.Run("scan on", func(t *testing.T) {
		stack := installedStack(t, "on")
		if _, ok := stack["clamav"]; !ok {
			t.Errorf("install.sh --scan on: the stack has no service clamav (services %v)", serviceNames(stack))
		}
		for _, name := range uploadServices {
			if got := serviceConfig(t, stack, name).String(scanModeKey); got != "required" {
				t.Errorf("install.sh --scan on: %s runs with %s = %q, want required", name, scanModeKey, got)
			}
		}
	})
	t.Run("scan off", func(t *testing.T) {
		stack := installedStack(t, "off")
		if _, ok := stack["clamav"]; ok {
			t.Fatal("install.sh --scan off: the stack still runs the service clamav")
		}
		for _, name := range uploadServices {
			if got := serviceConfig(t, stack, name).String(scanModeKey); got != "off" {
				t.Fatalf("install.sh --scan off: %s runs with %s = %q and the stack has no clamav, so every upload stays pending (12 5.3); want off",
					name, scanModeKey, got)
			}
		}
		worker := serviceConfig(t, stack, "worker")
		s := harness.Start(t, harness.Options{Config: config(worker.String("file.scan.clamd_address"),
			map[string]string{scanModeKey: worker.String(scanModeKey)})})
		w := harness.BuildMatrixWorld(t, s)
		alex := w.People["Alex"]
		db := s.AdminDB()
		u := send(t, alex, w.Channels["Support/Allgemein"], "wizard.png", "", pngBytes, pngBytes)
		u.complete(t, alex, "")
		if r := waitFinal(t, db, u.FileID); r.ScanStatus != "skipped" {
			t.Fatalf("the upload of an installation without the virus scan: %+v, want scan_status skipped", r)
		}
		if r := waitComplete(t, db, u.FileID); r.UploadState != "complete" {
			t.Fatalf("after files.thumbnail: %+v, want upload_state complete", r)
		}
	})
}

// composeService is one service of the model `docker compose config --format json` prints.
type composeService struct {
	Environment map[string]*string `json:"environment"`
}

// installedStack installs with install.sh and the given answer for the virus scan into a
// temporary folder and returns the services Compose would start there.
func installedStack(t *testing.T, scan string) map[string]composeService {
	t.Helper()
	return renderStack(t, runWizard(t, scan))
}

// repoPath returns the absolute path of a file of the repository, given relative to its root.
func repoPath(t *testing.T, rel string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// readRepoFile reads a file of the repository, given relative to its root.
func readRepoFile(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(repoPath(t, rel)) // #nosec G304 -- a fixed file of the repository
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// replaceLine replaces the one line of text that pattern matches (a whole line) by line.
func replaceLine(t *testing.T, text []byte, pattern, line string) []byte {
	t.Helper()
	re := regexp.MustCompile("(?m)^" + pattern + "$")
	if n := len(re.FindAll(text, -1)); n != 1 {
		t.Fatalf("%d lines match %s, want exactly one", n, pattern)
	}
	return re.ReplaceAllLiteral(text, []byte(line))
}

// wizardBundle builds the installation bundle of the fixture release: the real compose.yaml and
// the real .env.example of the repository, with the server version and the registry a release
// pins there, below the top directory bothy-<version>.
func wizardBundle(t *testing.T) []byte {
	t.Helper()
	example := readRepoFile(t, "infra/compose/.env.example")
	example = replaceLine(t, example, `BOTHY_VERSION=.*`, "BOTHY_VERSION="+wizardServerVersion)
	example = replaceLine(t, example, `BOTHY_REGISTRY=.*`, "BOTHY_REGISTRY="+wizardRegistry)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range map[string][]byte{
		"compose.yaml": readRepoFile(t, "infra/compose/compose.yaml"),
		".env.example": example,
	} {
		hdr := &tar.Header{Name: "bothy-" + wizardVersion + "/infra/compose/" + name, Mode: 0o644, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// runWizard runs infra/compose/install.sh without a terminal, as cloud-init and CI do, and
// returns the Compose folder of the installation. The script is rendered the way make
// release-build renders it (its three built-in lines) and gets its bundle with --bundle. The
// only stand-ins are the stubs of the install tests for docker, curl and cosign
// (infra/compose/test/install/stubs): the wizard writes .env before it starts anything, so no
// container is needed to see what it wrote.
func runWizard(t *testing.T, scan string) string {
	t.Helper()
	dir := t.TempDir()
	bundle := wizardBundle(t)
	bundlePath := filepath.Join(dir, "bothy-compose-"+wizardVersion+".tar.gz")
	script := readRepoFile(t, "infra/compose/install.sh")
	script = replaceLine(t, script, `BOTHY_INSTALL_VERSION='[^']*'`, "BOTHY_INSTALL_VERSION='"+wizardVersion+"'")
	script = replaceLine(t, script, `BOTHY_INSTALL_BUNDLE_SHA256='[^']*'`, "BOTHY_INSTALL_BUNDLE_SHA256='"+sha(bundle)+"'")
	script = replaceLine(t, script, `BOTHY_INSTALL_BASE_URL='[^']*'`, "BOTHY_INSTALL_BASE_URL='"+wizardBaseURL+"'")
	scriptPath := filepath.Join(dir, "install.sh")
	for path, content := range map[string][]byte{bundlePath: bundle, scriptPath: script} {
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(dir, "bothy")
	//nolint:gosec // G204: the repository's own script with fixed arguments; running it is the test.
	cmd := exec.CommandContext(testkit.Context(t), "sh", scriptPath, "--bundle", bundlePath, "--dir", target, "--yes",
		"--public-url", "https://chat.example.com", "--scan", scan, "--owner-handle", "alex", "--owner-name", "Alex")
	cmd.Dir = dir
	// Only the stubs and the system tools: the wizard never sees the real Docker CLI.
	cmd.Env = []string{
		"PATH=" + repoPath(t, "infra/compose/test/install/stubs") + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin",
		"HOME=" + dir,
		"STUB_LOG=" + filepath.Join(dir, "stub.log"),
	}
	// A session of its own has no controlling terminal: the wizard asks nothing and shows the
	// enrollment code of the stub on its output, not on the terminal of whoever runs the tests.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("install.sh --scan %s: %v\n%s", scan, err, out)
	}
	return filepath.Join(target, "infra", "compose")
}

// renderStack returns the services of the installation in composeDir as the real Docker CLI
// renders them: `docker compose config` there reads compose.yaml and the .env of the wizard and
// starts nothing. Compose prefers the environment of its process over .env, so the variables an
// installation sets in .env do not reach it from the environment of the test.
func renderStack(t *testing.T, composeDir string) map[string]composeService {
	t.Helper()
	cmd := exec.CommandContext(testkit.Context(t), "docker", "compose", "config", "--format", "json")
	cmd.Dir = composeDir
	for _, kv := range os.Environ() { //nolint:forbidigo // the environment of the Docker CLI, not server configuration
		if !strings.HasPrefix(kv, "BOTHY_") && !strings.HasPrefix(kv, "COMPOSE_") && !strings.HasPrefix(kv, "TS_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("docker compose config in %s: %v: %s", composeDir, err, errOut.String())
	}
	var model struct {
		Services map[string]composeService `json:"services"`
	}
	if err := json.Unmarshal(out.Bytes(), &model); err != nil {
		t.Fatalf("decode docker compose config: %v", err)
	}
	return model.Services
}

// serviceNames lists the services of a rendered stack, sorted.
func serviceNames(stack map[string]composeService) []string {
	names := make([]string, 0, len(stack))
	for name := range stack {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// serviceConfig loads the configuration of a server service of the rendered stack the way its
// process does at start: the schema of the server over exactly the environment Compose gives the
// container, so an unknown key or a failed cross-check fails here as it would stop the start.
func serviceConfig(t *testing.T, stack map[string]composeService, name string) *serverconfig.Values {
	t.Helper()
	svc, ok := stack[name]
	if !ok {
		t.Fatalf("the stack has no service %s (services %v)", name, serviceNames(stack))
	}
	env := make([]string, 0, len(svc.Environment))
	for k, v := range svc.Environment {
		if v != nil {
			env = append(env, k+"="+*v)
		}
	}
	schema, err := wire.ConfigSchema()
	if err != nil {
		t.Fatal(err)
	}
	values, err := serverconfig.Load(schema, serverconfig.Sources{Env: env})
	if err != nil {
		t.Fatalf("the environment of %s does not load: %v", name, err)
	}
	return values
}
