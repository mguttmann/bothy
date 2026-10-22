// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The execution of slash commands (docs/spec/09-agents.md section 10.4, brief M2-16 design notes
// 10 to 13): the option values of an invocation and the payloads of command.invoked and of the
// command variant of agent.invocation. Pure; the definition comes from the agents port.

// The option types of command_options.v1.
const (
	optionString  = "string"
	optionInteger = "integer"
	optionBoolean = "boolean"
	optionChoice  = "choice"
	optionUser    = "user"
	optionFile    = "file"
)

// The field error codes of the option values (06 section 5).
const (
	fieldRequired      = "required"
	fieldUnknownMember = "unknown_member"
	fieldInvalid       = "invalid"
	fieldInvalidFormat = "invalid_format"
	fieldInvalidEnum   = "invalid_enum"
	fieldNotEnabled    = "not_enabled"
	// FieldTooLarge is the code of option values whose event payload would not fit
	// sync.event_payload_max_bytes (pointer /options).
	FieldTooLarge = "too_large"
)

// OptionDef is one option of the invoked command as its definition states it.
type OptionDef struct {
	Name, Type             string
	Required, Autocomplete bool
	// Choices are the static values.
	Choices []string
}

// OptionValue is one submitted option value (schema CommandOptionValue).
type OptionValue struct {
	Name, Value string
}

// ValidateOptionValues checks the submitted values against the definition (design note 10 i) and
// returns them in the order of the definition: an unknown option is unknown_member, an option twice
// invalid, a string must be a display text (SEC-IN-4), an integer the canonical decimal form of a
// signed 64-bit integer, a boolean true or false, a choice one of the static values unless the
// option has autocomplete (then any display text: the owner validates), a user a UUID in canonical
// form (existence is not checked, because the check would reveal accounts), a file not_enabled
// until files arrive; a required option missing is required on /options with the option named.
// No error repeats a value. Errors: validation_failed with one field error.
//
// Example:
//
//	vals, err := domain.ValidateOptionValues(defs, []domain.OptionValue{{Name: "ticket", Value: "T-1"}})
func ValidateOptionValues(defs []OptionDef, values []OptionValue) ([]OptionValue, error) {
	byName := make(map[string]int, len(defs))
	for i, d := range defs {
		byName[d.Name] = i
	}
	got := make(map[string]string, len(values))
	for i, v := range values {
		p := "/options/" + strconv.Itoa(i)
		di, ok := byName[v.Name]
		switch {
		case !ok:
			return nil, optionField(p+"/name", fieldUnknownMember, "the command has no such option")
		case hasKey(got, v.Name):
			return nil, optionField(p+"/name", fieldInvalid, "the option is given twice")
		}
		if code := valueCode(defs[di], v.Value); code != "" {
			return nil, optionField(p+"/value", code, "the value does not fit the option type")
		}
		got[v.Name] = v.Value
	}
	out := make([]OptionValue, 0, len(values))
	for _, d := range defs {
		v, ok := got[d.Name]
		if !ok {
			if d.Required {
				return nil, optionField("/options", fieldRequired, "the option "+d.Name+" is required")
			}
			continue
		}
		out = append(out, OptionValue{Name: d.Name, Value: v})
	}
	return out, nil
}

// hasKey reports whether m has k.
func hasKey(m map[string]string, k string) bool {
	_, ok := m[k]
	return ok
}

// valueCode returns the code of a value that does not fit d, "" for a valid one.
func valueCode(d OptionDef, v string) string {
	switch d.Type {
	case optionString:
		if textcheck.DisplayText(v) != nil {
			return fieldInvalid
		}
	case optionInteger:
		if n, err := strconv.ParseInt(v, 10, 64); err != nil || strconv.FormatInt(n, 10) != v {
			return fieldInvalidFormat
		}
	case optionBoolean:
		if v != "true" && v != "false" {
			return fieldInvalidEnum
		}
	case optionChoice:
		return choiceCode(d, v)
	case optionUser:
		if id, err := ids.Parse(v); err != nil || id.String() != v || strings.ToLower(v) != v {
			return fieldInvalidFormat
		}
	case optionFile:
		return fieldNotEnabled
	default:
		return fieldInvalid
	}
	return ""
}

// choiceCode is the rule of a choice value: one of the static values, or with autocomplete any
// display text.
func choiceCode(d OptionDef, v string) string {
	if d.Autocomplete {
		if textcheck.DisplayText(v) != nil {
			return fieldInvalidEnum
		}
		return ""
	}
	if slices.Contains(d.Choices, v) {
		return ""
	}
	return fieldInvalidEnum
}

// optionField returns validation_failed with one field error on pointer.
func optionField(pointer, code, detail string) *errs.Error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// OptionField returns validation_failed with one field error (too_large of the use case).
func OptionField(pointer, code, detail string) *errs.Error { return optionField(pointer, code, detail) }

// optionValuePayload is the schema CommandOptionValue.
type optionValuePayload struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// optionPayloads converts values; never nil, so the member options is an array.
func optionPayloads(values []OptionValue) []optionValuePayload {
	out := make([]optionValuePayload, 0, len(values))
	for _, v := range values {
		out = append(out, optionValuePayload(v))
	}
	return out
}

// CommandInvokedInput are the members of CommandInvokedPayload of a control command (design note
// 11).
type CommandInvokedInput struct {
	InvocationID, CommandID ids.ID
	CommandName             string
	ChannelID, ThreadID     ids.ID
	// RunID is the owner's active run in the thread; zero when the owner has none (R-AZ-25).
	RunID     ids.ID
	InvokedBy PrincipalRefPayload
	Options   []OptionValue
}

// commandInvokedPayload is CommandInvokedPayload in schema order.
type commandInvokedPayload struct {
	InvocationID string               `json:"invocation_id"`
	CommandID    string               `json:"command_id"`
	CommandName  string               `json:"command_name"`
	Control      bool                 `json:"control"`
	ChannelID    string               `json:"channel_id"`
	ThreadID     string               `json:"thread_id,omitzero"`
	RunID        string               `json:"run_id,omitzero"`
	InvokedBy    PrincipalRefPayload  `json:"invoked_by"`
	Options      []optionValuePayload `json:"options"`
}

// CommandInvokedPayload encodes the payload of command.invoked: control true, run_id exactly when
// the owner has an active run in the thread, no JSON null (ADR 0053), never a prompt. Errors: a
// zero ID, an empty name or an empty invoker.
func CommandInvokedPayload(in CommandInvokedInput) ([]byte, error) {
	if in.InvocationID.IsZero() || in.CommandID.IsZero() || in.ChannelID.IsZero() || in.ThreadID.IsZero() || in.CommandName == "" ||
		in.InvokedBy.ID == "" || in.InvokedBy.Kind == "" {
		return nil, errPayload
	}
	return json.Marshal(commandInvokedPayload{
		InvocationID: in.InvocationID.String(), CommandID: in.CommandID.String(), CommandName: in.CommandName, Control: true,
		ChannelID: in.ChannelID.String(), ThreadID: in.ThreadID.String(), RunID: idText(in.RunID), InvokedBy: in.InvokedBy,
		Options: optionPayloads(in.Options),
	})
}

// InvokedRunID reads run_id of a CommandInvokedPayload; zero when absent (the delivery of
// command.invoked, design note 13 a). Errors: a payload that is not the schema.
func InvokedRunID(payload []byte) (ids.ID, error) {
	p, err := invokedOf(payload)
	if err != nil || p.RunID == "" {
		return ids.ID{}, err
	}
	return ids.Parse(p.RunID)
}

// InvokedFacts are the members of a CommandInvokedPayload a delivery reads.
type InvokedFacts struct {
	CommandID, ThreadID, RunID, InvokedBy ids.ID
}

// InvokedFactsOf reads the command, thread, run and invoker of a CommandInvokedPayload. Errors: a
// payload that is not the schema or names no valid IDs.
func InvokedFactsOf(payload []byte) (InvokedFacts, error) {
	p, err := invokedOf(payload)
	if err != nil {
		return InvokedFacts{}, err
	}
	var f InvokedFacts
	for _, x := range []struct {
		s   string
		dst *ids.ID
		opt bool
	}{{p.CommandID, &f.CommandID, false}, {p.ThreadID, &f.ThreadID, false}, {p.RunID, &f.RunID, true}, {p.InvokedBy.ID, &f.InvokedBy, false}} {
		if x.s == "" && x.opt {
			continue
		}
		id, err := ids.Parse(x.s)
		if err != nil || id.IsZero() {
			return InvokedFacts{}, errPayload
		}
		*x.dst = id
	}
	return f, nil
}

// invokedOf decodes a CommandInvokedPayload.
func invokedOf(payload []byte) (commandInvokedPayload, error) {
	var p commandInvokedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return commandInvokedPayload{}, errPayload
	}
	return p, nil
}

// InvocationCommandID reads command_id of an AgentInvocationPayload with trigger_kind command, the
// triggering object of its delivery (design note 13 b: the payload is immutable, while
// runs.triggering_command_id may become NULL). Errors: a payload without a valid command_id.
func InvocationCommandID(payload []byte) (ids.ID, error) {
	var p struct {
		CommandID string `json:"command_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return ids.ID{}, errPayload
	}
	id, err := ids.Parse(p.CommandID)
	if err != nil || id.IsZero() {
		return ids.ID{}, errPayload
	}
	return id, nil
}
