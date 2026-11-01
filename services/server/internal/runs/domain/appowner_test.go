// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestAppCommandTargetThread (M2-25 design note 5 b): the given thread, else the main thread of a
// text channel, else none in a forum; never an auto thread.
func TestAppCommandTargetThread(t *testing.T) {
	t.Parallel()
	given, main := ids.New(), ids.New()
	if got := domain.AppCommandTarget(given, "text", main); got != given {
		t.Errorf("given thread: %v", got)
	}
	if got := domain.AppCommandTarget(given, "forum", ids.ID{}); got != given {
		t.Errorf("given thread in a forum: %v", got)
	}
	if got := domain.AppCommandTarget(ids.ID{}, "text", main); got != main {
		t.Errorf("text channel: %v", got)
	}
	if got := domain.AppCommandTarget(ids.ID{}, "forum", ids.ID{}); !got.IsZero() {
		t.Errorf("forum without thread: %v", got)
	}
}

// TestInteractionStateAfterOutcome (M2-25 design note 8): only from dispatched; answered gives
// acknowledged, deferred keeps dispatched, every failure gives failed; any other state stays.
func TestInteractionStateAfterOutcome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, from string
		o          domain.AppOutcomeFacts
		to         string
		change     bool
	}{
		{"answered", "dispatched", domain.AppOutcomeFacts{Answered: true}, "acknowledged", true},
		{"deferred", "dispatched", domain.AppOutcomeFacts{Deferred: true}, "dispatched", false},
		{"failure", "dispatched", domain.AppOutcomeFacts{}, "failed", true},
		{"received", "received", domain.AppOutcomeFacts{Answered: true}, "received", false},
		{"acknowledged by the app", "acknowledged", domain.AppOutcomeFacts{}, "acknowledged", false},
		{"rejected by the app", "rejected", domain.AppOutcomeFacts{Answered: true}, "rejected", false},
		{"failed", "failed", domain.AppOutcomeFacts{Answered: true}, "failed", false},
	}
	for _, c := range cases {
		to, change := domain.InteractionStateAfterOutcome(c.from, c.o)
		if to != c.to || change != c.change {
			t.Errorf("%s: %s %v, want %s %v", c.name, to, change, c.to, c.change)
		}
	}
}

// TestAppCommandPayload (M2-25 design note 13): the payload of an app command validates against
// CommandInvokedPayload, has no run_id and no null, keeps control as stored and leaves thread_id
// out without a target thread.
func TestAppCommandPayload(t *testing.T) {
	t.Parallel()
	in := domain.AppCommandInvokedInput{
		InvocationID: ids.New(), CommandID: ids.New(), CommandName: "ticket", ChannelID: ids.New(), ThreadID: ids.New(), InvokedBy: refOf(),
		Options: []domain.OptionValue{{Name: "title", Value: "Printer"}},
	}
	for _, control := range []bool{false, true} {
		in.Control = control
		p, err := domain.AppCommandInvokedPayload(in)
		if err != nil {
			t.Fatal(err)
		}
		validateAgainst(t, "CommandInvokedPayload", p)
		var m map[string]any
		if err := json.Unmarshal(p, &m); err != nil {
			t.Fatal(err)
		}
		if _, ok := m["run_id"]; ok || m["control"] != control || testkit.HasJSONNull(p) {
			t.Fatalf("payload %s", p)
		}
	}
	in.ThreadID = ids.ID{}
	p, err := domain.AppCommandInvokedPayload(in)
	if err != nil {
		t.Fatal(err)
	}
	validateAgainst(t, "CommandInvokedPayload", p)
	if strings.Contains(string(p), "thread_id") {
		t.Fatalf("payload without target thread %s", p)
	}
	in.CommandName = ""
	if _, err := domain.AppCommandInvokedPayload(in); err == nil {
		t.Fatal("empty name accepted")
	}
}
