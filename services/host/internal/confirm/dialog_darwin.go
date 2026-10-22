// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// osascriptPath is the AppleScript runner of macOS.
const osascriptPath = "/usr/bin/osascript"

// osascriptScript shows the question with "Deny" as default and cancel button and gives up after
// the timeout. Text and seconds arrive as arguments of the run handler (item 1 and 2 of argv) and
// are never part of the script, so no value can change it. Every way out but the button "Allow"
// (Deny, Escape, giving up, an error) returns "deny".
var osascriptScript = []string{
	"on run argv",
	`set answer to "deny"`,
	"try",
	`set r to display dialog (item 1 of argv) with title "` + dialogTitle + `" buttons {"Deny", "Allow"} default button "Deny" cancel button "Deny" with icon caution giving up after ((item 2 of argv) as integer)`,
	`if (gave up of r) is false and (button returned of r) is "Allow" then set answer to "allow"`,
	"end try",
	"return answer",
	"end run",
}

// NativeDialog returns the dialog of macOS: /usr/bin/osascript with display dialog, "Deny" as
// default button and giving up after the timeout. Without osascript every request is denied and
// logged with the code no_dialog_tool. environ is unused on macOS: osascript needs no variable of
// the desktop session.
func NativeDialog(log *slog.Logger, environ []string) Dialog {
	_ = environ
	return osascriptDialog(log, hostos.ExecRunner{}, toolExists)
}

func osascriptDialog(log *slog.Logger, run hostos.Runner, exists func(string) bool) Dialog {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return func(ctx context.Context, r Request, timeout time.Duration) bool {
		if !exists(osascriptPath) {
			log.WarnContext(ctx, "no_dialog_tool")
			return false
		}
		args := make([]string, 0, 2*len(osascriptScript)+2)
		for _, l := range osascriptScript {
			args = append(args, "-e", l)
		}
		args = append(args, dialogText(r), strconv.Itoa(dialogSeconds(timeout)))
		out, err := run.Run(ctx, hostos.Cmd{Path: osascriptPath, Args: args})
		return err == nil && !out.TimedOut && out.ExitCode == 0 && strings.TrimSpace(string(out.Stdout)) == "allow"
	}
}
