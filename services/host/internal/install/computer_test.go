// SPDX-License-Identifier: Apache-2.0

package install

import (
	"strings"
	"testing"
)

// TestComputerEndpointFolder: with host_service.computer_use install creates the folder of the
// helper endpoint (docs/spec/27-computer-use.md 9.4): macOS a folder of the service user with mode
// 0755, Linux the runtime folder of the unit; without it the plans stay as before.
func TestComputerEndpointFolder(t *testing.T) {
	tg := target("darwin")
	tg.ComputerUse = true
	steps, err := Install(tg, people)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range steps {
		if s.Kind == KindMkdir && s.Path == DarwinComputerDir {
			found = s.Owner == "bothy-host" && s.Mode == 0o755
		}
	}
	if !found {
		t.Error("no folder of the computer endpoint in the macOS plan")
	}
	tg = target("linux")
	tg.ComputerUse = true
	steps, err = Install(tg, people)
	if err != nil {
		t.Fatal(err)
	}
	unit := ""
	for _, s := range steps {
		if strings.HasSuffix(s.Path, "/bothy-host.service") {
			unit = s.Content
		}
	}
	if !strings.Contains(unit, "RuntimeDirectory=bothy-host-confirm bothy-host-computer\n") {
		t.Errorf("unit without the computer runtime folder:\n%s", unit)
	}
	for _, goos := range []string{"darwin", "linux"} {
		steps, _ := Install(target(goos), people)
		if s := lines(steps) + unitsOf(steps); strings.Contains(s, "computer") {
			t.Errorf("%s: a computer folder without host_service.computer_use", goos)
		}
	}
}

func unitsOf(steps []Step) string {
	var b strings.Builder
	for _, s := range steps {
		b.WriteString(s.Content)
	}
	return b.String()
}
