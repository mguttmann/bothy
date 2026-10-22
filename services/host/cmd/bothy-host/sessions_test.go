// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/confirm"
	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// writeKeys stores a fixed key pair of the host in st.
func writeKeys(t *testing.T, st *state.Dir) {
	t.Helper()
	k := state.Keys{AuthPrivate: ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))}
	k.NoisePrivate[0] = 1
	if err := st.WriteKeys(k); err != nil {
		t.Fatal(err)
	}
}

func writePairing(t *testing.T, st *state.Dir, serverURL string) {
	t.Helper()
	if err := st.WritePairing(state.Pairing{ServerURL: serverURL, HostID: "h", InstanceID: "i", Fingerprint: "F"}); err != nil {
		t.Fatal(err)
	}
}

// confirmEndpointAvailable reports whether the folder of the confirmation endpoint of goos exists
// on this machine (a host that runs bothy-host); the tests of a missing endpoint skip there.
func confirmEndpointAvailable(goos string) bool {
	_, err := os.Stat(filepath.Dir(confirm.Endpoint(goos)))
	return err == nil
}

func loadFakeConfig(t *testing.T, f *fakeSys) *config.Config {
	t.Helper()
	c, err := sys.loadConfig(f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Without the keys or the pairing record the host gets no session manager.
func TestNewSessionsNeedsPairing(t *testing.T) {
	f := newFake(t)
	c := loadFakeConfig(t, f)
	st := state.Open(f.state, state.Plain{})
	log := sys.log(io.Discard)
	if w, err := newSessions(c, st, log, sys.doctorEnv(c, st)); !errors.Is(err, state.ErrNotPaired) || w != nil {
		t.Fatalf("without keys: %v %v, want ErrNotPaired", w, err)
	}
	writeKeys(t, st)
	if w, err := newSessions(c, st, log, sys.doctorEnv(c, st)); !errors.Is(err, state.ErrNotPaired) || w != nil {
		t.Fatalf("without a pairing record: %v %v, want ErrNotPaired", w, err)
	}
}

// A paired host gets a session manager, without a confirmation server unless the configuration
// requires the local confirmation; bind sets the control client.
func TestNewSessionsPaired(t *testing.T) {
	f := newFake(t)
	c := loadFakeConfig(t, f)
	st := state.Open(f.state, state.Plain{})
	writeKeys(t, st)
	writePairing(t, st, "https://chat.example.com")
	w, err := newSessions(c, st, sys.log(io.Discard), sys.doctorEnv(c, st))
	if err != nil {
		t.Fatal(err)
	}
	if w.mgr == nil || w.gate == nil || w.confirm != nil {
		t.Fatalf("wiring %+v, want a manager and a gate without a confirmation server", w)
	}
	cl := &control.Client{}
	w.bind(cl)
	if w.cl != cl {
		t.Fatal("bind did not set the control client")
	}
}

// A host that requires the local confirmation gets no session manager when the confirmation
// endpoint cannot open, so no session opens without the confirmation.
func TestNewSessionsConfirmEndpointUnavailable(t *testing.T) {
	f := newFake(t)
	if confirmEndpointAvailable(sys.goos) {
		t.Skip("this machine has the confirmation endpoint folder")
	}
	c := loadFakeConfig(t, f)
	c.RequireLocalConfirm = true
	st := state.Open(f.state, state.Plain{})
	writeKeys(t, st)
	writePairing(t, st, "https://chat.example.com")
	if w, err := newSessions(c, st, sys.log(io.Discard), sys.doctorEnv(c, st)); err == nil || w != nil {
		t.Fatalf("newSessions = %v, %v; want an error without the endpoint", w, err)
	}
}

// localServer is a loopback TCP listener that closes every connection, the server URL of a paired
// host in a test: the control connection fails at once and nothing leaves this machine. The
// channel receives a value for every connection.
func localServer(t *testing.T) (string, <-chan struct{}) {
	t.Helper()
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	dialed := make(chan struct{}, 1)
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
			select {
			case dialed <- struct{}{}:
			default:
			}
		}
	}()
	return "https://" + l.Addr().String(), dialed
}
