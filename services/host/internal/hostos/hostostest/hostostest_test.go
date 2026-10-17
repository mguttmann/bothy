// SPDX-License-Identifier: Apache-2.0

package hostostest

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

func TestWorld(t *testing.T) {
	w := New("linux")
	w.AddUser("alex", "1001", "/home/alex", true)
	w.Dir("/home/alex", "1001", 0o700)
	w.File("/home/alex/f", "1001", 0o600, "EXAMPLE")
	w.Link("/home/alex/l", "1001", "/home/alex/f")
	if w.UID("alex") != "1001" || w.UID("ghost") != "" {
		t.Error("UID")
	}
	if _, err := w.Users().Lookup("ghost"); !errors.Is(err, hostos.ErrUnknownUser) {
		t.Error("lookup")
	}
	if _, err := w.Users().Groups("ghost"); err == nil {
		t.Error("groups")
	}
	if accs, _ := w.Users().Accounts(); len(accs) != 2 {
		t.Error("accounts")
	}
	if _, err := w.Security().Lstat("/none"); err == nil {
		t.Error("lstat")
	}
	alex, root := w.As("alex"), w.As("root")
	if names, err := alex.ReadDir("/home/alex"); err != nil || len(names) != 2 {
		t.Errorf("%v %v", names, err)
	}
	if _, err := w.As("nobody").ReadDir("/home/alex"); err == nil {
		t.Error("closed home listed")
	}
	if b, err := alex.Head("/home/alex/f", 3); err != nil || string(b) != "EXA" {
		t.Errorf("%q %v", b, err)
	}
	if id, err := alex.Identity("/home/alex/l"); err != nil || id != "/home/alex/f" {
		t.Errorf("%q %v", id, err)
	}
	if _, err := alex.Identity("/none"); err == nil {
		t.Error("identity of a missing path")
	}
	if _, err := alex.Readlink("/home/alex/f"); err == nil {
		t.Error("readlink of a file")
	}
	if ok, _ := root.Probe().SudoWithoutPassword(context.Background()); ok {
		t.Error("sudo")
	}
	if s, _ := root.Probe().PtraceScope(); s != 1 {
		t.Error("ptrace")
	}
	if s, _ := New("darwin").As("root").Probe().PtraceScope(); s != -1 {
		t.Error("ptrace darwin")
	}
	w.Remove("/home/alex")
	if w.Node("/home/alex/f") != nil {
		t.Error("remove")
	}
	r := &Runner{}
	if _, err := r.Run(context.Background(), hostos.Cmd{Path: "/bin/x"}); err != nil || len(r.Recorded()) != 1 {
		t.Error("runner")
	}
	if w.Subject("alex").ID != "1001" {
		t.Error("subject")
	}
}
