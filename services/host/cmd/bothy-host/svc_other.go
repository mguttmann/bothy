// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"context"
	"io"
)

// serviceMode is Windows only.
func serviceMode(func(ctx context.Context, once bool) int) (int, bool) { return 0, false }

// doctorOnceWindows is Windows only.
func (a *app) doctorOnceWindows(context.Context, io.Writer, io.Writer) int { return exitFailed }
