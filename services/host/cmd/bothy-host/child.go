// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// childRun runs args as the service user while the service is stopped (D-M2-42-1); stdin is
// handed to the child (the CA certificates of pair, which the service user may not be able to read
// from their file).
func (a *app) childRun(ctx context.Context, args []string, stdin []byte, stdout, stderr io.Writer) int {
	if a.goos == hostos.Windows {
		return a.doctorOnceWindows(ctx, stdout, stderr)
	}
	uid, gid, err := a.childIDs()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "bothy-host: the service user is missing; run bothy-host install")
		return exitFailed
	}
	code, err := a.asServiceUser(ctx, uid, gid, args, stdin, stdout, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host: %v\n", err)
		return exitFailed
	}
	return code
}
