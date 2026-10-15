// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance"
)

// fixtureFlags is a registry with one release flag (remove_by 1.4.0) and one option.
var fixtureFlags = []instance.FlagDefinition{
	{Key: "chat.live_box", Kind: instance.FlagKindRelease, Description: "Live box", Module: "chat", RemoveBy: "1.4.0"},
	{Key: "dm.enabled", Kind: instance.FlagKindOption, Description: "Direct messages", Module: "chat"},
}

// versionFile writes v as a VERSION file and returns its path.
func versionFile(t *testing.T, v string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "VERSION")
	if err := os.WriteFile(p, []byte(v+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// runFixture runs the check over defs with the given VERSION and config doc of testdata.
func runFixture(t *testing.T, defs []instance.FlagDefinition, version, doc string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := runWith([]string{"-version-file", versionFile(t, version), "-config-doc", filepath.Join("testdata", doc), "-modules", "testdata/modules.yaml"},
		defs, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestFlagCheck_ReleaseReachedRemoveByFails(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"1.4.0", "1.4.1", "2.0.0", "1.4.0+build.5"} {
		code, _, errOut := runFixture(t, fixtureFlags, v, "configuration.md")
		want := "flag-check: remove flag chat.live_box: VERSION " + v + " reached remove_by 1.4.0"
		if code != 1 || !strings.Contains(errOut, want) {
			t.Errorf("%s: code %d, stderr %q", v, code, errOut)
		}
	}
}

func TestFlagCheck_PreReleaseBeforeRemoveByPasses(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"1.4.0-beta.1", "1.3.9", "0.0.0"} {
		code, out, errOut := runFixture(t, fixtureFlags, v, "configuration.md")
		if code != 0 || out != "flag-check: ok (2 flags)\n" || errOut != "" {
			t.Errorf("%s: code %d, stdout %q, stderr %q", v, code, out, errOut)
		}
	}
}

func TestFlagCheck_ConfigDocMismatchFails(t *testing.T) {
	t.Parallel()
	code, _, errOut := runFixture(t, fixtureFlags, "1.0.0", "configuration-drift.md")
	if code != 1 {
		t.Fatalf("code %d", code)
	}
	for _, want := range []string{
		"flag-check: chat.live_box: kind release in the registry, option in configuration.md 7",
		"flag-check: dm.enabled: default false in the registry, on in configuration.md 7",
		"flag-check: voice.enabled: in configuration.md 7 but not registered",
	} {
		if !strings.Contains(errOut, want+"\n") {
			t.Errorf("missing %q in %q", want, errOut)
		}
	}
	code, _, errOut = runFixture(t, fixtureFlags[1:], "1.0.0", "configuration.md")
	if code != 1 || !strings.Contains(errOut, "flag-check: chat.live_box: in configuration.md 7 but not registered") {
		t.Errorf("missing registration: %d %q", code, errOut)
	}
	code, _, errOut = runFixture(t, append(fixtureFlags[:2:2], instance.FlagDefinition{Key: "files.x", Kind: instance.FlagKindOption, Description: "x", Module: "chat"}),
		"1.0.0", "configuration.md")
	if code != 1 || !strings.Contains(errOut, "flag-check: files.x: registered but not in configuration.md 7") {
		t.Errorf("missing row: %d %q", code, errOut)
	}
}

func TestFlagCheck_RegistryFindingsFail(t *testing.T) {
	t.Parallel()
	bad := []instance.FlagDefinition{{Key: "voice", Kind: instance.FlagKindOption, Description: "x", Module: "files"}}
	code, _, errOut := runFixture(t, append(bad, fixtureFlags...), "1.0.0", "configuration.md")
	if code != 1 || !strings.Contains(errOut, `flag-check: flag "voice": key`) || !strings.Contains(errOut, `module "files"`) {
		t.Errorf("code %d, stderr %q", code, errOut)
	}
}

func TestFlagCheck_UsageAndIOErrors(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	if code := runWith(nil, fixtureFlags, &out, &errOut); code != 2 {
		t.Errorf("no flags: code %d", code)
	}
	for _, args := range [][]string{
		{"-version-file", "testdata/none", "-config-doc", "testdata/configuration.md", "-modules", "testdata/modules.yaml"},
		{"-version-file", versionFile(t, "v1.0.0"), "-config-doc", "testdata/configuration.md", "-modules", "testdata/modules.yaml"},
		{"-version-file", versionFile(t, "1.0.0"), "-config-doc", "testdata/none", "-modules", "testdata/modules.yaml"},
		{"-version-file", versionFile(t, "1.0.0"), "-config-doc", "testdata/configuration.md", "-modules", "testdata/none"},
	} {
		errOut.Reset()
		if code := runWith(args, fixtureFlags, &out, &errOut); code != 1 || !strings.HasPrefix(errOut.String(), "flag-check: ") {
			t.Errorf("%v: code %d, %q", args, code, errOut.String())
		}
	}
}

func TestFlagCheck_RealRepositoryPasses(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	code := run([]string{"-version-file", "../../VERSION", "-config-doc", "../../../../docs/operations/configuration.md",
		"-modules", "../../architecture/modules.yaml"}, &out, &errOut)
	if code != 0 || errOut.Len() != 0 || !strings.HasPrefix(out.String(), "flag-check: ok (") {
		t.Fatalf("code %d, stdout %q, stderr %q", code, out.String(), errOut.String())
	}
}
