// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package hostos

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

const dscacheTwo = `name: bothy-host
password: *
uid: 999
gid: 999
dir: /var/lib/bothy-host
shell: /usr/bin/false

name: bothy-s-alex
password: *
uid: 1001
gid: 20
dir: /home/bothy-s-alex
shell: /bin/zsh
`

// countRunner answers dscacheutil, dscl and ls like macOS and counts the starts per program.
type countRunner struct {
	mu    sync.Mutex
	calls map[string]int
	acl   string
}

func (c *countRunner) Run(ctx context.Context, cmd Cmd) (Output, error) {
	if err := ctx.Err(); err != nil {
		return Output{}, err
	}
	c.mu.Lock()
	c.calls[cmd.Path]++
	c.mu.Unlock()
	switch cmd.Path {
	case "/usr/bin/dscacheutil":
		return Output{Stdout: []byte(dscacheTwo)}, nil
	case "/usr/bin/dscl":
		return Output{Stdout: []byte("PrimaryGroupID: 20\n")}, nil
	case "/bin/ls":
		return Output{Stdout: []byte(c.acl)}, nil
	}
	return Output{ExitCode: 1}, nil
}

func (c *countRunner) count(p string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls[p]
}

// TestScopeReadsTheUserDatabaseOncePerRun: inside one scope every lookup of the macOS user
// database (dscacheutil) and of a group (dscl) runs at most once, however many paths name them.
func TestScopeReadsTheUserDatabaseOncePerRun(t *testing.T) {
	r := &countRunner{calls: map[string]int{}, acl: "drwxr-xr-x+ 3 root wheel 96 Oct 15 09:00 x\n 0: user:bothy-s-alex allow add_file\n 1: group:staff allow list\n 2: user:ghost allow write\n"}
	dir := t.TempDir()
	var paths []string
	for _, n := range []string{"a", "b", "c", "d"} {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	u := DarwinUsers{Runner: r}
	users, sec := Scope(context.Background(), u, DarwinSecurity{Runner: r, Users: u})
	for range 3 {
		if a, err := users.Lookup("bothy-s-alex"); err != nil || a.ID != "1001" || !a.Person {
			t.Fatalf("lookup %+v %v", a, err)
		}
		if _, err := users.Lookup("ghost"); err == nil {
			t.Fatal("an unknown user was found")
		}
		if accs, err := users.Accounts(); err != nil || len(accs) != 2 {
			t.Fatalf("accounts %+v %v", accs, err)
		}
	}
	for _, p := range paths {
		f, err := sec.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		if len(f.ACL) != 3 || f.ACL[0].ID != "1001" || f.ACL[1].ID != "20" || !f.ACL[2].Unmappable {
			t.Fatalf("acl %+v", f.ACL)
		}
	}
	if n := r.count("/usr/bin/dscacheutil"); n != 1 {
		t.Errorf("dscacheutil ran %d times in one run, want 1", n)
	}
	if n := r.count("/usr/bin/dscl"); n != 1 {
		t.Errorf("dscl ran %d times for one group in one run, want 1", n)
	}
	if n := r.count("/bin/ls"); n != len(paths) {
		t.Errorf("ls ran %d times, want one per path (%d)", n, len(paths))
	}
	// A new scope is a new run: it reads the database again.
	users2, _ := Scope(context.Background(), u, DarwinSecurity{Runner: r, Users: u})
	if _, err := users2.Lookup("bothy-s-alex"); err != nil {
		t.Fatal(err)
	}
	if n := r.count("/usr/bin/dscacheutil"); n != 2 {
		t.Errorf("a second run read the database %d times in total, want 2", n)
	}
}

// TestScopeEndsWithTheRun: once the context of the run ended, no program starts any more, also
// when the caller of a command passes a context of its own.
func TestScopeEndsWithTheRun(t *testing.T) {
	r := &countRunner{calls: map[string]int{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	u := DarwinUsers{Runner: r}
	users, sec := Scope(ctx, u, DarwinSecurity{Runner: r, Users: u})
	if _, err := users.Lookup("bothy-s-alex"); err == nil {
		t.Error("a lookup after the end of the run succeeded")
	}
	if _, err := users.Groups("bothy-s-alex"); err == nil {
		t.Error("groups after the end of the run succeeded")
	}
	f, err := sec.Lstat(t.TempDir())
	if err != nil || len(f.ACL) != 1 || !f.ACL[0].Unmappable {
		t.Errorf("lstat after the end of the run: %+v %v", f, err)
	}
	if len(r.calls) != 0 {
		t.Errorf("programs started after the end of the run: %v", r.calls)
	}
}
