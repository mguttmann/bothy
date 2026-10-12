// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

const usageHeader = "Usage: bothy-host <command> [arguments]\n\nCommands:\n"

func TestRunWithoutArgumentsPrintsUsageAndExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), nil, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if want := usageHeader + "  (none in this build)\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunHelpExits0(t *testing.T) {
	for _, spelling := range []string{"help", "-h", "--help"} {
		t.Run(spelling, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := run(context.Background(), []string{spelling}, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if !strings.HasPrefix(stdout.String(), usageHeader) {
				t.Errorf("stdout = %q, want usage starting with %q", stdout.String(), usageHeader)
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunUnknownCommandExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"frobnicate"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.HasPrefix(stderr.String(), "bothy-host: unknown command \"frobnicate\"\n"+usageHeader) {
		t.Errorf("stderr = %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

// The table is a parameter, so the dispatch to a registered command and the sorted usage are
// tested before M2-42 registers the first real command.
func TestDispatchRunsCommandWithRemainingArguments(t *testing.T) {
	var got []string
	table := []command{
		{name: "zeta", summary: "Last", run: func(_ context.Context, _ []string, _, _ io.Writer) int { return 0 }},
		{name: "alpha", summary: "First", run: func(_ context.Context, args []string, _, _ io.Writer) int {
			got = args
			return 7
		}},
	}
	var stdout, stderr bytes.Buffer

	code := dispatch(context.Background(), table, []string{"alpha", "-x", "y"}, &stdout, &stderr)

	if code != 7 || strings.Join(got, " ") != "-x y" {
		t.Fatalf("code %d, args %q", code, got)
	}
	stdout.Reset()
	if dispatch(context.Background(), table, []string{"--help"}, &stdout, &stderr) != 0 {
		t.Fatal("help did not exit 0")
	}
	if want := usageHeader + "  alpha  First\n  zeta   Last\n"; stdout.String() != want {
		t.Errorf("usage = %q, want %q", stdout.String(), want)
	}
}
