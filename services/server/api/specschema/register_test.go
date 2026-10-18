// SPDX-License-Identifier: Apache-2.0

package specschema_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/api/specschema"
)

// fixtureRegister is a register that matches the real contracts: identify registered and enabled,
// every other client op of asyncapi.yaml later, and the event type typing.started.
func fixtureRegister(t *testing.T, c *specschema.Contract) specschema.Register {
	t.Helper()
	reg := specschema.Register{
		Ops:     []specschema.RegisteredOp{{Op: "identify", Action: "gateway.connect", RateLimitClass: "rl.gateway_connect"}},
		Events:  []specschema.RegisteredEvent{{Type: "typing.started", Kind: "ephemeral", ReceiveAction: "thread.read"}},
		Enabled: []string{"identify"},
	}
	for _, o := range c.Operations() {
		if !o.Send && o.Op != "identify" {
			reg.Later = append(reg.Later, specschema.LaterOp{Op: o.Op, Milestone: "M2"})
		}
	}
	if p := specschema.Compare(reg, c.Catalogue(), c.AsyncAPI()); len(p) != 0 {
		t.Fatalf("the fixture register itself differs: %v", p)
	}
	return reg
}

func TestCompare_UnlistedEventTypeFails(t *testing.T) {
	t.Parallel()
	c := contract(t)
	reg := fixtureRegister(t, c)
	reg.Events = append(reg.Events, specschema.RegisteredEvent{Type: "test.unlisted", Kind: "durable", ReceiveAction: "thread.read"})
	got := specschema.Compare(reg, c.Catalogue(), c.AsyncAPI())
	if len(got) != 2 {
		t.Fatalf("%d problems %v, want missing in actions.yaml and missing in asyncapi.yaml", len(got), got)
	}
	for _, p := range got {
		if p.Subject != "event test.unlisted" {
			t.Errorf("problem %v", p)
		}
	}
}

func TestCompare_WrongOpActionFails(t *testing.T) {
	t.Parallel()
	c := contract(t)
	reg := fixtureRegister(t, c)
	reg.Ops[0].Action = "typing.start"
	got := specschema.Compare(reg, c.Catalogue(), c.AsyncAPI())
	if len(got) != 1 || got[0].Subject != "op identify" {
		t.Fatalf("problems %v, want one about op identify", got)
	}
}

func TestCompare_EnableList(t *testing.T) {
	t.Parallel()
	c := contract(t)
	reg := fixtureRegister(t, c)
	// A later op that is registered, and an enabled op that is not.
	reg.Ops = append(reg.Ops, specschema.RegisteredOp{Op: "heartbeat", Action: "gateway.connect", RateLimitClass: "rl.gateway_connect"})
	reg.Enabled = append(reg.Enabled, "focus")
	got := specschema.Compare(reg, c.Catalogue(), c.AsyncAPI())
	var texts []string
	for _, p := range got {
		texts = append(texts, p.String())
	}
	for _, want := range []string{
		"op focus: in gateway_ops.enabled, but not registered",
		"op focus: in gateway_ops.enabled and gateway_ops.later",
		"op heartbeat: in gateway_ops.later, but registered",
		"op heartbeat: registered, but not in gateway_ops.enabled",
	} {
		if !slices.Contains(texts, want) {
			t.Errorf("missing %q in %v", want, texts)
		}
	}
}

func TestSchemaName(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"read_state.updated": "ReadStateUpdatedEvent", "thread.moved": "ThreadMovedEvent", "typing.started": "TypingStartedEvent"} {
		if got := specschema.SchemaName(in); got != want {
			t.Errorf("%s: %s, want %s", in, got, want)
		}
	}
}
