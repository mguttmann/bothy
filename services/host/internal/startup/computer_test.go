// SPDX-License-Identifier: Apache-2.0

package startup

import (
	"context"
	"strings"
	"testing"
)

// TestDoctorComputerSection: the doctor answer of the running service ends with the section
// computer (docs/spec/27-computer-use.md 9.3), after the summary of HD1 to HD11.
func TestDoctorComputerSection(t *testing.T) {
	x := newWorld(t)
	x.pair()
	l, path := adminPair(t)
	d := x.deps()
	d.Admin = l
	d.ComputerDoctor = func() string { return "computer: off\n" }
	s, code := Start(context.Background(), d)
	if s == nil {
		t.Fatalf("refused %d", code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = s.Serve(ctx) }()
	s.Runner().WaitScans()
	a := askDoctor(t, path)
	if !strings.HasSuffix(a.Output, "\ncomputer: off\n") || !strings.Contains(a.Output, "doctor: ") {
		t.Errorf("output %q", a.Output)
	}
	for _, e := range a.Doctor {
		if strings.Contains(e.Check, "computer") {
			t.Errorf("a doctor entry %+v for computer", e)
		}
	}
}
