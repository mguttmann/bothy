// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	terminalhttp "github.com/OWNER/bothy/services/server/internal/terminal/adapters/httpapi"
	terminalapp "github.com/OWNER/bothy/services/server/internal/terminal/app"
)

// TestWire_HostSource (M2-39): the production wiring binds the terminal implementation as
// authz.HostSource for the Authorizer, the rights hook and the grant use cases; the presence of
// the host view is NoConnections until M2-43; the start fills the host handlers; terminal is
// one of the module registrations (host.updated).
func TestWire_HostSource(t *testing.T) {
	if _, ok := NewHostSource(nil).(terminalapp.HostSource); !ok {
		t.Fatal("NewHostSource does not return the HostSource of terminal")
	}
	src := RightsSources{}
	if src.Hosts != nil {
		t.Fatal("the zero RightsSources names a host source")
	}
	var p terminalapp.Presence = terminalapp.NoConnections{}
	if p.Connected(context.Background(), ids.New()) {
		t.Fatal("NoConnections reports a connection")
	}
	hosts := &terminalhttp.Hosts{}
	s := &server{terminalState: terminalState{terminal: &Terminal{Handlers: hosts}}}
	if h := s.handlers(); h.Hosts != hosts {
		t.Fatal("the start does not fill the host handlers")
	}
	if len(moduleRegistrations) == 0 || !registersHostUpdated() {
		t.Fatal("terminal does not register host.updated")
	}
}

// registersHostUpdated reports whether the module registrations declare host.updated of terminal.
func registersHostUpdated() bool {
	r := authorize.NewRegistry()
	for _, reg := range moduleRegistrations {
		reg(r)
	}
	for _, e := range r.Events() {
		if e.Module == "terminal" && e.Type == authorize.EventHostUpdated {
			return true
		}
	}
	return false
}
