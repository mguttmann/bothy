// SPDX-License-Identifier: Apache-2.0

package install

import (
	"context"
	"strings"
	"testing"
)

// TestApplyNamesFailedRunStep: a failed run step is named by the base name of its program and its
// first argument only, never by a later argument (a value), and no step after it runs.
func TestApplyNamesFailedRunStep(t *testing.T) {
	steps := []Step{
		{Kind: KindRun, Argv: []string{"/usr/sbin/usermod", "-aG", "value-group", "bothy-s-alex"}},
		{Kind: KindRemove, Path: "/after"},
	}
	ops := &fakeOps{fail: "run /usr/sbin/usermod"}
	err := Apply(context.Background(), ops, steps)
	if err == nil || !strings.HasPrefix(err.Error(), "install: run usermod -aG: ") {
		t.Fatalf("error %v", err)
	}
	if strings.Contains(err.Error(), "value-group") || strings.Contains(err.Error(), "bothy-s-alex") || strings.Contains(err.Error(), "/usr/sbin") {
		t.Errorf("the error names a value or the path: %v", err)
	}
	if len(ops.log) != 1 {
		t.Errorf("steps after the failure ran: %v", ops.log)
	}
	err = Apply(context.Background(), &fakeOps{fail: "run /usr/bin/true"}, []Step{{Kind: KindRun, Argv: []string{"/usr/bin/true"}}})
	if err == nil || !strings.HasPrefix(err.Error(), "install: run true: ") {
		t.Errorf("a program without arguments: %v", err)
	}
}
