// SPDX-License-Identifier: Apache-2.0

package authorize_test

import (
	"slices"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

func TestParse_KnownAndUnknown(t *testing.T) {
	if a, ok := authorize.ParseAction("message.read"); !ok || a != authorize.ActionMessageRead {
		t.Fatalf("ParseAction = %q, %v", a, ok)
	}
	if e, ok := authorize.ParseEventType("access.changed"); !ok || e != authorize.EventAccessChanged {
		t.Fatalf("ParseEventType = %q, %v", e, ok)
	}
	if o, ok := authorize.ParseOp("command.autocomplete_result"); !ok || o != authorize.OpCommandAutocompleteResult {
		t.Fatalf("ParseOp = %q, %v", o, ok)
	}
	if r, ok := authorize.ParseResourceKind("auth_session"); !ok || r != authorize.ResourceAuthSession {
		t.Fatalf("ParseResourceKind = %q, %v", r, ok)
	}
	if c, ok := authorize.ParseChannelOp("host_control/session.open"); !ok || c != authorize.ChannelOpHostControlSessionOpen {
		t.Fatalf("ParseChannelOp = %q, %v", c, ok)
	}
	if a, ok := authorize.ParseAction("agent.billing_mode.set_all"); !ok || a != authorize.ActionAgentBillingModeSetAll {
		t.Fatalf("ParseAction = %q, %v", a, ok)
	}
	for _, s := range []string{"", "no.such.action", "Message.Read", "message.read "} {
		if _, ok := authorize.ParseAction(s); ok {
			t.Errorf("ParseAction(%q) accepted", s)
		}
	}
	if _, ok := authorize.ParseOp("host_control/hello"); ok {
		t.Error("a channel op parsed as gateway op")
	}
	if _, ok := authorize.ParseChannelOp("hello"); ok {
		t.Error("a gateway op parsed as channel op")
	}
	if _, ok := authorize.ParseEventType("no.such.event"); ok {
		t.Error("unknown event type parsed")
	}
	if _, ok := authorize.ParseResourceKind("wie die Zielaktion"); ok {
		t.Error("05 text parsed as resource kind")
	}
}

func TestAll_SortedFreshSlices(t *testing.T) {
	a := authorize.AllActions()
	if len(a) == 0 || !slices.IsSorted(a) {
		t.Fatal("AllActions empty or unsorted")
	}
	a[0] = authorize.ActionMessageRead
	if authorize.AllActions()[0] == authorize.ActionMessageRead {
		t.Fatal("AllActions shares its slice")
	}
	if !slices.IsSorted(authorize.AllEventTypes()) || !slices.IsSorted(authorize.AllOps()) ||
		!slices.IsSorted(authorize.AllResourceKinds()) || !slices.IsSorted(authorize.AllChannelOps()) {
		t.Fatal("an All function is unsorted")
	}
	for _, x := range authorize.AllOps() {
		if y, ok := authorize.ParseOp(string(x)); !ok || y != x {
			t.Errorf("op %q does not parse to itself", x)
		}
	}
}

func TestRegistry_RecordsSortedAndIdempotent(t *testing.T) {
	r := authorize.NewRegistry()
	r.Event("messaging", authorize.EventMessageCreated)
	r.Event("authz", authorize.EventAccessChanged)
	r.Event("authz", authorize.EventAccessChanged)
	r.Event("chat", authorize.EventAccessChanged)
	r.Op("gateway", authorize.OpTypingStart)
	r.Op("gateway", authorize.OpTypingStart)
	r.Op("gateway", authorize.OpHeartbeat)
	r.ChannelOp("terminal", authorize.ChannelOpHostControlSessionOpen)
	r.ChannelOp("terminal", authorize.ChannelOpHostControlSessionOpen)

	wantEvents := []authorize.EventRegistration{
		{Module: "authz", Type: authorize.EventAccessChanged},
		{Module: "chat", Type: authorize.EventAccessChanged},
		{Module: "messaging", Type: authorize.EventMessageCreated},
	}
	if got := r.Events(); !slices.Equal(got, wantEvents) {
		t.Errorf("Events() = %v", got)
	}
	wantOps := []authorize.OpRegistration{
		{Module: "gateway", Op: authorize.OpHeartbeat},
		{Module: "gateway", Op: authorize.OpTypingStart},
	}
	if got := r.Ops(); !slices.Equal(got, wantOps) {
		t.Errorf("Ops() = %v", got)
	}
	if got := r.ChannelOps(); len(got) != 1 || got[0].Op != authorize.ChannelOpHostControlSessionOpen {
		t.Errorf("ChannelOps() = %v", got)
	}
	ev := r.Events()
	ev[0].Module = "changed"
	if r.Events()[0].Module != "authz" {
		t.Error("Events shares its slice")
	}
}

func TestRegistry_ConcurrentUse(t *testing.T) {
	r := authorize.NewRegistry()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			r.Event("messaging", authorize.EventMessageCreated)
			r.Op("gateway", authorize.OpTypingStart)
			r.ChannelOp("terminal", authorize.ChannelOpHostDataHello)
			_ = r.Events()
			_ = r.Ops()
			_ = r.ChannelOps()
		})
	}
	wg.Wait()
	if len(r.Events()) != 1 || len(r.Ops()) != 1 || len(r.ChannelOps()) != 1 {
		t.Fatal("concurrent registrations not idempotent")
	}
}
