// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestCancelDecision covers every state (brief M2-09 notes 7b to 7d).
func TestCancelDecision(t *testing.T) {
	t.Parallel()
	want := map[domain.State]domain.CancelOutcome{
		domain.StateQueued:       domain.CancelImmediate,
		domain.StateRunning:      domain.CancelRequest,
		domain.StateWaitingInput: domain.CancelRequest,
		domain.StateCancelled:    domain.CancelNoChange,
		domain.StateCompleted:    domain.CancelInvalid,
		domain.StateFailed:       domain.CancelInvalid,
	}
	for _, st := range domain.States() {
		if got := domain.CancelDecision(st); got != want[st] {
			t.Errorf("CancelDecision(%s) = %d, want %d", st, got, want[st])
		}
	}
	if got := domain.CancelDecision("unknown"); got != domain.CancelInvalid {
		t.Errorf("CancelDecision(unknown) = %d, want CancelInvalid", got)
	}
}

// TestCancelReasonFor covers the action rule of brief M2-09 note 8c.
func TestCancelReasonFor(t *testing.T) {
	t.Parallel()
	for action, want := range map[string]string{
		"user.disable":        domain.RunReasonUserDisabled,
		"channel.archive":     domain.RunReasonChannelReadOnly,
		"space.archive":       domain.RunReasonChannelReadOnly,
		"grant.revoke":        domain.RunReasonAccessRevoked,
		"grant.expire":        domain.RunReasonAccessRevoked,
		"group.member.remove": domain.RunReasonAccessRevoked,
		"owner.remove":        domain.RunReasonAccessRevoked,
		"":                    domain.RunReasonAccessRevoked,
	} {
		if got := domain.CancelReasonFor(action); got != want {
			t.Errorf("CancelReasonFor(%q) = %q, want %q", action, got, want)
		}
	}
}

// TestLostTriggerRuns covers the diff rule of brief M2-09 note 8a.
func TestLostTriggerRuns(t *testing.T) {
	t.Parallel()
	alex, sam := ids.New(), ids.New()
	support, marketing := ids.New(), ids.New()
	allgemein, second, texte := ids.New(), ids.New(), ids.New()
	at := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	run := func(by, channel, space ids.ID, st domain.State) domain.LiveRun {
		return domain.LiveRun{ID: ids.New(), AgentID: ids.New(), TriggeredBy: by, ThreadID: ids.New(), ChannelID: channel, SpaceID: space,
			State: st, QueuedAt: at}
	}
	r1 := run(alex, allgemein, support, domain.StateRunning)
	r2 := run(alex, second, support, domain.StateQueued)
	r3 := run(alex, texte, marketing, domain.StateWaitingInput)
	r4 := run(sam, allgemein, support, domain.StateRunning)
	ended := run(alex, allgemein, support, domain.StateCompleted)
	runs := []domain.LiveRun{r1, r2, r3, r4, ended}
	change := func(p ids.ID, removed, changed []domain.AccessTarget) []domain.AccessChange {
		return []domain.AccessChange{{PrincipalID: p, Removed: removed, Changed: changed}}
	}
	rows := []struct {
		name    string
		changes []domain.AccessChange
		want    []ids.ID
	}{
		{"removed channel", change(alex, []domain.AccessTarget{{Kind: "channel", ID: allgemein}}, nil), []ids.ID{r1.ID}},
		{"level changed to read", change(alex, nil, []domain.AccessTarget{{Kind: "channel", ID: allgemein, Level: "read"}}), []ids.ID{r1.ID}},
		{"level changed to write", change(alex, nil, []domain.AccessTarget{{Kind: "channel", ID: second, Level: "write"}}), []ids.ID{r2.ID}},
		{"level absent", change(alex, nil, []domain.AccessTarget{{Kind: "channel", ID: allgemein}}), []ids.ID{r1.ID}},
		{"capability-only change", change(alex, nil, []domain.AccessTarget{{Kind: "channel", ID: allgemein, Level: "trigger"}}), nil},
		{"change on another channel", change(alex, []domain.AccessTarget{{Kind: "channel", ID: ids.New()}}, nil), nil},
		{"removed space", change(alex, []domain.AccessTarget{{Kind: "space", ID: support}}, nil), []ids.ID{r1.ID, r2.ID}},
		{"changed space is no loss of a channel", change(alex, nil, []domain.AccessTarget{{Kind: "space", ID: support, Level: "read"}}), nil},
		{"host target", change(alex, []domain.AccessTarget{{Kind: "host", ID: allgemein}}, nil), nil},
		{"another principal", change(sam, []domain.AccessTarget{{Kind: "channel", ID: texte}}, nil), nil},
		{"another principal on the same channel", change(sam, []domain.AccessTarget{{Kind: "channel", ID: allgemein}}, nil), []ids.ID{r4.ID}},
		{"two principals in input order", append(change(sam, []domain.AccessTarget{{Kind: "space", ID: support}}, nil),
			change(alex, []domain.AccessTarget{{Kind: "space", ID: marketing}}, nil)...), []ids.ID{r3.ID, r4.ID}},
		{"no change", nil, nil},
	}
	for _, row := range rows {
		if got := domain.LostTriggerRuns(row.changes, runs); !slices.Equal(got, row.want) {
			t.Errorf("%s: got %v, want %v", row.name, got, row.want)
		}
	}
}
