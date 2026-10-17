// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package helper

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/localchan"
)

func TestServeOverSocket(t *testing.T) {
	d, err := os.MkdirTemp("", "hp")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(d) }()
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", filepath.Join(d, "h.sock"))
	if err != nil {
		t.Fatal(err)
	}
	me := strconv.Itoa(os.Getuid())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- Serve(ctx, Options{Listener: l, ServiceUID: me, Env: world(t)}) }()
	c, err := localchan.DialHelper(context.Background(), filepath.Join(d, "h.sock"), func() (localchan.Expect, error) { return localchan.Expect{UID: me, PID: os.Getpid()}, nil })
	if err != nil {
		t.Fatal(err)
	}
	var hello map[string]any
	if err := c.Call(context.Background(), localchan.OpHello, nil, localchan.OpHello, &hello); err != nil || hello["version"] != float64(1) {
		t.Errorf("%v %v", hello, err)
	}
	if err := c.Call(context.Background(), "nonsense", nil, "x", nil); err == nil {
		t.Error("unknown op answered")
	}
	_ = c.Close()
	cancel()
	if err := <-done; err != nil {
		t.Error(err)
	}
}

func TestLoginEnvAndMkdir(t *testing.T) {
	dir := t.TempDir()
	shell := filepath.Join(dir, "shell")
	for _, tc := range []struct {
		script string
		want   bool
		err    bool
		dir    string
	}{
		{"#!/bin/sh\necho token=1\necho dir=\n", true, false, ""},
		{"#!/bin/sh\necho banner\necho token=0\necho dir=/home/x/c\n", false, false, "/home/x/c"},
		{"#!/bin/sh\necho nothing\n", false, true, ""},
		{"#!/bin/sh\nexit 3\n", false, true, ""},
	} {
		if err := os.WriteFile(shell, []byte(tc.script), 0o700); err != nil { //nolint:gosec // a shell script of the test must be executable
			t.Fatal(err)
		}
		got, cd, err := LoginEnv(hostos.ExecRunner{}, hostos.Account{Name: "bothy-s-alex", Home: dir, Shell: shell})(context.Background(), 30*time.Second)
		if got != tc.want || (err != nil) != tc.err || cd != tc.dir {
			t.Errorf("%q: %v %v", tc.script, got, err)
		}
	}
	// A relative shell falls back to /bin/sh, which prints token=0 with the minimal environment.
	if got, _, err := LoginEnv(hostos.ExecRunner{}, hostos.Account{Home: dir, Shell: "sh"})(context.Background(), 30*time.Second); got || err != nil {
		t.Errorf("fallback %v %v", got, err)
	}
	p := filepath.Join(dir, "a", "b")
	if err := MkdirPrivate(p); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{filepath.Join(dir, "a"), p} {
		fi, _ := os.Stat(q)
		if fi.Mode().Perm() != 0o700 {
			t.Errorf("%s mode %v", q, fi.Mode().Perm())
		}
	}
	if err := Protect(""); err != nil {
		t.Error(err)
	}
}
