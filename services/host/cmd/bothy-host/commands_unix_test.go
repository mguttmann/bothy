// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/localchan"
)

func currentUID() string { return strconv.Itoa(os.Getuid()) }

func TestDoctorAsAdministrator(t *testing.T) {
	f := newFake(t)
	sys.isAdmin = func() bool { return true }
	code, out, _ := runCmd("doctor", "--config", f.cfg)
	if code != 0 || out != "child\n" || len(f.child) != 1 || f.child[0][0] != "doctor" {
		t.Errorf("child way: %d %q %v", code, out, f.child)
	}
	// A running service answers over the admin endpoint.
	l, path := admin(t)
	go serveFakeAdmin(l)
	sys.dialAdmin = func(ctx context.Context, _, _ string) (*localchan.Conn, error) {
		return localchan.DialAdmin(ctx, path, currentUID())
	}
	code, out, _ = runCmd("doctor", "--config", f.cfg)
	if code != 1 || out != "HD1 FAIL x\n" {
		t.Errorf("service way: %d %q", code, out)
	}
	code, out, stderr := runCmd("pair", "--server", "https://chat.example.com", "--code", "bothy_hpc_EXAMPLE", "--config", f.cfg)
	if code != 0 || out != "fingerprint: F0CC\nhost: h\n" || len(f.applied) != 1 || strings.Contains(out+stderr, "bothy_hpc_") {
		t.Errorf("pair through the service: %d %q %q %d", code, out, stderr, len(f.applied))
	}
}

func admin(t *testing.T) (net.Listener, string) {
	t.Helper()
	d, err := os.MkdirTemp("", "ad")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	p := filepath.Join(d, "a.sock")
	l, err := localchan.ListenAdmin(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l, p
}

func serveFakeAdmin(l net.Listener) {
	_ = localchan.ServeAdmin(context.Background(), l, currentUID(), func(_ context.Context, m localchan.Message) (string, any) {
		if m.Op == localchan.OpPairRun {
			return localchan.OpPairRes, map[string]string{"fingerprint": "F0CC", "host_id": "h"}
		}
		return localchan.OpDoctorRes, map[string]any{"output": "HD1 FAIL x\n", "exit": 1}
	})
}
