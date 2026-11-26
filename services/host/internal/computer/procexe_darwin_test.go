// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package computer

import (
	"os"
	"testing"
	"time"
)

// TestDefaultStartTimeAndExePath: the start time and the executable of this test process, read
// without cgo, are stable and the start lies in the past.
func TestDefaultStartTimeAndExePath(t *testing.T) {
	a, err := DefaultStartTime(os.Getpid())
	if err != nil || !a.Before(time.Now()) {
		t.Fatalf("start %v %v", a, err)
	}
	if b, err := DefaultStartTime(os.Getpid()); err != nil || !b.Equal(a) {
		t.Errorf("second read %v %v", b, err)
	}
	exe, err := DefaultExePath(os.Getpid())
	self, _ := os.Executable()
	if err != nil || exe != self {
		t.Errorf("exe %q %v, want %q", exe, err, self)
	}
}
