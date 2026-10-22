// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The autocompletion of slash command options (docs/spec/09-agents.md section 10.5, brief M2-17
// design notes 3, 4, 6 and 9): the target thread, the rules of a query and of an answer, the
// payload of command.autocomplete_requested, the grant of its invocation token and the state of
// one open request. Pure; the app layer gathers the facts and holds the requests.

// The field error codes of autocompletion (06 section 5 point 3; docs/DECISIONS.md D-M2-17).
const (
	// FieldUnknownOption: the query names an option the command does not have.
	FieldUnknownOption = "unknown_option"
	// FieldAutocompleteNotEnabled: the option of the query has no autocomplete.
	FieldAutocompleteNotEnabled = "autocomplete_not_enabled"
	// FieldDuplicate: an option appears twice, or the option of the query again in options.
	FieldDuplicate = "duplicate"
	// FieldTooManyItems: an answer with more than command.autocomplete_max_choices choices.
	FieldTooManyItems = "too_many_items"
	// FieldTooShort: an empty name or value of a choice.
	FieldTooShort = "too_short"
	// FieldTooLong: a request whose payload does not fit sync.event_payload_max_bytes.
	FieldTooLong = "too_long"
)

// channelKindForum is the kind of a forum channel, which has no main thread (R-DM-11).
const channelKindForum = "forum"

// autocompleteTimestamp is RFC 3339 in UTC with milliseconds (04 section 2.2).
const autocompleteTimestamp = "2006-01-02T15:04:05.000Z"

// AutocompleteTarget returns the target thread of a request (R-AZ-26, design note 3): the thread
// of the request; without one the main thread of a text channel; zero in a forum channel, where
// the server asks nobody. channelKind is text, forum or dm; mainThread is the channel's main
// thread (zero for a forum).
//
// Example:
//
//	target := domain.AutocompleteTarget(q.ThreadID, ch.Kind, ch.MainThreadID)
func AutocompleteTarget(thread ids.ID, channelKind string, mainThread ids.ID) ids.ID {
	switch {
	case !thread.IsZero():
		return thread
	case channelKind == channelKindForum:
		return ids.ID{}
	}
	return mainThread
}

// ValidateAutocompleteQuery checks a query against the definition of the command (design note 9):
// option must name an option of the command (/option unknown_option) with autocomplete
// (/option autocomplete_not_enabled); each name in others must name another option of the command
// (/options/<i>/name unknown_option), at most once and never option itself (duplicate). Values are
// not checked, because the person is still typing; invokeCommand checks the final values. No error
// repeats a value. Errors: validation_failed with one field error.
//
// Example:
//
//	err := domain.ValidateAutocompleteQuery(defs, "ticket", []domain.OptionValue{{Name: "priority", Value: "hi"}})
func ValidateAutocompleteQuery(defs []OptionDef, option string, others []OptionValue) error {
	byName := make(map[string]OptionDef, len(defs))
	for _, d := range defs {
		byName[d.Name] = d
	}
	d, ok := byName[option]
	switch {
	case !ok:
		return optionField("/option", FieldUnknownOption, "the command has no such option")
	case !d.Autocomplete:
		return optionField("/option", FieldAutocompleteNotEnabled, "the option has no autocomplete")
	}
	seen := make(map[string]bool, len(others))
	for i, v := range others {
		p := "/options/" + strconv.Itoa(i) + "/name"
		_, known := byName[v.Name]
		switch {
		case !known:
			return optionField(p, FieldUnknownOption, "the command has no such option")
		case v.Name == option || seen[v.Name]:
			return optionField(p, FieldDuplicate, "the option is given twice")
		}
		seen[v.Name] = true
	}
	return nil
}

// Choice is one suggestion of an answer (schema CommandChoice).
type Choice struct {
	Name, Value string
}

// ValidateAutocompleteAnswer checks the owner's choices (design note 9): at most maxChoices
// (command.autocomplete_max_choices; /choices too_many_items), each name and value non-empty
// (/choices/<i>/name or /value too_short) and a display text of SEC-IN-4 (the code of the
// violated rule: invalid_utf8, control_character or bidi_control). No error repeats a value.
// Errors: validation_failed with one field error.
//
// Example:
//
//	err := domain.ValidateAutocompleteAnswer([]domain.Choice{{Name: "High", Value: "high"}}, 25)
func ValidateAutocompleteAnswer(choices []Choice, maxChoices int) error {
	if len(choices) > maxChoices {
		return optionField("/choices", FieldTooManyItems, "more choices than command.autocomplete_max_choices")
	}
	for i, c := range choices {
		base := "/choices/" + strconv.Itoa(i)
		for _, f := range []struct{ member, text string }{{"/name", c.Name}, {"/value", c.Value}} {
			if f.text == "" {
				return optionField(base+f.member, FieldTooShort, "the member must not be empty")
			}
			var te *textcheck.Error
			if err := textcheck.DisplayText(f.text); errors.As(err, &te) {
				return optionField(base+f.member, string(te.Rule), "the member is no display text")
			}
		}
	}
	return nil
}

// AutocompleteGrantInput are the facts of the invocation token of one autocomplete request
// (design note 4): the owner agent, the person, the command, the place with the target thread, the
// current target billing mode and tool class of the agent and the pinned permission version.
type AutocompleteGrantInput struct {
	AgentID, TriggeredBy, CommandID ids.ID
	ThreadID, ChannelID, SpaceID    ids.ID
	BillingMode, ToolClass          string
	PermissionVersion               int64
}

// AutocompleteGrant returns the grant of the token of an autocomplete request: trigger_kind
// autocomplete, the command as the triggering object, never a run (09 section 10.5 point 5,
// R-AG-14), and level trigger, because an allow of command.autocomplete implies the effective
// level trigger (as for a delivery, M2-08).
func AutocompleteGrant(in AutocompleteGrantInput) InvocationGrant {
	return InvocationGrant{
		AgentID: in.AgentID, TriggeredBy: in.TriggeredBy, TriggerKind: TriggerAutocomplete, CommandID: in.CommandID,
		ThreadID: in.ThreadID, ChannelID: in.ChannelID, SpaceID: in.SpaceID, Level: LevelTrigger,
		BillingMode: in.BillingMode, ToolClass: in.ToolClass, PermissionVersion: in.PermissionVersion,
	}
}

// AutocompletePayloadInput are the members of command.autocomplete_requested.
type AutocompletePayloadInput struct {
	RequestID, CommandID, ChannelID, ThreadID, UserID ids.ID
	Option, Value                                     string
	// Options are the other values of the query; nil or empty leaves the member out.
	Options []OptionValue
	// ExpiresAt is the end of the wait (now plus command.autocomplete_timeout).
	ExpiresAt time.Time
}

// autocompletePayload is the schema CommandAutocompleteRequestedPayload.
type autocompletePayload struct {
	RequestID string               `json:"request_id"`
	CommandID string               `json:"command_id"`
	ChannelID string               `json:"channel_id"`
	ThreadID  string               `json:"thread_id,omitzero"`
	UserID    string               `json:"user_id"`
	Option    string               `json:"option"`
	Value     string               `json:"value"`
	Options   []optionValuePayload `json:"options,omitempty"`
	ExpiresAt string               `json:"expires_at"`
}

// AutocompletePayload encodes the payload of command.autocomplete_requested (design note 4): no
// null, options only when the query had some, expires_at in UTC with milliseconds; no message
// content and no prompt (04 section 2.2 rule 1). Errors: the encoder's (never for valid input).
func AutocompletePayload(in AutocompletePayloadInput) ([]byte, error) {
	p := autocompletePayload{
		RequestID: in.RequestID.String(), CommandID: in.CommandID.String(), ChannelID: in.ChannelID.String(), ThreadID: idText(in.ThreadID),
		UserID: in.UserID.String(), Option: in.Option, Value: in.Value, ExpiresAt: in.ExpiresAt.UTC().Format(autocompleteTimestamp),
	}
	if len(in.Options) > 0 {
		p.Options = optionPayloads(in.Options)
	}
	return json.Marshal(p)
}

// AutocompleteState is the state of one open request (design note 6).
type AutocompleteState uint8

// The states of a request: open until the first valid answer or the end of the wait.
const (
	AutocompleteOpen AutocompleteState = iota
	AutocompleteAnswered
	AutocompleteExpired
)

// AutocompleteRequest is the state machine open -> answered | expired of one request. The caller
// guards it with a mutex; every transition is a compare-and-swap out of open, so a request has
// exactly one outcome.
type AutocompleteRequest struct {
	expiresAt time.Time
	state     AutocompleteState
	choices   []Choice
}

// NewAutocompleteRequest returns an open request that waits until expiresAt.
func NewAutocompleteRequest(expiresAt time.Time) AutocompleteRequest {
	return AutocompleteRequest{expiresAt: expiresAt}
}

// State returns the state.
func (r *AutocompleteRequest) State() AutocompleteState { return r.state }

// Choices returns the accepted choices; nil unless answered.
func (r *AutocompleteRequest) Choices() []Choice { return r.choices }

// Answer moves open -> answered with choices; false in any other state.
func (r *AutocompleteRequest) Answer(choices []Choice) bool {
	if r.state != AutocompleteOpen {
		return false
	}
	r.state, r.choices = AutocompleteAnswered, append([]Choice{}, choices...)
	return true
}

// Expire moves open -> expired; false in any other state.
func (r *AutocompleteRequest) Expire() bool {
	if r.state != AutocompleteOpen {
		return false
	}
	r.state = AutocompleteExpired
	return true
}

// Forgotten reports whether the request may be forgotten at now: from expires_at plus timeout
// (command.autocomplete_timeout) on (the tombstone rule of design note 6). Until then an answer
// learns 410 gone or 204 instead of 404.
func (r *AutocompleteRequest) Forgotten(now time.Time, timeout time.Duration) bool {
	return !now.Before(r.expiresAt.Add(timeout))
}

// AutocompleteOptionError returns validation_failed with one field error; the use case reports a
// payload above sync.event_payload_max_bytes with it (pointer "", code too_long).
func AutocompleteOptionError(pointer, code, detail string) *errs.Error {
	return optionField(pointer, code, detail)
}
