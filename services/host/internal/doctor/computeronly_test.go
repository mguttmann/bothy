// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"slices"
	"testing"
)

// TestComputerOnlyHost holds D-CU-03-13: with host_service.computer_use and no session users the
// empty list (HD1) and the unproven peer check of the terminal helpers (HD10) are warnings, so they
// refuse no start on any platform; without computer use, or with session users, both refuse as
// before.
func TestComputerOnlyHost(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		x := newTW(t)
		x.env.GOOS = goos
		x.cfg.SessionUsers = nil
		x.cfg.ComputerUse = true
		x.env.PeerCheckProven = false
		r := x.run()
		// The fake world is a Linux one, so other platforms only show the two codes as warnings.
		if failedWith(r, HD1, CodeSessionUsersEmpty) || failedWith(r, HD10, CodePeerCheckUnavailable) {
			t.Errorf("%s: HD1 %v HD10 %v refuse a computer-only host", goos, codes(r, HD1), codes(r, HD10))
		}
		if goos == "linux" && r.StartRefused() {
			t.Errorf("linux computer-only host refused: %+v", r.Report())
		}
		if goos == "linux" && !slices.Contains(r.Report(), ReportEntry{Check: HD1, OK: true, Code: CodeSessionUsersEmpty}) {
			t.Errorf("the report does not name the missing terminal: %+v", r.Report())
		}
		if !warnedWith(r, HD1, CodeSessionUsersEmpty) || !warnedWith(r, HD10, CodePeerCheckUnavailable) {
			t.Errorf("%s: the missing terminal is not reported as warnings: %v %v", goos, codes(r, HD1), codes(r, HD10))
		}
	}

	x := newTW(t)
	x.cfg.SessionUsers = nil
	x.env.PeerCheckProven = false
	if r := x.run(); !r.StartRefused() || !has(codes(r, HD1), CodeSessionUsersEmpty) || !has(codes(r, HD10), CodePeerCheckUnavailable) {
		t.Errorf("without computer use: %v %v", codes(r, HD1), codes(r, HD10))
	}

	x = newTW(t)
	x.cfg.ComputerUse = true
	x.env.PeerCheckProven = false
	if r := x.run(); !r.StartRefused() || !has(codes(r, HD10), CodePeerCheckUnavailable) {
		t.Errorf("with session users: %v", codes(r, HD10))
	}
}

func failedWith(r *Result, check, code string) bool {
	for _, e := range r.Check(check).Entries {
		if e.Code == code && e.Status == Fail {
			return true
		}
	}
	return false
}

func warnedWith(r *Result, check, code string) bool {
	for _, e := range r.Check(check).Entries {
		if e.Code == code && e.Status == Warn {
			return true
		}
	}
	return false
}
