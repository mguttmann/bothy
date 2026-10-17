// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package hostos

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestUnixAdapters(t *testing.T) {
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "f")
	if err := os.WriteFile(f, nil, 0o640); err != nil { //nolint:gosec // the mode under test
		t.Fatal(err)
	}
	users, sec, p := Platform(ExecRunner{})
	fs, err := sec.Lstat(f)
	if err != nil || fs.OwnerID != me.Uid || fs.Mode.Perm() != 0o640 || fs.IsDir {
		t.Errorf("%+v %v", fs, err)
	}
	if _, err := sec.Lstat(filepath.Join(dir, "none")); err == nil {
		t.Error("missing file")
	}
	if err := os.Symlink(f, filepath.Join(dir, "l")); err != nil {
		t.Fatal(err)
	}
	if l, err := sec.Lstat(filepath.Join(dir, "l")); err != nil || !l.IsLink {
		t.Errorf("link %+v %v", l, err)
	}
	g, err := users.Groups(me.Username)
	if err != nil || len(g) == 0 {
		t.Errorf("groups %v %v", g, err)
	}
	if _, err := users.Groups("bothy-no-such-user"); err == nil {
		t.Error("groups of an unknown user")
	}
	accs, err := users.Accounts()
	if err != nil || len(accs) == 0 {
		t.Fatalf("accounts %v", err)
	}
	if a, err := users.Lookup("root"); err != nil || a.ID != "0" || a.Person {
		t.Errorf("root %+v %v", a, err)
	}
	if _, err := users.Lookup("bothy-no-such-user"); err == nil {
		t.Error("unknown user found")
	}
	if _, err := p.SudoWithoutPassword(context.Background()); err != nil {
		t.Error(err)
	}
	if _, err := p.PtraceScope(); err != nil {
		t.Error(err)
	}
}
