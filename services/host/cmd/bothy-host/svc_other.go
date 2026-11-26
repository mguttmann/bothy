// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"context"
	"io"
)

// serviceStderr is the standard error itself: launchd and systemd keep it or the journal does.
func serviceStderr(w io.Writer) io.Writer { return w }

// serviceMode is Windows only.
func serviceMode(func(ctx context.Context, once bool) int) (int, bool) { return 0, false }

// doctorOnceWindows is Windows only.
func (a *app) doctorOnceWindows(context.Context, io.Writer, io.Writer) int { return exitFailed }
