// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package startup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/localchan"
)

// dscacheCounter answers "dscacheutil -q user" with the accounts of the test world, as macOS
// does, and counts its starts.
type dscacheCounter struct {
	mu  sync.Mutex
	out []byte
	n   int
}

func (d *dscacheCounter) Run(ctx context.Context, c hostos.Cmd) (hostos.Output, error) {
	if err := ctx.Err(); err != nil {
		return hostos.Output{}, err
	}
	if c.Path != "/usr/bin/dscacheutil" {
		return hostos.Output{ExitCode: 1}, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.n++
	return hostos.Output{Stdout: d.out}, nil
}

func (d *dscacheCounter) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.n
}

// TestChannelHelpersShareTheRunsUserCache: the peer check of the helper channel looks the session
// user up; in the real wiring that lookup must come from the one read of the user database of the
// run, not start its own dscacheutil outside the run's cache and context.
func TestChannelHelpersShareTheRunsUserCache(t *testing.T) {
	x := newWorld(t)
	accs, err := x.w.Users().Accounts()
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, a := range accs {
		fmt.Fprintf(&b, "name: %s\npassword: *\nuid: %s\ngid: 20\ndir: %s\nshell: /bin/zsh\n\n", a.Name, a.ID, a.Home)
	}
	d := &dscacheCounter{out: []byte(b.String())}
	users := hostos.DarwinUsers{Runner: d}
	var mu sync.Mutex
	peerChecks := 0
	env := x.deps().Doctor
	env.Users = users
	env.Helpers = ChannelHelpers{GOOS: "linux", Users: users,
		Services: fakeServices{hostos.HelperServiceName("linux", alex): 1, hostos.HelperServiceName("linux", sam): 2},
		Dial: func(_ context.Context, _ string, expect func() (localchan.Expect, error)) (*localchan.Conn, error) {
			if _, err := expect(); err != nil {
				return nil, err
			}
			mu.Lock()
			peerChecks++
			mu.Unlock()
			return nil, errors.New("test: no helper")
		}}
	for i := 1; i <= 2; i++ {
		doctor.RunService(context.Background(), env)
		if n := d.count(); n != i {
			t.Fatalf("run %d: dscacheutil ran %d times in total, want once per run", i, n)
		}
	}
	// The Light run of the gate wraps the helpers in the runner's answer cache.
	doctor.NewRunner(env, nil).RunLight(context.Background())
	if n := d.count(); n != 3 {
		t.Fatalf("Light run: dscacheutil ran %d times, want once", n-2)
	}
	if peerChecks != 6 {
		t.Errorf("peer checks %d, want one per session user and run", peerChecks)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	doctor.RunService(ctx, env)
	if n := d.count(); n != 3 {
		t.Errorf("a cancelled run started dscacheutil %d times", n-3)
	}
}
