// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"os"
	"strconv"
	"syscall"
)

// consolePath is the console device; its owner is the user logged in at the screen.
const consolePath = "/dev/console"

// ConsoleOwner returns the uid of the owner of /dev/console. While the login window shows,
// /dev/console belongs to root and nobody sits at the machine as a person, so uid 0 is an error.
func ConsoleOwner(context.Context) (string, error) {
	fi, err := os.Stat(consolePath)
	if err != nil {
		return "", err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || st.Uid == 0 {
		return "", errNoConsole
	}
	return strconv.FormatUint(uint64(st.Uid), 10), nil
}

// consoleSession is nil on macOS: the owner of /dev/console decides.
var consoleSession func(context.Context, Peer) (bool, error)
