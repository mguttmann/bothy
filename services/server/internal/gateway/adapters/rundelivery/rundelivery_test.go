// SPDX-License-Identifier: Apache-2.0

package rundelivery_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/rundelivery"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// delivery is a scripted runs.Delivery.
type delivery struct {
	out runs.DeliveryOutcome
	err error
	got []runs.DeliveryInput
}

func (d *delivery) Prepare(_ context.Context, in runs.DeliveryInput) (runs.DeliveryOutcome, error) {
	d.got = append(d.got, in)
	return d.out, d.err
}

// TestHook: an invocation-bearing envelope passes Prepare and carries the token on Send, is
// refused without Send and fails with the error; other envelopes pass unchanged.
func TestHook(t *testing.T) {
	t.Parallel()
	agent, run, seq := ids.New(), ids.New(), int64(41)
	ev := sync.Envelope{ID: ids.New(), Seq: &seq, Type: "agent.invocation", Resource: sync.EnvelopeResource{Kind: "run", ID: run},
		Payload: []byte(`{}`), CorrelationID: ids.New(), Redelivered: true}
	d := &delivery{out: runs.DeliveryOutcome{Send: true, InvocationToken: "a.b.c"}}
	h := rundelivery.Hook{Delivery: d}
	got, ok, err := h.Prepare(context.Background(), agent, ev)
	if err != nil || !ok || got.InvocationToken != "a.b.c" || len(d.got) != 1 {
		t.Fatalf("send: %+v %v %v", got, ok, err)
	}
	if in := d.got[0]; in.AgentID != agent || in.ResourceID != run || in.ResourceKind != "run" || in.Seq != seq || in.EventID != ev.ID ||
		in.CorrelationID != ev.CorrelationID || !in.Redelivered || in.Type != "agent.invocation" {
		t.Fatalf("input %+v", in)
	}
	d.out = runs.DeliveryOutcome{}
	if _, ok, err := h.Prepare(context.Background(), agent, ev); ok || err != nil {
		t.Fatalf("refused: %v %v", ok, err)
	}
	d.err = errors.New("boom")
	if _, ok, err := h.Prepare(context.Background(), agent, ev); ok || err == nil {
		t.Fatalf("error: %v %v", ok, err)
	}
	other := sync.Envelope{ID: ids.New(), Type: "run.cancel_requested"}
	if got, ok, err := h.Prepare(context.Background(), agent, other); err != nil || !ok || got.InvocationToken != "" || len(d.got) != 3 {
		t.Fatalf("other: %+v %v %v", got, ok, err)
	}
	if _, _, err := (rundelivery.Hook{}).Prepare(context.Background(), agent, ev); err == nil {
		t.Fatal("a hook without the port delivered")
	}
}
