// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package install

import (
	"context"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/hostos/hostostest"
)

func TestOSOps(t *testing.T) {
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	w := hostostest.New("linux")
	w.AddUser("tester", me.Uid, dir, false)
	w.SetGroups("tester", hostos.Group{Name: "g", ID: me.Gid})
	ops := OSOps{Runner: hostos.ExecRunner{}, Users: w.Users()}
	ctx := context.Background()
	p := filepath.Join(dir, "a", "b")
	if err := ops.Mkdir(p, 0o700, "tester"); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(p, "unit")
	for range 2 {
		if err := ops.Write(f, []byte("x"), 0o644, "tester"); err != nil {
			t.Fatal(err)
		}
	}
	if fi, _ := os.Stat(f); fi.Mode().Perm() != 0o644 {
		t.Errorf("mode %v", fi.Mode())
	}
	if err := ops.Copy(f, filepath.Join(p, "bin"), 0o755, "tester"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(p, "bin")); string(b) != "x" { //nolint:gosec // the file of this test
		t.Error("copy content")
	}
	if err := ops.Chmod(p, 0o711, ""); err != nil {
		t.Fatal(err)
	}
	if err := ops.Chmod(p, 0o700, "tester"); err != nil {
		t.Fatal(err)
	}
	if err := ops.Remove(f); err != nil || ops.Remove(f) != nil {
		t.Error("remove")
	}
	if h, err := ops.Home("tester"); err != nil || h != dir {
		t.Error("home")
	}
	if _, err := ops.Home("ghost"); err == nil {
		t.Error("ghost home")
	}
	if err := ops.Mkdir(filepath.Join(dir, "x"), 0o700, "ghost"); err == nil {
		t.Error("unknown owner")
	}
	if err := ops.Copy(filepath.Join(dir, "none"), filepath.Join(dir, "y"), 0o755, "tester"); err == nil {
		t.Error("missing source")
	}
	if err := ops.Run(ctx, []string{"/usr/bin/true"}); err != nil {
		t.Error(err)
	}
	if err := ops.Run(ctx, []string{"/usr/bin/false"}); err == nil {
		t.Error("exit code ignored")
	}
	if err := ops.Run(ctx, []string{"true"}); err == nil {
		t.Error("relative program")
	}
	if err := ops.Windows(ctx, Step{Kind: KindWinUser}); !errors.Is(err, ErrNoWindows) {
		t.Error("windows step")
	}
	if uid, gid, err := ops.ids("root"); err != nil || uid != 0 || gid != 0 {
		t.Error("root ids")
	}
}
