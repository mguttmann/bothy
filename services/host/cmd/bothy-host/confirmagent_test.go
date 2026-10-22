// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"testing"
)

// confirm-agent takes no arguments, and it keeps trying an unavailable endpoint until its context
// ends instead of failing: the service may start after the desktop session.
func TestConfirmAgentCommand(t *testing.T) {
	newFake(t)
	if code, _, _ := runCmd("confirm-agent", "extra"); code != exitUsage {
		t.Fatalf("confirm-agent with an argument: %d, want %d", code, exitUsage)
	}
	if code, _, _ := runCmd("confirm-agent", "--bogus"); code != exitUsage {
		t.Fatalf("confirm-agent with an unknown flag: %d, want %d", code, exitUsage)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	if code := run(ctx, []string{"confirm-agent"}, &stdout, &stderr); code != exitOK || bytes.Contains(stderr.Bytes(), []byte("endpoint unavailable")) {
		t.Fatalf("confirm-agent at the end of ctx: %d %q", code, stderr.String())
	}
}
