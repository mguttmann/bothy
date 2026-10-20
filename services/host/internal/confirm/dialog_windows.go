// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"log/slog"
	"runtime"
	"time"

	"golang.org/x/sys/windows"
)

// Results and messages of MessageBoxW (winuser.h).
const (
	idYes     = 6
	idNo      = 7
	wmCommand = 0x0111
)

// closeRetry is the pause between two attempts to close a dialog after its request ended, and
// closeAttempts bounds them; protocol constants of the agent, not product windows.
const (
	closeRetry    = 50 * time.Millisecond
	closeAttempts = 40
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	procEnumThreadWindows = user32.NewProc("EnumThreadWindows")
	procPostMessageW      = user32.NewProc("PostMessageW")
	// answerNo presses "No" in every top-level window of the thread it is called for; one
	// callback for the whole process, because the runtime keeps every callback forever.
	answerNo = windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		_, _, _ = procPostMessageW.Call(hwnd, wmCommand, idNo, 0)
		return 1
	})
)

// NativeDialog returns the dialog of Windows: MessageBoxW with MB_YESNO, MB_DEFBUTTON2 ("No" is
// the default) and MB_SYSTEMMODAL. MessageBoxW has no timeout, so at the timeout or a cancel the
// agent presses "No" in the box of its thread. environ is unused on Windows.
func NativeDialog(log *slog.Logger, environ []string) Dialog {
	_ = environ
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return func(ctx context.Context, r Request, timeout time.Duration) bool {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		text, err := windows.UTF16PtrFromString(dialogText(r))
		if err != nil {
			return false
		}
		title, _ := windows.UTF16PtrFromString(dialogTitle)
		tid := make(chan uint32, 1)
		res := make(chan int32, 1)
		go func() {
			// The box belongs to this thread, which the agent needs to know to close it.
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			tid <- windows.GetCurrentThreadId()
			ret, _ := windows.MessageBox(0, text, title,
				windows.MB_YESNO|windows.MB_DEFBUTTON2|windows.MB_SYSTEMMODAL|windows.MB_ICONWARNING|windows.MB_SETFOREGROUND)
			res <- ret
		}()
		thread := <-tid
		select {
		case ret := <-res:
			return ret == idYes && ctx.Err() == nil
		case <-ctx.Done():
		}
		for range closeAttempts {
			_, _, _ = procEnumThreadWindows.Call(uintptr(thread), answerNo, 0)
			select {
			case <-res:
				return false
			case <-time.After(closeRetry):
			}
		}
		log.WarnContext(ctx, "confirm_dialog_not_closed")
		return false
	}
}
