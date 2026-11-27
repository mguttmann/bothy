// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// accountsUsers knows the accounts it holds by name; every other name is unknown.
type accountsUsers map[string]hostos.Account

func (u accountsUsers) Lookup(name string) (hostos.Account, error) {
	if a, ok := u[name]; ok {
		return a, nil
	}
	return hostos.Account{}, hostos.ErrUnknownUser
}

func (accountsUsers) Groups(string) ([]hostos.Group, error) { return nil, errors.New("not used") }
func (accountsUsers) Accounts() ([]hostos.Account, error)   { return nil, errors.New("not used") }

// TestComputerSessionAccounts: the peer rule excludes the configured and the former session users
// (bindings.json) that exist, read at every call, and HD10 checks them and everyone as writers.
func TestComputerSessionAccounts(t *testing.T) {
	st := state.Open(t.TempDir(), state.Plain{})
	users := accountsUsers{
		"bothy-s-a":   {Name: "bothy-s-a", ID: "501"},
		"bothy-s-old": {Name: "bothy-s-old", ID: "502"},
	}
	u := sessionAccounts{c: &config.Config{SessionUsers: []string{"bothy-s-a", "bothy-s-gone"}}, st: st, users: users}
	if got := u.ids(); !slices.Equal(got, []string{"501"}) {
		t.Errorf("ids without bindings %v", got)
	}
	if err := st.WriteBindings(map[string]string{"bothy-s-old": "p"}); err != nil {
		t.Fatal(err)
	}
	got := u.ids()
	slices.Sort(got)
	if !slices.Equal(got, []string{"501", "502"}) {
		t.Errorf("ids with a former session user %v", got)
	}
	w := u.writers()
	if len(w) != 3 || w[len(w)-1].ID != "everyone" {
		t.Errorf("writers %+v", w)
	}
}

// TestComputerWiringOff: without host_service.computer_use or without pairing the host refuses
// computer sessions, a nil wiring binds and starts as a no-op, and doctor says off.
func TestComputerWiringOff(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	st := state.Open(t.TempDir(), state.Plain{})
	for _, c := range []struct {
		on, paired bool
	}{{false, true}, {true, false}} {
		got, cw := wireComputers(context.Background(), &config.Config{ComputerUse: c.on}, st, log, doctor.Env{}, c.paired)
		r, ok := got.(control.RefuseComputers)
		if !ok || cw != nil || r.Signature != buildSignature() {
			t.Errorf("on %v paired %v: %T %+v", c.on, c.paired, got, cw)
		}
	}
	// Paired with the key but without keys in the state directory: refused, nothing listens.
	if got, cw := wireComputers(context.Background(), &config.Config{ComputerUse: true}, st, log, doctor.Env{}, true); cw != nil {
		t.Errorf("without keys %T %+v", got, cw)
	}
	var cw *computerWiring
	cw.bind(nil)
	cw.start(context.Background())()
	if out := computerDoctor(&config.Config{}, doctor.Env{}, nil)(); out != "computer: off\n" {
		t.Errorf("doctor %q", out)
	}
	out := computerDoctor(&config.Config{ComputerUse: true}, doctor.Env{}, nil)()
	if !strings.Contains(out, "computer: on\n") || !strings.Contains(out, "computer helper: "+control.HelperAbsent+"\n") {
		t.Errorf("doctor without a helper %q", out)
	}
}

// TestComputerPeerRule: the peer rule of 27 9.4 carries the platform, the installed helper path
// and, on Linux only, the uid of host_service.computer_test_peer; the console rule excludes the
// service account.
func TestComputerPeerRule(t *testing.T) {
	old := sys
	t.Cleanup(func() { sys = old })
	cp := *old
	sys = &cp
	st := state.Open(t.TempDir(), state.Plain{})
	env := doctor.Env{ServiceUser: "bothy-host", Users: accountsUsers{
		"bothy-host": {Name: "bothy-host", ID: "400"}, "tester": {Name: "tester", ID: "1001"},
	}}
	c := &config.Config{ComputerTestPeer: "tester"}
	sys.goos = hostos.Linux
	if r := peerRule(c, st, env); r.GOOS != hostos.Linux || r.TestPeerUID != "1001" || r.Console.ServiceUID != "400" || r.Console.Excluded == nil || r.Trusted == nil {
		t.Errorf("linux rule %+v", r)
	}
	sys.goos = hostos.Darwin
	if r := peerRule(c, st, env); r.TestPeerUID != "" || r.HelperPath == "" {
		t.Errorf("macOS rule %+v", r)
	}
}
