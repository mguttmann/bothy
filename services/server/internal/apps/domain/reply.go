// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The outcome markers of an answer of an interaction address that is no failure (M2-25 design
// note 9 b 5): OutcomeOf returns them next to the reasons of ReplyReasons.
const (
	// ReplyAnswered is a 200 in time: the body decides whether the reply is used.
	ReplyAnswered = "answered"
	// ReplyDeferred is a 202 in time: the app answers later through the API; the body is ignored.
	ReplyDeferred = "deferred"
)

// The closed list of reasons why a request to the interaction address of an app was not answered
// (M2-25 design note 2 b). The texts equal the runs.AppReason* values of the source port, which
// this package cannot import (it imports only the kernel); a test of the adapter holds both lists
// equal.
const (
	// ReasonInactive: the installation is not active; no request was sent.
	ReasonInactive = "inactive"
	// ReasonNoURL: the manifest names no delivery.interactions_url; no request was sent.
	ReasonNoURL = "no_url"
	// ReasonDenied: the re-check with the rights of the moment refused; no request was sent.
	ReasonDenied = "denied"
	// ReasonTimeout: no answer within app.interaction_timeout or the deadline of the request.
	ReasonTimeout = "timeout"
	// ReasonTransport: the connection failed for another reason.
	ReasonTransport = "transport"
	// ReasonBlocked: the host is not in webhook.allowed_hosts or the egress client refused it.
	ReasonBlocked = "blocked"
	// ReasonRedirect: a 3xx; redirects are never followed.
	ReasonRedirect = "redirect"
	// ReasonHTTPStatus: any status other than 200, 202 and 3xx.
	ReasonHTTPStatus = "http_status"
	// ReasonTooLarge: the answer exceeds http.max_json_body_bytes.
	ReasonTooLarge = "too_large"
	// ReasonInvalidReply: a 200 whose body is no valid MessageCreate or AutocompleteAnswer.
	ReasonInvalidReply = "invalid_reply"
	// ReasonNoThread: a 200 with a message, but the request has no thread to write it to.
	ReasonNoThread = "no_thread"
	// ReasonReplyRefused: a 200 with a message that the create path refused.
	ReasonReplyRefused = "reply_refused"
)

// ReplyReasons returns the closed list of reasons in the order of the specification, in a fresh
// slice.
func ReplyReasons() []string {
	return []string{ReasonInactive, ReasonNoURL, ReasonDenied, ReasonTimeout, ReasonTransport, ReasonBlocked, ReasonRedirect,
		ReasonHTTPStatus, ReasonTooLarge, ReasonInvalidReply, ReasonNoThread, ReasonReplyRefused}
}

// ErrReplyTooLarge reports an answer body above http.max_json_body_bytes. The adapter converts
// the size error of the egress client into it, so OutcomeOf needs no platform import.
var ErrReplyTooLarge = errors.New("apps: the answer of the interaction address is too large")

// OutcomeOf maps the answer of one request to an interaction address to ReplyAnswered,
// ReplyDeferred or a reason of ReplyReasons (M2-25 design note 9 b 5). A transport error decides
// before the status, also an error while reading the body of a 200: a deadline (also a net error
// that reports Timeout) is timeout, a *BlockedError, ErrHostNotAllowed or ErrInvalidTarget is
// blocked, ErrReplyTooLarge too_large and anything else transport. Without an error 200 is
// answered, 202 deferred, 3xx redirect and every other status, other 2xx included, http_status.
//
// Example:
//
//	domain.OutcomeOf(http.StatusAccepted, nil) // domain.ReplyDeferred
func OutcomeOf(status int, transportErr error) string {
	if transportErr != nil {
		return transportReason(transportErr)
	}
	switch {
	case status == http.StatusOK:
		return ReplyAnswered
	case status == http.StatusAccepted:
		return ReplyDeferred
	case status >= 300 && status <= 399:
		return ReasonRedirect
	default:
		return ReasonHTTPStatus
	}
}

// transportReason is the reason of a transport error.
func transportReason(err error) string {
	var blocked *BlockedError
	var timeout interface{ Timeout() bool }
	switch {
	case errors.Is(err, ErrReplyTooLarge):
		return ReasonTooLarge
	case errors.As(err, &blocked), errors.Is(err, ErrHostNotAllowed), errors.Is(err, ErrInvalidTarget):
		return ReasonBlocked
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &timeout) && timeout.Timeout():
		return ReasonTimeout
	default:
		return ReasonTransport
	}
}

// ErrInvalidReply reports an answer body that is no valid MessageCreate or AutocompleteAnswer.
// Errors that wrap it name the broken rule, never a member name or a value of the body.
var ErrInvalidReply = errors.New("apps: the answer of the interaction address is invalid")

// invalidReply wraps ErrInvalidReply with the rule that failed.
func invalidReply(rule string) error { return &replyError{rule: rule} }

// replyError is an ErrInvalidReply with its rule.
type replyError struct{ rule string }

// Error names the rule only.
func (e *replyError) Error() string { return ErrInvalidReply.Error() + ": " + e.rule }

// Unwrap makes errors.Is(err, ErrInvalidReply) hold.
func (e *replyError) Unwrap() error { return ErrInvalidReply }

// Reply is an answer body decoded as MessageCreate of docs/api/openapi.yaml: absent members are
// nil (pointers and lists) or false, so the adapter builds the create command exactly as the REST
// handler of createMessage does.
type Reply struct {
	// ClientMsgID is client_msg_id, always set: it makes a repeated job idempotent (04 5.1).
	ClientMsgID ids.ID
	// Kind is kind (text, voice or card); nil when absent.
	Kind *string
	// ContentMD is content_md; nil when absent.
	ContentMD *string
	// ReplyToMessageID is reply_to_message_id; nil when absent.
	ReplyToMessageID *ids.ID
	// AttachmentFileIDs is attachment_file_ids; nil when absent.
	AttachmentFileIDs []ids.ID
	// MentionIDs is mention_ids; nil when absent, so the create path skips the optional check.
	MentionIDs []ids.ID
	// Silent and ClearDraft are silent and clear_draft; false when absent.
	Silent, ClearDraft bool
	// RunID is run_id; nil when absent. The create path refuses it for an app.
	RunID *ids.ID
	// Components are the members of components in body order; nil when absent.
	Components []ReplyComponent
}

// ReplyComponent is one element of MessageCreate.components (schema ComponentCreate). Definition
// stays raw JSON, because the create path checks it against component.v1.
type ReplyComponent struct {
	// CustomID and Type are custom_id and type.
	CustomID, Type string
	// Definition is the JSON object definition as the body carried it.
	Definition json.RawMessage
	// SingleUse is single_use; nil when absent.
	SingleUse *bool
	// RequiredCapability and RestrictedTo are "" when absent.
	RequiredCapability, RestrictedTo string
	// ExpiresAt is expires_at; nil when absent.
	ExpiresAt *time.Time
}

// Choice is one suggestion of an AutocompleteAnswer (schema CommandChoice).
type Choice struct {
	Name, Value string
}

// The enums of the members of MessageCreate and ComponentCreate.
var (
	replyKinds         = []string{"text", "voice", "card"}
	componentTypes     = []string{"button", "select", "form"}
	capabilities       = []string{"approve_actions", "upload", "export", "manage", "terminal"}
	componentRestricts = []string{"any", "run_starter", "owner"}
)

// shape is the member set an object at one place of a body may have: members names each allowed
// member with the shape of its value, items the shape of the elements of an array. A shape
// without members and items is free: any value without null and without a repeated member name.
type shape struct {
	members map[string]*shape
	items   *shape
}

// free is the shape of a value whose content the schema leaves open (scalars, definition).
var free = &shape{}

// The shapes of the answer schemas.
var (
	componentShape = &shape{members: map[string]*shape{
		"custom_id": free, "type": free, "definition": free, "single_use": free, "required_capability": free,
		"restricted_to": free, "expires_at": free,
	}}
	messageShape = &shape{members: map[string]*shape{
		"client_msg_id": free, "kind": free, "content_md": free, "reply_to_message_id": free, "attachment_file_ids": free,
		"mention_ids": free, "silent": free, "clear_draft": free, "run_id": free,
		"components": {items: componentShape},
	}}
	answerShape = &shape{members: map[string]*shape{
		"choices": {items: &shape{members: map[string]*shape{"name": free, "value": free}}},
	}}
)

// wireReply is the JSON form of MessageCreate. Pointers tell an absent member from a zero value.
type wireReply struct {
	ClientMsgID       *ids.ID         `json:"client_msg_id"`
	Kind              *string         `json:"kind"`
	ContentMD         *string         `json:"content_md"`
	ReplyToMessageID  *ids.ID         `json:"reply_to_message_id"`
	AttachmentFileIDs []ids.ID        `json:"attachment_file_ids"`
	MentionIDs        []ids.ID        `json:"mention_ids"`
	Silent            *bool           `json:"silent"`
	ClearDraft        *bool           `json:"clear_draft"`
	RunID             *ids.ID         `json:"run_id"`
	Components        []wireComponent `json:"components"`
}

// wireComponent is the JSON form of ComponentCreate.
type wireComponent struct {
	CustomID           *string         `json:"custom_id"`
	Type               *string         `json:"type"`
	Definition         json.RawMessage `json:"definition"`
	SingleUse          *bool           `json:"single_use"`
	RequiredCapability *string         `json:"required_capability"`
	RestrictedTo       *string         `json:"restricted_to"`
	ExpiresAt          *time.Time      `json:"expires_at"`
}

// wireAnswer is the JSON form of AutocompleteAnswer.
type wireAnswer struct {
	Choices *[]wireChoice `json:"choices"`
}

// wireChoice is the JSON form of CommandChoice.
type wireChoice struct {
	Name  *string `json:"name"`
	Value *string `json:"value"`
}

// DecodeAppReply decodes the body of a 200 answer to a command or an interaction strictly as
// MessageCreate (M2-25 design note 9 b 5; 06 section 2 "Unknown values" and "No null"): valid
// UTF-8, exactly one JSON object, no repeated member name and no null at any depth, only the
// members of MessageCreate and ComponentCreate with their exact names, their types and enums, a
// client_msg_id, and for each component custom_id, type and a definition that is a JSON object.
// The rules of the create path (rights, content, card rules, run_id) are not checked here.
//
// Errors: one that wraps ErrInvalidReply; it never repeats a member name or a value.
//
// Example:
//
//	r, err := domain.DecodeAppReply(body) // errors.Is(err, domain.ErrInvalidReply) -> invalid_reply
func DecodeAppReply(body []byte) (Reply, error) {
	var w wireReply
	if err := decodeStrict(body, messageShape, &w); err != nil {
		return Reply{}, err
	}
	if w.ClientMsgID == nil {
		return Reply{}, invalidReply("client_msg_id is missing")
	}
	if w.Kind != nil && !slices.Contains(replyKinds, *w.Kind) {
		return Reply{}, invalidReply("kind is outside its enum")
	}
	r := Reply{
		ClientMsgID: *w.ClientMsgID, Kind: w.Kind, ContentMD: w.ContentMD, ReplyToMessageID: w.ReplyToMessageID,
		AttachmentFileIDs: w.AttachmentFileIDs, MentionIDs: w.MentionIDs, Silent: w.Silent != nil && *w.Silent,
		ClearDraft: w.ClearDraft != nil && *w.ClearDraft, RunID: w.RunID,
	}
	if w.Components != nil {
		r.Components = make([]ReplyComponent, 0, len(w.Components))
		for _, c := range w.Components {
			rc, err := replyComponent(c)
			if err != nil {
				return Reply{}, err
			}
			r.Components = append(r.Components, rc)
		}
	}
	return r, nil
}

// replyComponent checks the required members and enums of one ComponentCreate.
func replyComponent(c wireComponent) (ReplyComponent, error) {
	switch {
	case c.CustomID == nil || c.Type == nil || c.Definition == nil:
		return ReplyComponent{}, invalidReply("a component lacks custom_id, type or definition")
	case !slices.Contains(componentTypes, *c.Type):
		return ReplyComponent{}, invalidReply("a component type is outside its enum")
	case c.RequiredCapability != nil && !slices.Contains(capabilities, *c.RequiredCapability):
		return ReplyComponent{}, invalidReply("a required_capability is outside its enum")
	case c.RestrictedTo != nil && !slices.Contains(componentRestricts, *c.RestrictedTo):
		return ReplyComponent{}, invalidReply("a restricted_to is outside its enum")
	case bytes.TrimSpace(c.Definition)[0] != '{':
		return ReplyComponent{}, invalidReply("a definition is no JSON object")
	}
	rc := ReplyComponent{CustomID: *c.CustomID, Type: *c.Type, Definition: c.Definition, SingleUse: c.SingleUse, ExpiresAt: c.ExpiresAt}
	if c.RequiredCapability != nil {
		rc.RequiredCapability = *c.RequiredCapability
	}
	if c.RestrictedTo != nil {
		rc.RestrictedTo = *c.RestrictedTo
	}
	return rc, nil
}

// DecodeAutocompleteReply decodes the body of a 200 answer to an autocomplete request strictly as
// AutocompleteAnswer (the rules of DecodeAppReply for its members), then applies the answer rule
// of M2-17: at most maxChoices (command.autocomplete_max_choices), each name and value non-empty
// and a display text (kernel/textcheck.DisplayText). The result is never nil.
//
// Errors: one that wraps ErrInvalidReply; it never repeats a value.
//
// Example:
//
//	choices, err := domain.DecodeAutocompleteReply(body, cfg.MaxChoices)
func DecodeAutocompleteReply(body []byte, maxChoices int) ([]Choice, error) {
	var w wireAnswer
	if err := decodeStrict(body, answerShape, &w); err != nil {
		return nil, err
	}
	if w.Choices == nil {
		return nil, invalidReply("choices is missing")
	}
	if len(*w.Choices) > maxChoices {
		return nil, invalidReply("more choices than command.autocomplete_max_choices")
	}
	out := make([]Choice, 0, len(*w.Choices))
	for _, c := range *w.Choices {
		if c.Name == nil || c.Value == nil {
			return nil, invalidReply("a choice lacks name or value")
		}
		for _, s := range []string{*c.Name, *c.Value} {
			if s == "" || textcheck.DisplayText(s) != nil {
				return nil, invalidReply("a choice is empty or no display text")
			}
		}
		out = append(out, Choice{Name: *c.Name, Value: *c.Value})
	}
	return out, nil
}

// decodeStrict checks body against root with checkShape, then decodes it into v. The shape check
// comes first because encoding/json matches member names without regard to case and lets the last
// of two equal names win.
func decodeStrict(body []byte, root *shape, v any) error {
	if err := checkShape(body, root); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return invalidReply("a member has the wrong type or format")
	}
	return nil
}

// frame is one open object or array while checkShape walks the tokens.
type frame struct {
	object    bool
	expectKey bool
	keys      map[string]struct{}
	// s is the shape of the container; next the shape of the value of the member just read.
	s, next *shape
}

// checkShape walks the tokens of body without recursion, so a deeply nested body costs no stack:
// valid UTF-8, exactly one value, an object at the top, no null, no repeated member name and, where
// the shape names members, no other member.
func checkShape(body []byte, root *shape) error {
	if !utf8.Valid(body) {
		return invalidReply("the body is not valid UTF-8")
	}
	b := bytes.TrimSpace(body)
	if len(b) == 0 || b[0] != '{' {
		return invalidReply("the body is no JSON object")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	w := walker{root: root}
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) && len(w.stack) == 0 && w.values == 1 {
			return nil
		}
		if err != nil || (len(w.stack) == 0 && w.values == 1) {
			return invalidReply("the body is not one JSON value")
		}
		if err := w.step(tok); err != nil {
			return err
		}
	}
}

// walker is the state of checkShape: the open containers and the number of top-level values.
type walker struct {
	root   *shape
	stack  []*frame
	values int
}

// step applies one token: a closing delimiter ends its container, a member name is recorded,
// every other token is a value of the shape its place gives.
func (w *walker) step(tok json.Token) error {
	cur := w.root
	if len(w.stack) == 0 {
		w.values++
		return w.value(tok, cur)
	}
	top := w.stack[len(w.stack)-1]
	if d, ok := tok.(json.Delim); ok && (d == '}' || d == ']') {
		w.stack = w.stack[:len(w.stack)-1]
		return nil
	}
	if top.object && top.expectKey {
		return top.member(tok)
	}
	return w.value(tok, top.valueShape())
}

// value checks one value token of shape s: null is refused, an opening delimiter opens a container.
func (w *walker) value(tok json.Token, s *shape) error {
	if tok == nil {
		return invalidReply("the body contains null")
	}
	if d, ok := tok.(json.Delim); ok {
		w.stack = append(w.stack, &frame{object: d == '{', expectKey: d == '{', keys: map[string]struct{}{}, s: s})
	}
	return nil
}

// member records the member name tok of the object f and selects the shape of its value.
func (f *frame) member(tok json.Token) error {
	name, _ := tok.(string)
	if _, dup := f.keys[name]; dup {
		return invalidReply("a member name is repeated")
	}
	f.keys[name] = struct{}{}
	f.expectKey = false
	f.next = free
	if f.s.members != nil {
		s, ok := f.s.members[name]
		if !ok {
			return invalidReply("the body has an unknown member")
		}
		f.next = s
	}
	return nil
}

// valueShape returns the shape of the next value in f: the member's for an object (and expects a
// member name after it), the items' for an array.
func (f *frame) valueShape() *shape {
	if f.object {
		f.expectKey = true
		return f.next
	}
	if f.s.items != nil {
		return f.s.items
	}
	return free
}
