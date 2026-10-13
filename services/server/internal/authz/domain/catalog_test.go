// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
)

func TestLookupAction_Known(t *testing.T) {
	a, ok := domain.LookupAction("message.read")
	if !ok || a.ID != "message.read" || !slices.Equal(a.Resources, []string{"message"}) || a.Right != "read" ||
		a.RateLimitClass != "rl.read" || a.System || a.Effect != domain.EffectRead {
		t.Fatalf("message.read = %+v, %v", a, ok)
	}
	b, ok := domain.LookupAction("instance.bootstrap")
	if !ok || !b.System || b.RateLimitClass != "" || b.Right != "system" {
		t.Fatalf("instance.bootstrap = %+v, %v", b, ok)
	}
	if p, _ := domain.LookupAction("access.preview"); !p.OwnerAsTarget || p.Owner || len(p.Resources) != 0 {
		t.Fatalf("access.preview = %+v", p)
	}
}

func TestLookupAction_Unknown(t *testing.T) {
	if a, ok := domain.LookupAction("no.such.action"); ok || a.ID != "" {
		t.Fatalf("unknown action found: %+v", a)
	}
}

// TestLookupAction_ReturnsCopy: a caller that changes the returned slices does not change the
// catalogue.
func TestLookupAction_ReturnsCopy(t *testing.T) {
	a, _ := domain.LookupAction("message.read")
	a.Resources[0] = "changed"
	if b, _ := domain.LookupAction("message.read"); b.Resources[0] != "message" {
		t.Fatal("the catalogue shares its slices with callers")
	}
	e, _ := domain.LookupEvent("message.created")
	e.MentionedActions[0] = "changed"
	o, _ := domain.LookupOp("heartbeat")
	o.MentionedActions[0] = "changed"
	if e2, _ := domain.LookupEvent("message.created"); e2.MentionedActions[0] != "message.read" {
		t.Fatal("event slices shared")
	}
	if o2, _ := domain.LookupOp("heartbeat"); o2.MentionedActions[0] != "agent.heartbeat" {
		t.Fatal("op slices shared")
	}
	ids := domain.ActionIDs()
	ids[0] = "changed"
	if domain.ActionIDs()[0] == "changed" {
		t.Fatal("ActionIDs shares its slice")
	}
}

func TestEventSpecs_DurableAndEphemeral(t *testing.T) {
	m, ok := domain.LookupEvent("message.created")
	if !ok || !m.Durable || m.Resource != "message" || !slices.Equal(m.MentionedActions, []string{"message.read"}) || !m.ResourceVersion {
		t.Fatalf("message.created = %+v, %v", m, ok)
	}
	ty, ok := domain.LookupEvent("typing.started")
	if !ok || ty.Durable {
		t.Fatalf("typing.started = %+v, %v", ty, ok)
	}
	if a, ok := domain.LookupEvent("access.changed"); !ok || !a.AudienceSelf || a.ReceiveAction != "self" {
		t.Fatalf("access.changed = %+v, %v", a, ok)
	}
	if _, ok := domain.LookupEvent("no.such.event"); ok {
		t.Fatal("unknown event found")
	}
}

func TestOpSpecs_MentionedActions(t *testing.T) {
	h, ok := domain.LookupOp("heartbeat")
	if !ok || !slices.Equal(h.MentionedActions, []string{"agent.heartbeat", "gateway.connect"}) || !h.Inbound() {
		t.Fatalf("heartbeat = %+v, %v", h, ok)
	}
	if hello, ok := domain.LookupOp("hello"); !ok || hello.Inbound() || hello.Direction != domain.DirectionServerToClient {
		t.Fatalf("hello = %+v, %v", hello, ok)
	}
	if d, _ := domain.LookupOp("dispatch"); !d.PerEventReceiveAction {
		t.Fatalf("dispatch = %+v", d)
	}
	if _, ok := domain.LookupOp("no_such_op"); ok {
		t.Fatal("unknown op found")
	}
}

func TestChannelOpSpecs(t *testing.T) {
	c, ok := domain.LookupChannelOp("host_control", "session.open")
	if !ok || c.Action != "host.connect" || c.Direction != "server_to_host" {
		t.Fatalf("host_control/session.open = %+v, %v", c, ok)
	}
	if _, ok := domain.LookupChannelOp("terminal_stream", "noise_frame"); !ok {
		t.Fatal("terminal_stream/noise_frame missing")
	}
	// A gateway op is no channel op and the reverse.
	if _, ok := domain.LookupChannelOp("", "hello"); ok {
		t.Fatal("gateway op hello found as channel op")
	}
	if _, ok := domain.LookupOp("host_control/hello"); ok {
		t.Fatal("channel op found as gateway op")
	}
	if _, ok := domain.LookupChannelOp("host_control/hello", ""); ok {
		t.Fatal("split key accepted")
	}
}

// TestLists: the list functions return sorted keys that every lookup finds.
func TestLists(t *testing.T) {
	lists := map[string][]string{
		"actions": domain.ActionIDs(), "events": domain.EventTypes(), "ops": domain.OpNames(),
		"channel ops": domain.ChannelOpIDs(), "resource kinds": domain.ResourceKinds(),
	}
	for name, l := range lists {
		if len(l) == 0 || !slices.IsSorted(l) {
			t.Errorf("%s: empty or unsorted", name)
		}
	}
	for _, id := range domain.ActionIDs() {
		if _, ok := domain.LookupAction(id); !ok {
			t.Errorf("action %s not found", id)
		}
	}
	for _, ty := range domain.EventTypes() {
		if _, ok := domain.LookupEvent(ty); !ok {
			t.Errorf("event %s not found", ty)
		}
	}
	for _, op := range domain.OpNames() {
		if _, ok := domain.LookupOp(op); !ok {
			t.Errorf("op %s not found", op)
		}
	}
	for _, id := range domain.ChannelOpIDs() {
		c, ok := domain.LookupChannelOpID(id)
		if !ok || c.Channel+"/"+c.Op != id {
			t.Errorf("channel op %s not found", id)
		}
	}
}

// TestEventSpecs_Note: the column "Note" or "Additional condition" of 05 section 6 is kept.
func TestEventSpecs_Note(t *testing.T) {
	e, ok := domain.LookupEvent("agent.invocation")
	if !ok || !strings.Contains(e.Note, "agent.invoke") {
		t.Fatalf("agent.invocation = %+v", e)
	}
}
