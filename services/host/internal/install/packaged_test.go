// SPDX-License-Identifier: Apache-2.0

package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/config"
)

// infraHost is infra/host of the repository, where the package recipes and the service
// definitions the packages ship live (brief M7-23).
var infraHost = filepath.Join("..", "..", "..", "..", "infra", "host")

// TestPackagedDefinitionsEqualInstall: the service definition of the host service a .deb, .rpm or
// .pkg package ships is byte for byte the one bothy-host install writes for the default service
// user and configuration path, so install finds it unchanged and the two never drift apart
// (docs/spec/16-operations.md 4.4 item 2). go test -update rewrites the files under infra/host.
func TestPackagedDefinitionsEqualInstall(t *testing.T) {
	for goos, file := range map[string]string{"linux": "linux/bothy-host.service", "darwin": "macos/bothy-host.plist"} {
		path, content, err := HostDefinition(goos, "bothy-host", config.DefaultPath(config.Platform{GOOS: goos}))
		if err != nil {
			t.Fatal(err)
		}
		steps, err := Install(target(goos), people)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, s := range steps {
			if s.Kind == KindWrite && s.Path == path {
				found = s.Content == content
			}
		}
		if !found {
			t.Errorf("%s: install writes %s with another content than HostDefinition", goos, path)
		}
		p := filepath.Join(infraHost, file)
		if *update {
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil { //nolint:gosec // a definition the packages ship world-readable
				t.Fatal(err)
			}
			continue
		}
		b, err := os.ReadFile(p) //nolint:gosec // a file of the repository
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != content {
			t.Errorf("%s differs from what bothy-host install writes (go test ./internal/install -update)", file)
		}
	}
	if _, _, err := HostDefinition("windows", "bothy-host", `C:\Program Files\bothy-host\config.yaml`); err == nil {
		t.Error("windows: the MSI registers the service itself, HostDefinition has no file for it")
	}
}

// TestMSIServiceEqualsInstall: the service the MSI registers (infra/host/windows/bothy-host.wxs)
// has the name, display name, account and command line of the service bothy-host install creates,
// and the MSI leaves it stopped with the start type demand until install sets it to auto.
func TestMSIServiceEqualsInstall(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(infraHost, "windows", "bothy-host.wxs")) //nolint:gosec // a file of the repository
	if err != nil {
		t.Fatal(err)
	}
	wxs := string(b)
	tg := target("windows")
	var def map[string]string
	steps, err := Install(tg, people)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range steps {
		if s.Kind == KindWinService && s.Name == hostLabel {
			def = parseDefinitionText(s.Content)
		}
	}
	// The MSI names the program folder through its property; install through %ProgramFiles%.
	args := strings.TrimPrefix(def["binary"], `"`+tg.Binary()+`" `)
	args = strings.ReplaceAll(args, tg.ProgramFiles+`\`, "[ProgramFiles64Folder]")
	args = strings.ReplaceAll(args, `"`, "&quot;")
	for _, want := range []string{
		`Name="` + def["name"] + `"`, `DisplayName="` + def["display"] + `"`, `Account="` + def["account"] + `"`,
		`Arguments="` + args + `"`, `Start="demand"`, `Type="ownProcess"`, `Remove="uninstall"`,
	} {
		if !strings.Contains(wxs, want) {
			t.Errorf("bothy-host.wxs lacks %s", want)
		}
	}
	if strings.Contains(wxs, `Start="install"`) || strings.Contains(wxs, `Start="both"`) {
		t.Error("the MSI starts the service")
	}
}

// parseDefinitionText reads the key=value lines of a rendered Windows definition.
func parseDefinitionText(content string) map[string]string {
	out := map[string]string{}
	for _, l := range strings.Split(content, "\n") {
		if k, v, ok := strings.Cut(l, "="); ok && !strings.HasPrefix(l, "#") {
			out[k] = v
		}
	}
	return out
}

// TestDarwinInstallEnablesHostBeforeLoad: the .pkg disables the launchd job of the host service
// until install has run (infra/host/macos/scripts/postinstall), so install enables it before it
// bootstraps the LaunchDaemon; enabling a job that is not disabled changes nothing.
func TestDarwinInstallEnablesHostBeforeLoad(t *testing.T) {
	steps, err := Install(target("darwin"), people)
	if err != nil {
		t.Fatal(err)
	}
	enable, bootstrap := -1, -1
	for i, s := range steps {
		switch strings.Join(s.Argv, " ") {
		case "/bin/launchctl enable system/bothy-host":
			enable = i
		case "/bin/launchctl bootstrap system /Library/LaunchDaemons/bothy-host.plist":
			bootstrap = i
		}
	}
	if enable < 0 || bootstrap < 0 || enable > bootstrap || steps[enable].Kind != KindRun {
		t.Errorf("enable at %d, bootstrap at %d", enable, bootstrap)
	}
}
