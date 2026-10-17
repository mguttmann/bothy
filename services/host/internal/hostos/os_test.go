// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestOSResolverAndLister(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "real"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	r, err := OSResolver{}.Resolve(filepath.Join(dir, "link"))
	if err != nil || r.Final != filepath.Join(dir, "real") || len(r.Links) != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	a, err1 := OSResolver{}.Identity(filepath.Join(dir, "link"))
	b, err2 := OSResolver{}.Identity(filepath.Join(dir, "real"))
	if err1 != nil || err2 != nil || a != b || a == "" {
		t.Errorf("identity %q %q", a, b)
	}
	if _, err := (OSResolver{}).Identity(filepath.Join(dir, "none")); !errors.Is(err, ErrUnresolvable) {
		t.Error("missing identity")
	}
	names, err := OSLister{}.ReadDir(dir)
	if err != nil || len(names) != 2 {
		t.Errorf("%v %v", names, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "real", "f"), []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if h, err := (OSLister{}).Head(filepath.Join(dir, "link", "f"), 2); err != nil || string(h) != "#!" {
		t.Errorf("head %q %v", h, err)
	}
	if h, err := (OSLister{}).Head(filepath.Join(dir, "real", "f"), 100); err != nil || len(h) != 10 {
		t.Errorf("short head %q %v", h, err)
	}
	if _, err := (OSLister{}).Head(filepath.Join(dir, "none"), 1); err == nil {
		t.Error("missing head")
	}
	if _, err := (OSLister{}).ReadDir(filepath.Join(dir, "none")); err == nil {
		t.Error("missing listed")
	}
	if _, err := (OSLinkFS{}).Readlink(filepath.Join(dir, "real")); err == nil {
		t.Error("readlink of a folder")
	}
}

func TestExecRunner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh")
	}
	r := ExecRunner{}
	out, err := r.Run(context.Background(), Cmd{Path: "/bin/sh", Args: []string{"-c", "echo $HOME; cat; exit 3"}, Stdin: []byte("in")})
	if err != nil || out.ExitCode != 3 || strings.TrimSpace(string(out.Stdout)) != "in" {
		t.Errorf("%+v %v (the environment must be empty)", out, err)
	}
	out, err = r.Run(context.Background(), Cmd{Path: "/bin/sh", Args: []string{"-c", "sleep 5"}, Timeout: 50 * time.Millisecond})
	if err != nil || !out.TimedOut {
		t.Errorf("timeout %+v %v", out, err)
	}
	if _, err := r.Run(context.Background(), Cmd{Path: "sh"}); !errors.Is(err, ErrRelativeProgram) {
		t.Errorf("relative: %v", err)
	}
	if _, err := r.Run(context.Background(), Cmd{Path: "/nonexistent/prog"}); err == nil {
		t.Error("missing program")
	}
}

// TestExecRunnerTimeoutWithChild: a child that keeps the output open does not stretch the bound
// (review finding of M2-42).
func TestExecRunnerTimeoutWithChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh")
	}
	start := time.Now()
	out, err := ExecRunner{}.Run(context.Background(), Cmd{Path: "/bin/sh", Args: []string{"-c", "sleep 30 & sleep 30"}, Timeout: 100 * time.Millisecond})
	if err != nil || !out.TimedOut {
		t.Errorf("%+v %v", out, err)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("the run took %v", d)
	}
}
