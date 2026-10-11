// SPDX-License-Identifier: Apache-2.0

//go:build !(android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd)

package stack

import (
	"errors"
	"os"
)

// errNoFlock fails init closed where flock(2) is missing: without the lock two runs could remove
// each other's temp files and replace a generated secret a service already copied.
var errNoFlock = errors.New("secrets init needs flock(2) to keep two runs on one target apart; this platform has none")

// lockTarget cannot lock on this platform and refuses the run.
func lockTarget(*os.Root) (func(), error) { return nil, errNoFlock }
