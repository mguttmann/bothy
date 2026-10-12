// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance/domain"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

const (
	interval = 30 * 24 * time.Hour
	lead     = time.Hour
	window   = 90 * time.Second
)

func key(kid string, state domain.KeyState, created time.Time, notAfter ...time.Time) domain.SigningKey {
	k := domain.SigningKey{KID: kid, State: state, CreatedAt: created}
	if len(notAfter) > 0 {
		k.NotAfter = notAfter[0]
	}
	return k
}

func TestSigningKeyTransitions_Due(t *testing.T) {
	t.Parallel()
	scheduled := domain.RotationPolicy{Interval: interval, Lead: lead, Window: window}
	onRequest := domain.RotationPolicy{Lead: lead, Window: window}
	active := key("a", domain.KeyActive, t0)
	for _, c := range []struct {
		name   string
		keys   []domain.SigningKey
		policy domain.RotationPolicy
		at     time.Time
		want   domain.Plan
	}{
		{name: "nothing due", keys: []domain.SigningKey{active}, policy: scheduled, at: t0.Add(interval - time.Nanosecond)},
		{name: "create next exactly at the interval", keys: []domain.SigningKey{active}, policy: scheduled, at: t0.Add(interval),
			want: domain.Plan{CreateNext: true}},
		{name: "create next after the interval", keys: []domain.SigningKey{active}, policy: scheduled, at: t0.Add(2 * interval),
			want: domain.Plan{CreateNext: true}},
		{name: "no scheduled rotation without interval", keys: []domain.SigningKey{active}, policy: onRequest, at: t0.Add(100 * interval)},
		{name: "at most one next", keys: []domain.SigningKey{active, key("n", domain.KeyNext, t0.Add(interval))},
			policy: scheduled, at: t0.Add(interval + lead - time.Nanosecond)},
		{name: "no next without active", keys: nil, policy: scheduled, at: t0.Add(interval)},
		{name: "promote exactly after the lead", keys: []domain.SigningKey{active, key("n", domain.KeyNext, t0.Add(interval))},
			policy: scheduled, at: t0.Add(interval + lead),
			want: domain.Plan{Transitions: []domain.Transition{
				{KID: "a", From: domain.KeyActive, To: domain.KeyRetiring, NotAfter: t0.Add(interval + lead)},
				{KID: "n", From: domain.KeyNext, To: domain.KeyActive},
			}}},
		{name: "promote an on-request next", keys: []domain.SigningKey{active, key("n", domain.KeyNext, t0)},
			policy: onRequest, at: t0.Add(lead),
			want: domain.Plan{Transitions: []domain.Transition{
				{KID: "a", From: domain.KeyActive, To: domain.KeyRetiring, NotAfter: t0.Add(lead)},
				{KID: "n", From: domain.KeyNext, To: domain.KeyActive},
			}}},
		{name: "promote without an active key", keys: []domain.SigningKey{key("n", domain.KeyNext, t0)},
			policy: scheduled, at: t0.Add(lead),
			want: domain.Plan{Transitions: []domain.Transition{{KID: "n", From: domain.KeyNext, To: domain.KeyActive}}}},
		{name: "retiring stays within the window", keys: []domain.SigningKey{active, key("r", domain.KeyRetiring, t0.Add(-interval), t0)},
			policy: scheduled, at: t0.Add(window - time.Nanosecond)},
		{name: "retire exactly at the end of the window", keys: []domain.SigningKey{active, key("r", domain.KeyRetiring, t0.Add(-interval), t0)},
			policy: scheduled, at: t0.Add(window),
			want: domain.Plan{Transitions: []domain.Transition{{KID: "r", From: domain.KeyRetiring, To: domain.KeyRetired}}}},
		{name: "zero window retires at once", keys: []domain.SigningKey{active, key("r", domain.KeyRetiring, t0.Add(-interval), t0)},
			policy: domain.RotationPolicy{Lead: lead}, at: t0,
			want: domain.Plan{Transitions: []domain.Transition{{KID: "r", From: domain.KeyRetiring, To: domain.KeyRetired}}}},
		{name: "retired keys are ignored", keys: []domain.SigningKey{active, key("x", domain.KeyRetired, t0.Add(-interval), t0)},
			policy: scheduled, at: t0.Add(time.Hour)},
		{name: "retire, promote and announce in one run", keys: []domain.SigningKey{
			key("r", domain.KeyRetiring, t0.Add(-2*interval), t0.Add(-interval)),
			key("a", domain.KeyActive, t0.Add(-interval)),
			key("n", domain.KeyNext, t0.Add(-interval-lead)),
		}, policy: domain.RotationPolicy{Interval: interval / 2, Lead: lead, Window: window}, at: t0,
			want: domain.Plan{CreateNext: true, Transitions: []domain.Transition{
				{KID: "r", From: domain.KeyRetiring, To: domain.KeyRetired},
				{KID: "a", From: domain.KeyActive, To: domain.KeyRetiring, NotAfter: t0},
				{KID: "n", From: domain.KeyNext, To: domain.KeyActive},
			}}},
	} {
		got := domain.DueTransitions(c.keys, c.policy, c.at)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: DueTransitions = %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestSigningKeyTransitions_Emergency(t *testing.T) {
	t.Parallel()
	at := t0.Add(time.Hour)
	keys := []domain.SigningKey{
		key("old", domain.KeyRetired, t0.Add(-3*interval), t0.Add(-2*interval)),
		key("r", domain.KeyRetiring, t0.Add(-interval), t0.Add(-time.Minute)),
		key("a", domain.KeyActive, t0),
		key("n", domain.KeyNext, t0.Add(time.Minute)),
	}
	got := domain.EmergencyTransitions(keys, at)
	want := []domain.Transition{
		{KID: "r", From: domain.KeyRetiring, To: domain.KeyRetired},
		{KID: "a", From: domain.KeyActive, To: domain.KeyRetired, NotAfter: at},
		{KID: "n", From: domain.KeyNext, To: domain.KeyRetired, NotAfter: at},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EmergencyTransitions = %+v, want %+v", got, want)
	}
	if got := domain.EmergencyTransitions(nil, at); len(got) != 0 {
		t.Errorf("EmergencyTransitions of no keys = %+v", got)
	}
}

func TestSigningKeyTransitions_ManualScheduled(t *testing.T) {
	t.Parallel()
	active := key("a", domain.KeyActive, t0)
	if _, ok := domain.PendingNext([]domain.SigningKey{active}); ok {
		t.Error("PendingNext found a next key where none exists")
	}
	n := key("n", domain.KeyNext, t0.Add(time.Minute))
	got, ok := domain.PendingNext([]domain.SigningKey{active, n})
	if !ok || got.KID != "n" {
		t.Errorf("PendingNext = %+v, %v; want the next key", got, ok)
	}
	if a, ok := domain.ActiveKey([]domain.SigningKey{n, active}); !ok || a.KID != "a" {
		t.Errorf("ActiveKey = %+v, %v", a, ok)
	}
	if _, ok := domain.ActiveKey([]domain.SigningKey{n}); ok {
		t.Error("ActiveKey found an active key where none exists")
	}
}

func TestSigningKeyTransitions_Window(t *testing.T) {
	t.Parallel()
	if got := domain.Window(10*time.Minute, 30*time.Second); got != 10*time.Minute+30*time.Second {
		t.Errorf("Window = %v", got)
	}
}

func TestSigningKeyTransitions_StateValid(t *testing.T) {
	t.Parallel()
	for _, s := range []domain.KeyState{domain.KeyNext, domain.KeyActive, domain.KeyRetiring, domain.KeyRetired} {
		if !s.Valid() {
			t.Errorf("%s is not valid", s)
		}
	}
	if domain.KeyState("expired").Valid() {
		t.Error("an unknown state is valid")
	}
	if !domain.KeyRetiring.Published() || !domain.KeyNext.Published() || !domain.KeyActive.Published() || domain.KeyRetired.Published() {
		t.Error("Published must be true for next, active and retiring only")
	}
}

func TestSigningKeyTransitions_LostFiles(t *testing.T) {
	t.Parallel()
	at := t0.Add(time.Hour)
	keys := []domain.SigningKey{
		key("r", domain.KeyRetiring, t0.Add(-interval), t0),
		key("a", domain.KeyActive, t0),
		key("n", domain.KeyNext, t0.Add(time.Minute)),
	}
	for _, c := range []struct {
		name       string
		lost       []string
		want       []domain.Transition
		needActive bool
	}{
		{name: "nothing lost"},
		{name: "active lost", lost: []string{"a"}, needActive: true,
			want: []domain.Transition{{KID: "a", From: domain.KeyActive, To: domain.KeyRetired, NotAfter: at}}},
		{name: "next and retiring lost", lost: []string{"n", "r"},
			want: []domain.Transition{
				{KID: "r", From: domain.KeyRetiring, To: domain.KeyRetired},
				{KID: "n", From: domain.KeyNext, To: domain.KeyRetired, NotAfter: at},
			}},
		{name: "unknown kid", lost: []string{"x"}},
	} {
		lost := map[string]bool{}
		for _, k := range c.lost {
			lost[k] = true
		}
		got, need := domain.LostTransitions(keys, lost, at)
		if !reflect.DeepEqual(got, c.want) || need != c.needActive {
			t.Errorf("%s: LostTransitions = %+v, %v; want %+v, %v", c.name, got, need, c.want, c.needActive)
		}
	}
	if _, need := domain.LostTransitions(nil, nil, at); !need {
		t.Error("a purpose without keys needs an active key")
	}
}
