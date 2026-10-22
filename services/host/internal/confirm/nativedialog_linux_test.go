// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"testing"
	"time"
)

// The native dialog denies a request whose context already ended, whether or not a dialog
// program is installed: no program starts for it.
func TestNativeDialogDeniesEndedRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if NativeDialog(nil, []string{"DISPLAY=:0"})(ctx, reqOpen, time.Second) {
		t.Fatal("the native dialog allowed an ended request")
	}
}

// A dialog without a log still denies without a dialog program.
func TestLinuxDialogWithoutLog(t *testing.T) {
	f := &exitRunner{}
	d := linuxDialog(nil, f, func(string) bool { return false }, nil)
	if d(context.Background(), reqOpen, time.Second) || len(f.calls) != 0 {
		t.Fatalf("without a dialog program: calls %+v", f.calls)
	}
}
