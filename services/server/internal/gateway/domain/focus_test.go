// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bufio"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestFocusSet_DedupeAndLimit: NewFocusSet drops repeated ids and keeps the first order; more
// than max distinct ids is ErrTooManyFocused, exactly max is fine (04 section 11.3 item 2).
func TestFocusSet_DedupeAndLimit(t *testing.T) {
	t.Parallel()
	a, b, c := ids.New(), ids.New(), ids.New()
	got, err := domain.NewFocusSet([]ids.ID{a, b, a, b, a}, true, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Threads, []ids.ID{a, b}) || !got.Idle {
		t.Errorf("deduplicated %+v, want [%v %v] and idle", got, a, b)
	}
	if _, err := domain.NewFocusSet([]ids.ID{a, b, c}, false, 2); !errors.Is(err, domain.ErrTooManyFocused) {
		t.Errorf("three distinct ids over a limit of two: %v", err)
	}
	if got, err := domain.NewFocusSet(nil, false, 2); err != nil || len(got.Threads) != 0 {
		t.Errorf("empty set: %v %v", got, err)
	}
}

// TestTypingThrottle: the first start of a thread fans out; a second one within the interval does
// not, one after the interval does; Reset lets the next start fan out at once; threads are
// independent.
func TestTypingThrottle(t *testing.T) {
	t.Parallel()
	th := domain.NewTypingThrottle(5 * time.Second)
	t0 := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	a, b := ids.New(), ids.New()
	steps := []struct {
		name   string
		thread ids.ID
		at     time.Duration
		reset  bool
		want   bool
	}{
		{"first", a, 0, false, true},
		{"inside the interval", a, 4 * time.Second, false, false},
		{"other thread", b, 4 * time.Second, false, true},
		{"after the interval", a, 5 * time.Second, false, true},
		{"after stop", a, 6 * time.Second, true, true},
		{"again inside", a, 7 * time.Second, false, false},
	}
	for _, s := range steps {
		if s.reset {
			th.Reset(s.thread)
		}
		if got := th.Allow(s.thread, t0.Add(s.at)); got != s.want {
			t.Errorf("%s: %v, want %v", s.name, got, s.want)
		}
	}
}

// TestEphemeralTypes: the focus-bound types are typing.started and run.updated, each with the
// receive action of docs/spec/actions.yaml (05 "Ephemeral events"); an unknown type is not
// registered.
func TestEphemeralTypes(t *testing.T) {
	t.Parallel()
	want := ephemeralReceiveActions(t)
	for _, typ := range []string{"typing.started", "run.updated"} {
		et, ok := domain.LookupEphemeral(typ)
		if !ok || !et.FocusBound {
			t.Errorf("%s: %+v %v, want a focus-bound type", typ, et, ok)
			continue
		}
		if et.ReceiveAction != want[typ] {
			t.Errorf("%s receive action %q, the catalogue says %q", typ, et.ReceiveAction, want[typ])
		}
	}
	if et, _ := domain.LookupEphemeral("typing.started"); !et.ExcludeActor {
		t.Error("typing.started must not reach the typist's own connections (04 section 12 item 1)")
	}
	// host.presence_changed (M2-43): no focus, every live connection is a candidate.
	if et, ok := domain.LookupEphemeral("host.presence_changed"); !ok || et.FocusBound || et.ReceiveAction != want["host.presence_changed"] {
		t.Errorf("host.presence_changed: %+v %v, want an unbound type with receive action %q", et, ok, want["host.presence_changed"])
	}
	if _, ok := domain.LookupEphemeral("message.created"); ok {
		t.Error("a durable type is no ephemeral type")
	}
}

// actionsPath is the action catalogue, relative to this package.
const actionsPath = "../../../../../docs/spec/actions.yaml"

// ephemeralReceiveActions reads type and receive_action of the ephemeral entries of the event
// catalogue: a line scan, because the domain layer imports no YAML library.
func ephemeralReceiveActions(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open(actionsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	out := map[string]string{}
	typ, ephemeral := "", false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "- type: "):
			typ, ephemeral = strings.TrimPrefix(line, "- type: "), false
		case line == "kind: ephemeral":
			ephemeral = true
		case ephemeral && strings.HasPrefix(line, "receive_action: "):
			out[typ] = strings.TrimPrefix(line, "receive_action: ")
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("no ephemeral type in the catalogue")
	}
	return out
}
