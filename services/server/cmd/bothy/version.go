// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"

	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
)

// versionCommand prints "<version> <revision>" on one line: the version of services/server/VERSION
// and the commit the build embedded ("unknown" without it). It takes no arguments (exit 2).
func versionCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		_, _ = fmt.Fprintln(stderr, "bothy version: takes no arguments")
		return 2
	}
	_, _ = fmt.Fprintln(stdout, buildinfo.Version(), buildinfo.Revision())
	return 0
}
