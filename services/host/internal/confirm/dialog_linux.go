// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// Dialog programs of Linux, tried in this order (brief M2-45 Design note 15 c).
const (
	zenityPath  = "/usr/bin/zenity"
	kdialogPath = "/usr/bin/kdialog"
)

// dialogEnvKeys are the variables of the desktop session a dialog program needs to reach the
// display and the session bus; every other variable of the agent stays out.
var dialogEnvKeys = []string{
	"DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY", "XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS",
	"HOME", "LANG", "LC_ALL", "LC_MESSAGES",
}

// markupEscape escapes the Pango markup characters, because zenity renders --text as markup.
var markupEscape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace

// NativeDialog returns the dialog of Linux: /usr/bin/zenity --question with "Deny" as default
// and --timeout, or else /usr/bin/kdialog --warningyesno (whose default is "No"), ended through
// the context at the timeout. environ is the environment of the agent (os.Environ in
// cmd/bothy-host); only the variables of dialogEnvKeys reach the dialog program. Without either
// program every request is denied and logged with the code no_dialog_tool.
func NativeDialog(log *slog.Logger, environ []string) Dialog {
	return linuxDialog(log, hostos.ExecRunner{}, toolExists, environ)
}

func linuxDialog(log *slog.Logger, run hostos.Runner, exists func(string) bool, environ []string) Dialog {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	env := make([]string, 0, len(dialogEnvKeys))
	for _, kv := range environ {
		k, _, ok := strings.Cut(kv, "=")
		if ok && slices.Contains(dialogEnvKeys, k) {
			env = append(env, kv)
		}
	}
	return func(ctx context.Context, r Request, timeout time.Duration) bool {
		var c hostos.Cmd
		switch {
		case exists(zenityPath):
			c = hostos.Cmd{Path: zenityPath, Args: []string{
				"--question", "--title=" + dialogTitle, "--text=" + markupEscape(dialogText(r)),
				"--timeout=" + strconv.Itoa(dialogSeconds(timeout)), "--ok-label=Allow", "--cancel-label=Deny", "--default-cancel",
			}}
		case exists(kdialogPath):
			c = hostos.Cmd{Path: kdialogPath, Args: []string{
				"--title", dialogTitle, "--warningyesno", dialogText(r), "--yes-label", "Allow", "--no-label", "Deny",
			}}
		default:
			log.WarnContext(ctx, "no_dialog_tool")
			return false
		}
		c.Env = env
		out, err := run.Run(ctx, c)
		// Exit code 0 is "yes"; 1 is "no", 5 the timeout of zenity, -1 the end of ctx.
		return err == nil && !out.TimedOut && out.ExitCode == 0
	}
}
