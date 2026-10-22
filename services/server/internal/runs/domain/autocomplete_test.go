// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestAutocompleteTarget: the given thread; a text channel without thread gives the main thread; a
// forum without thread gives none (R-AZ-26).
func TestAutocompleteTarget(t *testing.T) {
	t.Parallel()
	given, main := ids.New(), ids.New()
	if got := domain.AutocompleteTarget(given, "text", main); got != given {
		t.Fatalf("given thread: %v", got)
	}
	if got := domain.AutocompleteTarget(given, "forum", ids.ID{}); got != given {
		t.Fatalf("given thread in a forum: %v", got)
	}
	if got := domain.AutocompleteTarget(ids.ID{}, "text", main); got != main {
		t.Fatalf("text channel: %v, want the main thread", got)
	}
	if got := domain.AutocompleteTarget(ids.ID{}, "forum", ids.ID{}); !got.IsZero() {
		t.Fatalf("forum without thread: %v, want none", got)
	}
}

// acDefs are the options of /ticket: ticket with autocomplete, priority with static choices.
func acDefs() []domain.OptionDef {
	return []domain.OptionDef{
		{Name: "ticket", Type: "string", Autocomplete: true},
		{Name: "priority", Type: "choice", Choices: []string{"high", "low"}},
		{Name: "team", Type: "string", Autocomplete: true},
	}
}

// fieldOf returns pointer and code of the one field error of a validation_failed err.
func fieldOf(t *testing.T, err error) (string, string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 {
		t.Fatalf("want validation_failed with one field error, got %v", err)
	}
	return e.Fields[0].Pointer, e.Fields[0].Code
}

// TestAutocompleteQueryValidation covers the query rules of design note 9.
func TestAutocompleteQueryValidation(t *testing.T) {
	t.Parallel()
	ov := func(pairs ...string) []domain.OptionValue {
		var out []domain.OptionValue
		for i := 0; i+1 < len(pairs); i += 2 {
			out = append(out, domain.OptionValue{Name: pairs[i], Value: pairs[i+1]})
		}
		return out
	}
	if err := domain.ValidateAutocompleteQuery(acDefs(), "ticket", ov("priority", "high", "team", "a")); err != nil {
		t.Fatalf("valid query: %v", err)
	}
	// Values are passed on without a type check: the person is still typing.
	if err := domain.ValidateAutocompleteQuery(acDefs(), "ticket", ov("priority", "hi")); err != nil {
		t.Fatalf("a partial value of another option: %v", err)
	}
	cases := []struct {
		name, option  string
		others        []domain.OptionValue
		pointer, code string
	}{
		{"unknown option", "nope", nil, "/option", "unknown_option"},
		{"option without autocomplete", "priority", nil, "/option", "autocomplete_not_enabled"},
		{"unknown name in options", "ticket", ov("priority", "high", "nope", "x"), "/options/1/name", "unknown_option"},
		{"a name twice", "ticket", ov("priority", "high", "priority", "low"), "/options/1/name", "duplicate"},
		{"the option repeated in options", "ticket", ov("ticket", "4"), "/options/0/name", "duplicate"},
	}
	for _, c := range cases {
		err := domain.ValidateAutocompleteQuery(acDefs(), c.option, c.others)
		if p, code := fieldOf(t, err); p != c.pointer || code != c.code {
			t.Errorf("%s: %s %s, want %s %s", c.name, p, code, c.pointer, c.code)
		}
	}
}

// TestAutocompleteAnswerValidation covers the answer rules of design note 9.
func TestAutocompleteAnswerValidation(t *testing.T) {
	t.Parallel()
	const maxChoices = 3
	ok := []domain.Choice{{Name: "High priority", Value: "high"}, {Name: "Low", Value: "low"}}
	if err := domain.ValidateAutocompleteAnswer(ok, maxChoices); err != nil {
		t.Fatalf("valid list: %v", err)
	}
	if err := domain.ValidateAutocompleteAnswer(nil, maxChoices); err != nil {
		t.Fatalf("empty list: %v", err)
	}
	cases := []struct {
		name          string
		choices       []domain.Choice
		pointer, code string
	}{
		{"too many", []domain.Choice{{Name: "a", Value: "a"}, {Name: "b", Value: "b"}, {Name: "c", Value: "c"}, {Name: "d", Value: "d"}}, "/choices", "too_many_items"},
		{"empty name", []domain.Choice{{Name: "a", Value: "a"}, {Name: "", Value: "b"}}, "/choices/1/name", "too_short"},
		{"empty value", []domain.Choice{{Name: "a", Value: ""}}, "/choices/0/value", "too_short"},
		{"control character", []domain.Choice{{Name: "a\nb", Value: "a"}}, "/choices/0/name", "control_character"},
		{"bidi control", []domain.Choice{{Name: "a", Value: "b\u202ec"}}, "/choices/0/value", "bidi_control"},
		{"invalid utf-8", []domain.Choice{{Name: "a\xff", Value: "a"}}, "/choices/0/name", "invalid_utf8"},
	}
	for _, c := range cases {
		err := domain.ValidateAutocompleteAnswer(c.choices, maxChoices)
		if p, code := fieldOf(t, err); p != c.pointer || code != c.code {
			t.Errorf("%s: %s %s, want %s %s", c.name, p, code, c.pointer, c.code)
		}
		if e, _ := errs.As(err); strings.Contains(e.Error()+e.Fields[0].Detail, "\u202e") || strings.Contains(e.Error()+e.Fields[0].Detail, "\xff") {
			t.Errorf("%s: the error repeats a value", c.name)
		}
	}
}

// TestAutocompleteGrant: trigger_kind autocomplete, no run_id, the command and the target thread,
// level trigger and the target tool class as given; the claims accept it.
func TestAutocompleteGrant(t *testing.T) {
	t.Parallel()
	in := domain.AutocompleteGrantInput{
		AgentID: ids.New(), TriggeredBy: ids.New(), CommandID: ids.New(), ThreadID: ids.New(), ChannelID: ids.New(), SpaceID: ids.New(),
		BillingMode: "organization", ToolClass: "chat", PermissionVersion: 7,
	}
	g := domain.AutocompleteGrant(in)
	if g.TriggerKind != domain.TriggerAutocomplete || !g.RunID.IsZero() || g.CommandID != in.CommandID || g.ThreadID != in.ThreadID ||
		g.Level != "trigger" || g.ToolClass != "chat" || g.BillingMode != "organization" || g.PermissionVersion != 7 ||
		g.AgentID != in.AgentID || g.TriggeredBy != in.TriggeredBy || g.ChannelID != in.ChannelID || g.SpaceID != in.SpaceID ||
		!g.MessageID.IsZero() || !g.InteractionID.IsZero() || g.Control {
		t.Fatalf("grant %+v", g)
	}
	c, err := domain.NewInvocationClaims(g, ids.New(), time.Now(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if c.RunID != "" || c.TriggerKind != "autocomplete" || c.CommandID != in.CommandID.String() || c.ThreadID != in.ThreadID.String() {
		t.Fatalf("claims %+v", c)
	}
}

// TestAutocompletePayload: the payload validates against CommandAutocompleteRequestedPayload,
// carries no null, options only when the query had some, expires_at as given in UTC milliseconds.
func TestAutocompletePayload(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 21, 8, 0, 0, 123456789, time.UTC)
	in := domain.AutocompletePayloadInput{
		RequestID: ids.New(), CommandID: ids.New(), ChannelID: ids.New(), ThreadID: ids.New(), UserID: ids.New(),
		Option: "ticket", Value: "47", ExpiresAt: now.Add(1500 * time.Millisecond),
	}
	p, err := domain.AutocompletePayload(in)
	if err != nil {
		t.Fatal(err)
	}
	validateAgainst(t, "CommandAutocompleteRequestedPayload", p)
	var m map[string]any
	if err := json.Unmarshal(p, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["options"]; ok {
		t.Fatalf("options without values: %s", p)
	}
	if m["expires_at"] != "2026-10-21T08:00:01.623Z" || m["thread_id"] != in.ThreadID.String() || m["user_id"] != in.UserID.String() ||
		m["request_id"] != in.RequestID.String() || m["option"] != "ticket" || m["value"] != "47" {
		t.Fatalf("payload %s", p)
	}
	in.Options = []domain.OptionValue{{Name: "priority", Value: "high"}}
	if p, err = domain.AutocompletePayload(in); err != nil {
		t.Fatal(err)
	}
	validateAgainst(t, "CommandAutocompleteRequestedPayload", p)
	if !strings.Contains(string(p), `"options":[{"name":"priority","value":"high"}]`) {
		t.Fatalf("payload %s", p)
	}
}

// TestAutocompleteRequestStates: open -> answered and open -> expired, no transition out of
// answered or expired, forgotten after expires_at plus the timeout.
func TestAutocompleteRequestStates(t *testing.T) {
	t.Parallel()
	exp := time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)
	const timeout = 2 * time.Second
	r := domain.NewAutocompleteRequest(exp)
	if r.State() != domain.AutocompleteOpen {
		t.Fatal("a new request is open")
	}
	if !r.Answer([]domain.Choice{{Name: "a", Value: "a"}}) || r.State() != domain.AutocompleteAnswered || len(r.Choices()) != 1 {
		t.Fatal("open -> answered")
	}
	if r.Answer(nil) || r.Expire() || r.State() != domain.AutocompleteAnswered || len(r.Choices()) != 1 {
		t.Fatal("no transition out of answered")
	}
	e := domain.NewAutocompleteRequest(exp)
	if !e.Expire() || e.State() != domain.AutocompleteExpired {
		t.Fatal("open -> expired")
	}
	if e.Answer([]domain.Choice{{Name: "a", Value: "a"}}) || e.Expire() || e.State() != domain.AutocompleteExpired || e.Choices() != nil {
		t.Fatal("no transition out of expired")
	}
	if e.Forgotten(exp.Add(timeout-time.Millisecond), timeout) || !e.Forgotten(exp.Add(timeout), timeout) {
		t.Fatal("forgotten exactly at expires_at plus the timeout")
	}
	// The waiter ends an open request at expires_at; the bound holds for it as well, so no entry
	// outlives it.
	o := domain.NewAutocompleteRequest(exp)
	if o.Forgotten(exp, timeout) || !o.Forgotten(exp.Add(timeout), timeout) {
		t.Fatal("an open request is forgotten at the same bound")
	}
}
