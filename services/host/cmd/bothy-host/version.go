// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"

	"github.com/OWNER/bothy/services/host/internal/buildinfo"
)

// cmdVersion is "version": it prints the version of this build (buildinfo.Version, set from
// services/host/VERSION by make release-build) on one line and exits 0; any argument is a usage
// error (2). The release trial and the package tests compare this line with the tag (M7-23).
func cmdVersion(_ context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		_, _ = fmt.Fprintln(stderr, "usage: bothy-host version")
		return exitUsage
	}
	_, _ = fmt.Fprintln(stdout, buildinfo.Version)
	return exitOK
}
