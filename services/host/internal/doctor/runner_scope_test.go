// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package doctor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// dscacheRunner answers "dscacheutil -q user" with the accounts of the test world, as macOS
// does, and counts every program start.
type dscacheRunner struct {
	mu    sync.Mutex
	out   []byte
	users hostos.Users
	calls map[string]int
}

func newDscacheRunner(t *testing.T, x *tw) *dscacheRunner {
	t.Helper()
	accs, err := x.w.Users().Accounts()
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, a := range accs {
		fmt.Fprintf(&b, "name: %s\npassword: *\nuid: %s\ngid: 20\ndir: %s\nshell: /bin/zsh\n\n", a.Name, a.ID, a.Home)
	}
	return &dscacheRunner{out: []byte(b.String()), users: x.w.Users(), calls: map[string]int{}}
}

func (d *dscacheRunner) Run(ctx context.Context, c hostos.Cmd) (hostos.Output, error) {
	if err := ctx.Err(); err != nil {
		return hostos.Output{}, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls[c.Path]++
	switch c.Path {
	case "/usr/bin/dscacheutil":
		return hostos.Output{Stdout: d.out}, nil
	case "/usr/bin/id": // "id -G name" and "id -Gn name" from the test world
		gs, err := d.users.Groups(c.Args[1])
		if err != nil {
			return hostos.Output{ExitCode: 1}, nil
		}
		var f []string
		for _, g := range gs {
			if c.Args[0] == "-G" {
				f = append(f, g.ID)
			} else {
				f = append(f, g.Name)
			}
		}
		return hostos.Output{Stdout: []byte(strings.Join(f, " ") + "\n")}, nil
	}
	return hostos.Output{ExitCode: 1}, nil
}

func (d *dscacheRunner) count(p string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls[p]
}

// TestRunServiceReadsTheUserDatabaseOnce: one run looks accounts up in HD1, HD2, HD4, HD8, HD9
// and the owner account checks; on macOS each lookup started dscacheutil, so the gate before a
// PTY started dozens of them. Now one run reads the database once and its verdicts stay.
func TestRunServiceReadsTheUserDatabaseOnce(t *testing.T) {
	x := newTW(t)
	d := newDscacheRunner(t, x)
	x.env.Users = hostos.DarwinUsers{Runner: d}
	for i := 1; i <= 2; i++ {
		res := RunService(context.Background(), x.env)
		if n := d.count("/usr/bin/dscacheutil"); n != i {
			t.Fatalf("run %d: dscacheutil ran %d times in total, want once per run", i, n)
		}
		// "id -G" and "id -Gn" once per account: the service user and the three session users.
		if n := d.count("/usr/bin/id"); n != 2*4*i {
			t.Errorf("run %d: id ran %d times in total, want twice per account per run", i, n)
		}
		for _, c := range res.Checks {
			for _, e := range c.Entries {
				if e.Status == Fail {
					t.Errorf("run %d: %s fails with %s", i, c.Check, e.Code)
				}
			}
		}
	}
}

// TestRunServiceStopsWithItsContext: a run whose context ended (the opening window of the gate
// closed) starts no program of the user database any more.
func TestRunServiceStopsWithItsContext(t *testing.T) {
	x := newTW(t)
	d := newDscacheRunner(t, x)
	x.env.Users = hostos.DarwinUsers{Runner: d}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	RunService(ctx, x.env)
	if len(d.calls) != 0 {
		t.Errorf("a cancelled run started programs: %v", d.calls)
	}
}

// TestRunLightUserCancelled: a Light run whose context ended gives no result, keeps the last
// result and does not keep the answers its helpers gave, so a later run of another session user
// answers from the earlier complete run.
func TestRunLightUserCancelled(t *testing.T) {
	x := newTW(t)
	c := &countingHelpers{tw: x, calls: map[string]int{}}
	x.env.Helpers = c
	r := NewRunner(x.env, nil)
	first := r.RunLight(context.Background())
	if first == nil || first.Check(HD6).Failed() {
		t.Fatalf("first run %+v", first)
	}
	x.auth[sam] = map[string]string{"": "claude.ai"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if res := r.RunLightUser(ctx, sam); res != nil {
		t.Error("a cancelled run gave a result")
	}
	if last := r.Last(); last == nil || last.Check(HD6).Failed() {
		t.Errorf("a cancelled run replaced the last result: %+v", last)
	}
	if res := r.RunLightUser(context.Background(), alex); res == nil || res.Check(HD6).Failed() {
		t.Error("the answer of a cancelled run was kept for the next run")
	}
}
