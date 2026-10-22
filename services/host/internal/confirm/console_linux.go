// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// loginctlPaths are the places of loginctl, tried in this order; never a lookup in PATH.
var loginctlPaths = []string{"/usr/bin/loginctl", "/bin/loginctl"}

// ConsoleOwner returns the uid of the user of the active session on seat0 (loginctl show-seat
// seat0 -p ActiveSession, then show-session -p User), run with an empty environment. It returns
// an error when loginctl is missing, no session is active or the output is not as expected.
func ConsoleOwner(ctx context.Context) (string, error) {
	for _, p := range loginctlPaths {
		if toolExists(p) {
			return loginctlOwner(ctx, hostos.ExecRunner{}, p)
		}
	}
	return "", errNoConsole
}

// consoleSession is nil on Linux: the owner of the active seat0 session decides.
var consoleSession func(context.Context, Peer) (bool, error)
