// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// fakeLoginctl answers show-seat and show-session like loginctl.
type fakeLoginctl struct {
	seat, session string
	exit          int
	err           error
	calls         []hostos.Cmd
}

func (f *fakeLoginctl) Run(_ context.Context, c hostos.Cmd) (hostos.Output, error) {
	f.calls = append(f.calls, c)
	if f.err != nil {
		return hostos.Output{}, f.err
	}
	out := f.session
	if slices.Contains(c.Args, "show-seat") {
		out = f.seat
	}
	return hostos.Output{Stdout: []byte(out), ExitCode: f.exit}, nil
}

func TestLoginctlConsoleOwner(t *testing.T) {
	ctx := context.Background()
	f := &fakeLoginctl{seat: "ActiveSession=c2\n", session: "User=1004\n"}
	uid, err := loginctlOwner(ctx, f, "/usr/bin/loginctl")
	if err != nil || uid != uidRobin {
		t.Fatalf("loginctlOwner = %q, %v; want %s", uid, err, uidRobin)
	}
	want := [][]string{{"show-seat", "seat0", "-p", "ActiveSession"}, {"show-session", "c2", "-p", "User"}}
	for i, c := range f.calls {
		if c.Path != "/usr/bin/loginctl" || !slices.Equal(c.Args, want[i]) || len(c.Env) != 0 {
			t.Errorf("call %d = %+v, want %v with an empty environment", i, c, want[i])
		}
	}
	for name, f := range map[string]*fakeLoginctl{
		"no active session":    {seat: "ActiveSession=\n", session: "User=1004\n"},
		"no seat":              {seat: "", session: "User=1004\n"},
		"odd session id":       {seat: "ActiveSession=c2 -p x\n", session: "User=1004\n"},
		"session id as option": {seat: "ActiveSession=-H\n", session: "User=1004\n"},
		"no user":              {seat: "ActiveSession=c2\n", session: "User=\n"},
		"user not a number":    {seat: "ActiveSession=c2\n", session: "User=robin\n"},
		"exit code":            {seat: "ActiveSession=c2\n", session: "User=1004\n", exit: 1},
		"start fails":          {err: errors.New("no loginctl")},
	} {
		if uid, err := loginctlOwner(ctx, f, "/usr/bin/loginctl"); err == nil {
			t.Errorf("%s: loginctlOwner = %q without an error", name, uid)
		}
	}
}

// TestFirstOwner: the console owner comes from the first source that names a SID; the session
// query that a virtual service account may not make (access denied on a Windows 11 test machine)
// falls through to the LogonUI session data, and a value that is no SID never counts.
func TestFirstOwner(t *testing.T) {
	const sid = "S-1-5-21-3458425980-4001700-720649186-1001"
	denied := func() (string, error) { return "", errors.New("access denied") }
	ok := func() (string, error) { return sid, nil }
	if got, err := firstOwner(denied, ok); err != nil || got != sid {
		t.Errorf("fallback: %q %v", got, err)
	}
	other := func() (string, error) { return "S-1-5-21-1-2-3-1002", nil }
	if got, _ := firstOwner(ok, other); got != sid {
		t.Errorf("the first source must win: %q", got)
	}
	garbage := func() (string, error) { return "Alex", nil }
	if _, err := firstOwner(garbage, denied); !errors.Is(err, errNoConsole) {
		t.Errorf("no SID: %v", err)
	}
	if _, err := firstOwner(); !errors.Is(err, errNoConsole) {
		t.Errorf("no source: %v", err)
	}
}
