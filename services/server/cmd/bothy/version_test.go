// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
)

// TestVersionCommand (AUD-PKG-R2-owner-server, #1624): bothy version prints the version and the
// embedded commit on one line and exits 0; an argument is a usage error (exit 2).
func TestVersionCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"version"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, stderr %q", code, stderr.String())
	}
	if want := buildinfo.Version() + " " + buildinfo.Revision() + "\n"; stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
	stdout.Reset()
	if code := run(context.Background(), []string{"version", "x"}, nil, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
		t.Errorf("version with an argument: exit %d, stdout %q; want 2 and nothing", code, stdout.String())
	}
}
