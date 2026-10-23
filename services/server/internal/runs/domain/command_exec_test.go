// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// optionDefs are the options of the test command, in definition order.
func optionDefs() []domain.OptionDef {
	return []domain.OptionDef{
		{Name: "ticket", Type: "string", Required: true},
		{Name: "count", Type: "integer"},
		{Name: "urgent", Type: "boolean"},
		{Name: "prio", Type: "choice", Choices: []string{"high", "low"}},
		{Name: "team", Type: "choice", Autocomplete: true},
		{Name: "owner", Type: "user"},
		{Name: "file", Type: "file"},
	}
}

// TestValidateOptionValues covers every rule of design note 10 i.
func TestValidateOptionValues(t *testing.T) {
	t.Parallel()
	secret := "SECRETVALUE"
	ov := func(name, value string) domain.OptionValue { return domain.OptionValue{Name: name, Value: value} }
	cases := []struct {
		name    string
		values  []domain.OptionValue
		pointer string
		code    string
	}{
		{"required missing", []domain.OptionValue{ov("count", "1")}, "/options", "required"},
		{"unknown option", []domain.OptionValue{ov("ticket", "a"), ov("nope", secret)}, "/options/1/name", "unknown_member"},
		{"option twice", []domain.OptionValue{ov("ticket", "a"), ov("ticket", "b")}, "/options/1/name", "invalid"},
		{"string bidi", []domain.OptionValue{ov("ticket", secret+"\u202e")}, "/options/0/value", "invalid"},
		{"string control", []domain.OptionValue{ov("ticket", secret+"\x07")}, "/options/0/value", "invalid"},
		{"integer leading zero", []domain.OptionValue{ov("ticket", "a"), ov("count", "007")}, "/options/1/value", "invalid_format"},
		{"integer plus", []domain.OptionValue{ov("ticket", "a"), ov("count", "+7")}, "/options/1/value", "invalid_format"},
		{"integer overflow", []domain.OptionValue{ov("ticket", "a"), ov("count", "1"+strings.Repeat("0", 19))}, "/options/1/value", "invalid_format"},
		{"boolean", []domain.OptionValue{ov("ticket", "a"), ov("urgent", "yes")}, "/options/1/value", "invalid_enum"},
		{"choice", []domain.OptionValue{ov("ticket", "a"), ov("prio", secret)}, "/options/1/value", "invalid_enum"},
		{"autocomplete choice bidi", []domain.OptionValue{ov("ticket", "a"), ov("team", "\u2067")}, "/options/1/value", "invalid_enum"},
		{"user", []domain.OptionValue{ov("ticket", "a"), ov("owner", "0192F4A0-7B1C-7BEB-80FC-F336F4FE79AA")}, "/options/1/value", "invalid_format"},
		{"file", []domain.OptionValue{ov("ticket", "a"), ov("file", ids.New().String())}, "/options/1/value", "not_enabled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.ValidateOptionValues(optionDefs(), tc.values)
			e, ok := errs.As(err)
			if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != tc.pointer || e.Fields[0].Code != tc.code {
				t.Fatalf("got %v %+v, want %s %s", err, e, tc.pointer, tc.code)
			}
			if strings.Contains(e.Detail+e.Fields[0].Detail, secret) {
				t.Fatalf("detail repeats a value: %q", e.Fields[0].Detail)
			}
		})
	}
	user := ids.New().String()
	got, err := domain.ValidateOptionValues(optionDefs(), []domain.OptionValue{
		ov("owner", user), ov("team", "any team"), ov("urgent", "false"), ov("count", "-12"), ov("prio", "low"), ov("ticket", "T-1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ticket", "count", "urgent", "prio", "team", "owner"}
	if len(got) != len(want) {
		t.Fatalf("values %+v", got)
	}
	for i, n := range want {
		if got[i].Name != n {
			t.Fatalf("order %+v, want %v", got, want)
		}
	}
	if got, err := domain.ValidateOptionValues(nil, nil); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("no options: %v %v", got, err)
	}
}

// FuzzValidateOptionValues: no panic, and a value with a control or bidi character is never
// accepted for a string or an autocomplete choice.
func FuzzValidateOptionValues(f *testing.F) {
	for _, s := range []string{"", "a", "\u202e", "\x00", "07", "-0", "true", ids.New().String(), "ä\u200d"} {
		f.Add(s, s)
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		vals := []domain.OptionValue{{Name: "ticket", Value: a}, {Name: "team", Value: b}}
		got, err := domain.ValidateOptionValues(optionDefs(), vals)
		if err != nil {
			return
		}
		for _, v := range got {
			if textcheck.DisplayText(v.Value) != nil || !utf8.ValidString(v.Value) {
				t.Fatalf("accepted a value outside SEC-IN-4")
			}
		}
	})
}

// refOf is the invoker of the payload tests.
func refOf() domain.PrincipalRefPayload {
	return domain.PrincipalRefPayload{ID: ids.New().String(), Kind: "user", Handle: "alex", DisplayName: "Alex"}
}

// TestCommandInvokedPayload: the payload validates against CommandInvokedPayload, carries no
// null, run_id exactly when given and control true.
func TestCommandInvokedPayload(t *testing.T) {
	t.Parallel()
	in := domain.CommandInvokedInput{
		InvocationID: ids.New(), CommandID: ids.New(), CommandName: "usage", ChannelID: ids.New(), ThreadID: ids.New(), InvokedBy: refOf(),
		Options: []domain.OptionValue{{Name: "ticket", Value: "T-1"}},
	}
	p, err := domain.CommandInvokedPayload(in)
	if err != nil {
		t.Fatal(err)
	}
	validateAgainst(t, "CommandInvokedPayload", p)
	var m map[string]any
	if err := json.Unmarshal(p, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["run_id"]; ok || m["control"] != true || m["invocation_id"] != in.InvocationID.String() {
		t.Fatalf("payload %s", p)
	}
	in.RunID, in.Options = ids.New(), nil
	p, err = domain.CommandInvokedPayload(in)
	if err != nil {
		t.Fatal(err)
	}
	validateAgainst(t, "CommandInvokedPayload", p)
	if !strings.Contains(string(p), `"run_id":"`+in.RunID.String()+`"`) || !strings.Contains(string(p), `"options":[]`) {
		t.Fatalf("payload %s", p)
	}
	in.CommandID = ids.ID{}
	if _, err := domain.CommandInvokedPayload(in); err == nil {
		t.Fatal("zero command accepted")
	}
	run, err := domain.InvokedRunID(p)
	if err != nil || run != in.RunID {
		t.Fatalf("InvokedRunID %v %v", run, err)
	}
}

// TestAgentInvocationPayloadCommand: the command variant validates against the extended
// AgentInvocationPayload, has no message_id and carries the options in definition order.
func TestAgentInvocationPayloadCommand(t *testing.T) {
	t.Parallel()
	in := domain.InvocationPayloadInput{
		RunID: ids.New(), ThreadID: ids.New(), ChannelID: ids.New(), SpaceID: ids.New(), TriggeredBy: refOf(), BillingMode: "organization",
		SettingsVersion: 3, TriggerKind: "command", CommandInvocationID: ids.New(), CommandID: ids.New(), CommandName: "escalate",
		Options: []domain.OptionValue{{Name: "ticket", Value: "T-1"}, {Name: "prio", Value: "high"}},
	}
	p, err := domain.InvocationPayload(in, domain.DecideSession(true, nil))
	if err != nil {
		t.Fatal(err)
	}
	validateAgainst(t, "AgentInvocationPayload", p)
	if strings.Contains(string(p), "message_id") || !strings.Contains(string(p), `"trigger_kind":"command"`) ||
		strings.Index(string(p), `"ticket"`) > strings.Index(string(p), `"prio"`) {
		t.Fatalf("payload %s", p)
	}
	id, err := domain.InvocationCommandID(p)
	if err != nil || id != in.CommandID {
		t.Fatalf("InvocationCommandID %v %v", id, err)
	}
	in.Options = nil
	if p, err = domain.InvocationPayload(in, domain.DecideSession(true, nil)); err != nil || !strings.Contains(string(p), `"options":[]`) {
		t.Fatalf("payload %s %v", p, err)
	}
	in.CommandID = ids.ID{}
	if _, err := domain.InvocationPayload(in, domain.DecideSession(true, nil)); err == nil {
		t.Fatal("a command payload without the command")
	}
	in.CommandID, in.MessageID = ids.New(), ids.New()
	if _, err := domain.InvocationPayload(in, domain.DecideSession(true, nil)); err == nil {
		t.Fatal("a command payload with a message")
	}
}

// commandGrant is a grant of trigger kind command without a run.
func commandGrant() domain.InvocationGrant {
	return domain.InvocationGrant{
		AgentID: ids.New(), TriggeredBy: ids.New(), TriggerKind: domain.TriggerCommand, CommandID: ids.New(), ThreadID: ids.New(),
		ChannelID: ids.New(), SpaceID: ids.New(), Level: "trigger", BillingMode: "organization", ToolClass: "chat",
	}
}

// TestCommandGrantWithoutRunOnlyForControl: a command grant without RunID signs with Control and
// is an internal error without it; the claims carry command_id and no message_id (R-AZ-25).
func TestCommandGrantWithoutRunOnlyForControl(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0)
	g := commandGrant()
	if _, err := domain.NewInvocationClaims(g, ids.New(), now, time.Minute); !errors.Is(err, domain.ErrInvocationInvalid) {
		t.Fatalf("model command without run: %v", err)
	}
	g.Control = true
	c, err := domain.NewInvocationClaims(g, ids.New(), now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if c.CommandID != g.CommandID.String() || c.MessageID != "" || c.RunID != "" || c.TriggerKind != "command" {
		t.Fatalf("claims %+v", c)
	}
	g.RunID = ids.New()
	if c, err = domain.NewInvocationClaims(g, ids.New(), now, time.Minute); err != nil || c.RunID != g.RunID.String() {
		t.Fatalf("control with run: %+v %v", c, err)
	}
	m := grantOf(domain.TriggerMessage)
	m.Control = true
	if _, err := domain.NewInvocationClaims(m, ids.New(), now, time.Minute); !errors.Is(err, domain.ErrInvocationInvalid) {
		t.Fatalf("control on a message grant: %v", err)
	}
}
