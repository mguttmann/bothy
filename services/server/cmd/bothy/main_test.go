// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
)

const usageHeader = "Usage: bothy <command> [arguments]\n\nCommands:\n"

func TestRunWithoutArgumentsPrintsUsageAndExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), nil, nil, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	want := usageHeader +
		"  config       Check or print the configuration without starting the server\n" +
		"  healthcheck  Probe a loopback health endpoint for a container without shell\n" +
		"  import       Import a Discord server; log the import tool in or out (device pairing as a cli device)\n" +
		"  migrate      Apply, list or validate the database migrations\n" +
		"  restore      Finish a restore or a relocation before api and worker start\n" +
		"  secrets      Create the secrets of the Compose stack or export them encrypted\n" +
		"  serve        Start the server in the role api, worker or all\n" +
		"  setup        Create the first owner or a recovery code on the host\n" +
		"  telemetry    Map a principal id to its telemetry pseudonym for the owner\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestUsageWithoutCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := dispatch(context.Background(), nil, []string{"help"}, nil, &stdout, &stderr)

	if code != 0 || stdout.String() != usageHeader+"  (none in this build)\n" {
		t.Errorf("exit code %d, usage %q", code, stdout.String())
	}
}

func TestRunHelpExits0(t *testing.T) {
	for _, spelling := range []string{"help", "-h", "--help"} {
		t.Run(spelling, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := run(context.Background(), []string{spelling}, nil, &stdout, &stderr)

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

	code := run(context.Background(), []string{"frobnicate", "--x"}, nil, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	firstLine, rest, found := strings.Cut(stderr.String(), "\n")
	if !found || firstLine != `bothy: unknown command "frobnicate"` {
		t.Errorf("first stderr line = %q, want %q", firstLine, `bothy: unknown command "frobnicate"`)
	}
	if !strings.HasPrefix(rest, usageHeader) {
		t.Errorf("stderr after the error line = %q, want usage starting with %q", rest, usageHeader)
	}
}

func TestRunDispatchesRegisteredCommand(t *testing.T) {
	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	var gotArgs, gotEnv []string
	var gotCtxValue any
	table := []command{
		{name: "other", summary: "never called", run: func(context.Context, []string, []string, io.Writer, io.Writer) int {
			t.Error("command other must not run")
			return 99
		}},
		{name: "probe", summary: "records its call", run: func(c context.Context, args, env []string, stdout, stderr io.Writer) int {
			gotArgs, gotEnv = args, env
			gotCtxValue = c.Value(ctxKey{})
			_, _ = io.WriteString(stdout, "out\n")
			_, _ = io.WriteString(stderr, "err\n")
			return 7
		}},
	}
	var stdout, stderr bytes.Buffer

	code := dispatch(ctx, table, []string{"probe", "--flag", "value"}, []string{"BOTHY_X__Y=1"}, &stdout, &stderr)

	if code != 7 {
		t.Errorf("exit code = %d, want the command's 7", code)
	}
	if want := []string{"--flag", "value"}; !reflect.DeepEqual(gotArgs, want) {
		t.Errorf("command args = %q, want %q", gotArgs, want)
	}
	if want := []string{"BOTHY_X__Y=1"}; !reflect.DeepEqual(gotEnv, want) {
		t.Errorf("command env = %q, want the caller's %q", gotEnv, want)
	}
	if gotCtxValue != "marker" {
		t.Errorf("command context value = %v, want the caller's context", gotCtxValue)
	}
	if stdout.String() != "out\n" || stderr.String() != "err\n" {
		t.Errorf("stdout, stderr = %q, %q; want the command's own output only", stdout.String(), stderr.String())
	}
}

func TestUsageListsCommandsSortedByName(t *testing.T) {
	noop := func(context.Context, []string, []string, io.Writer, io.Writer) int { return 0 }
	table := []command{
		{name: "serve", summary: "Start the server", run: noop},
		{name: "config", summary: "Check configuration", run: noop},
		{name: "migrate", summary: "Run migrations", run: noop},
	}
	original := append([]command(nil), table...)
	var stdout, stderr bytes.Buffer

	code := dispatch(context.Background(), table, []string{"help"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := usageHeader +
		"  config   Check configuration\n" +
		"  migrate  Run migrations\n" +
		"  serve    Start the server\n"
	if stdout.String() != want {
		t.Errorf("usage = %q, want %q", stdout.String(), want)
	}
	for i := range table {
		if table[i].name != original[i].name {
			t.Fatalf("table reordered in place: position %d is %q, want %q", i, table[i].name, original[i].name)
		}
	}
}
