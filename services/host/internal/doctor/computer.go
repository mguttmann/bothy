// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"strconv"
	"strings"
)

// ComputerSection is the state the section computer of bothy-host doctor prints
// (docs/spec/27-computer-use.md 9.3). It is no HD check: the server accepts exactly HD1 to HD11 in
// report (24 3.5), and the same state reaches it only in the report object computer.
type ComputerSection struct {
	// On is host_service.computer_use; TestPeer reports host_service.computer_test_peer set (Linux,
	// tests only).
	On, TestPeer bool
	// Installed reports the helper at its installed path; TrustedPath the rule of HD10 on it;
	// Signed a build with a code signing requirement for the helper.
	Installed, TrustedPath, Signed bool
	// Helper is connected, absent or refused; Enabled and Generation the helper's switch.
	Helper     string
	Enabled    bool
	Generation int64
	// Capture and Input are the permissions: granted, denied or unknown.
	Capture, Input string
}

// Format returns the lines of the section: "computer: off" alone without host_service.computer_use,
// else one line per fact. It names no path, user or app.
func (c ComputerSection) Format() string {
	if !c.On {
		return "computer: off\n"
	}
	var b strings.Builder
	line := func(k, v string) { b.WriteString("computer " + k + ": " + v + "\n") }
	b.WriteString("computer: on\n")
	line("helper installed", yesNo(c.Installed))
	line("helper path trusted", yesNo(c.TrustedPath))
	build := "development"
	if c.Signed {
		build = "signed"
	}
	line("build", build)
	line("helper", c.Helper)
	on := yesNo(c.Enabled)
	if c.Enabled {
		on += " (generation " + strconv.FormatInt(c.Generation, 10) + ")"
	} else {
		// The helper shows nothing outside a session (docs/decisions.d/FIX-CU-HELPER-NO-ICON.md),
		// so the line says where its switch is; switching on stays local (27 4 item 1).
		on += " (switch it on at this computer: open Computer Helper)"
	}
	line("switched on", on)
	line("permission capture", c.Capture)
	line("permission input", c.Input)
	if c.TestPeer {
		line("test peer", "set (test setting)")
	}
	return b.String()
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
