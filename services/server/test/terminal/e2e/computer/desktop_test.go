// SPDX-License-Identifier: Apache-2.0

//go:build computere2e

package computer

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/test/terminal/stack"
)

// The desktop machine: the account the test backend runs in (host_service.computer_test_peer),
// the X display, the copy of services/host/test/computer_backend and the event file of the
// fixture app.
const (
	deskUser    = "alex"
	display     = ":1"
	backendDir  = "/opt/cu/computer_backend"
	eventsFile  = "/home/" + deskUser + "/fixture-events.jsonl"
	helperUnit  = "cu-helper"
	helperUnit0 = "cu-helper-off"
)

// deskSetup runs as root after bothy-host install (suite.HostSpec.Setup): it creates the account
// of the desktop and installs again with host_service.computer_use, so the unit gets the folder of
// the helper endpoint (D-CU-03-9). The suite writes the whole configuration afterwards.
var deskSetup = strings.Join([]string{
	"useradd --create-home --shell /bin/bash " + deskUser,
	"printf 'host_service:\\n  session_users: [" + sessionUser + "]\\n  computer_use: true\\n  computer_test_peer: " + deskUser + "\\n' > " + stack.HostConfig,
	stack.HostBinDir + "/bothy-host install",
}, " && ")

// desktop is the host container of the desktop.
type desktop struct {
	h *stack.Host
}

// newDesktop copies the test backend and the fixture app into the desktop and starts Xvfb and the
// fixture app as the desktop's account (transient systemd units), then waits until the app is
// ready.
func newDesktop(t *testing.T, h *stack.Host) *desktop {
	t.Helper()
	ctx := t.Context()
	d := &desktop{h: h}
	root, err := stack.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	shell(t, h, "install -d -o root -g root -m 0755 /opt/cu && install -d -m 1777 /tmp/.X11-unix")
	if err := h.CopyIn(ctx, filepath.Join(root, "services", "host", "test", "computer_backend"), backendDir); err != nil {
		t.Fatal(err)
	}
	shell(t, h, "chown -R root:root "+backendDir+" && chmod -R a+rX,go-w "+backendDir)
	shell(t, h, "systemd-run --quiet --unit=cu-xvfb --uid="+deskUser+" --gid="+deskUser+
		" /usr/bin/Xvfb "+display+" -screen 0 "+strconv.Itoa(screenWidth)+"x"+strconv.Itoa(screenHeight)+"x24 -nolisten tcp")
	until(t, "Xvfb on the desktop", func() bool {
		r, err := h.Exec(ctx, nil, "test", "-S", "/tmp/.X11-unix/X"+strings.TrimPrefix(display, ":"))
		return err == nil && r.Code == 0
	})
	shell(t, h, "systemd-run --quiet --unit=cu-fixture --uid="+deskUser+" --gid="+deskUser+" --setenv=DISPLAY="+display+
		" --setenv=HOME=/home/"+deskUser+" /usr/bin/python3 "+backendDir+"/fixture_app.py "+eventsFile)
	d.waitEvent(t, `{"event": "ready"}`)
	return d
}

// startHelper starts the test backend as the desktop's account, switched on or off with
// generation (the helper's local switch, 27 4 item 1).
func (d *desktop) startHelper(t *testing.T, enabled bool, generation int) {
	t.Helper()
	unit, flags := helperUnit, ""
	if !enabled {
		unit, flags = helperUnit0, " --disabled"
	}
	shell(t, d.h, "systemd-run --quiet --unit="+unit+" --uid="+deskUser+" --gid="+deskUser+" --setenv=DISPLAY="+display+
		" --setenv=HOME=/home/"+deskUser+" /usr/bin/python3 "+backendDir+"/backend.py --generation "+strconv.Itoa(generation)+flags)
}

// stopHelper stops the switched-off backend.
func (d *desktop) stopHelper(t *testing.T) {
	t.Helper()
	shell(t, d.h, "systemctl stop "+helperUnit0)
}

// signalHelper sends sig to the running backend: SIGUSR1 simulates a lock, SIGUSR2 the unlock.
func (d *desktop) signalHelper(t *testing.T, sig string) {
	t.Helper()
	shell(t, d.h, "systemctl kill --kill-whom=main --signal="+sig+" "+helperUnit)
}

// waitEvent waits until the fixture app wrote the JSON line line.
func (d *desktop) waitEvent(t *testing.T, line string) {
	t.Helper()
	until(t, "the fixture event "+line, func() bool {
		r, err := d.h.Exec(t.Context(), nil, "cat", eventsFile)
		return err == nil && r.Code == 0 && strings.Contains(string(r.Stdout), line)
	})
}

// pressedProbe prints the number of keys the X server holds down and the button bits of the
// pointer (query_keymap and query_pointer of python-xlib).
const pressedProbe = `from Xlib import display
d = display.Display()
keys = sum(bin(b).count("1") for b in d.query_keymap())
buttons = d.screen().root.query_pointer().mask & 0x1f00
print(keys, buttons)
`

// checkNothingPressed fails unless the X server of the desktop holds no key and no button down
// (27 8.4 item 2).
func (d *desktop) checkNothingPressed(t *testing.T) {
	t.Helper()
	r, err := d.h.Exec(t.Context(), strings.NewReader(pressedProbe), "runuser", "-u", deskUser, "--", "env", "DISPLAY="+display, "python3", "-")
	if err != nil || r.Code != 0 {
		t.Fatalf("the probe of the pressed keys failed: %v %d", err, r.Code)
	}
	if got := strings.TrimSpace(string(r.Stdout)); got != "0 0" {
		t.Errorf("after the stop the X server holds keys and buttons %q, want 0 0", got)
	}
}

// shell runs script as root in h and fails the test unless it exits 0.
func shell(t *testing.T, h *stack.Host, script string) {
	t.Helper()
	if _, err := h.Shell(t.Context(), script); err != nil {
		t.Fatal(fmt.Errorf("%s: %w", h.Name, err))
	}
}
