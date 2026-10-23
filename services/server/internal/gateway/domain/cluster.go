// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The signals of the gateway between api nodes (M7-17, docs/spec/02-architecture.md sections 11.6
// and 16, 03 section 6.6): one JSON object per NOTIFY on the channel ephemeral with a kind, the
// node that sent it and IDs, versions and enumeration codes only, never content. The terminal
// sends its own kinds on the same channel (prefix terminal., M2-47); each receiver ignores the
// kinds of the other.

// SignalKind names a gateway signal on the channel ephemeral. The kinds are internal signals of
// the module, not event types of 05: no catalogue entry, no asyncapi message, nothing in the event
// log.
type SignalKind string

// The kinds (docs/decisions.d/M7-17.md D-M7-17-1).
const (
	// SignalTyping is typing.started of a person in a thread (thread_id, principal_id).
	SignalTyping SignalKind = "gateway.typing"
	// SignalRun is a run.updated of a run to a version (run_id, thread_id, version); the receiver
	// sends the run document as a snapshot.
	SignalRun SignalKind = "gateway.run"
	// SignalAgentPresence is agent.presence_changed (agent_id, state, and reported_billing_mode
	// and actor_id when the event carries them).
	SignalAgentPresence SignalKind = "gateway.agent_presence"
	// SignalDevice announces the new connection of a device (device_id, session_id, bind_id): the
	// older connection of the device on another node closes with 4017.
	SignalDevice SignalKind = "gateway.device"
	// SignalConsumer announces the takeover of an agent consumer (agent_id, session_id): the other
	// nodes check their connections of the agent against the cursor row and close with 4015.
	SignalConsumer SignalKind = "gateway.consumer"
	// SignalHint is notification.created (recipient_id, nid, hint_kind, quiet, count, and
	// thread_id, message_id and target_id when present).
	SignalHint SignalKind = "gateway.hint"
)

// SignalPrefix is the namespace of the gateway kinds on the shared channel.
const SignalPrefix = "gateway."

// NotifyPayloadMax is the protocol limit of a PostgreSQL NOTIFY payload: the payload must be
// shorter than 8000 bytes in the default configuration (PostgreSQL documentation, NOTIFY). It is a
// protocol constant, not a configuration key; the largest gateway signal stays far below it.
const NotifyPayloadMax = 8000

// Signal is one gateway signal. Node is the process that sent it; a node ignores its own. Which
// members a kind carries is fixed (signalShapes); every other member is zero.
type Signal struct {
	Kind SignalKind
	Node ids.ID
	// The IDs of the kinds.
	ThreadID, PrincipalID, RunID, AgentID, ActorID, DeviceID, SessionID, BindID ids.ID
	RecipientID, NotificationID, MessageID, TargetID                            ids.ID
	// Version is the run version of SignalRun (at least 1).
	Version int64
	// State and BillingMode are the codes of AgentPresencePayload; HintKind the kind of
	// NotificationCreatedPayload. Each is a lower-case code of the schema, never a text.
	State, BillingMode, HintKind string
	// Quiet and Count are the members of the same name of NotificationCreatedPayload.
	Quiet bool
	Count int64
}

// ErrMalformedSignal is a gateway payload that is no valid signal: the receiver drops it with one
// log record.
var ErrMalformedSignal = errors.New("gateway: malformed signal")

// wireSignal is the JSON form of a signal; absent members are omitted.
type wireSignal struct {
	Kind           SignalKind `json:"kind"`
	Node           ids.ID     `json:"node"`
	ThreadID       *ids.ID    `json:"thread_id,omitempty"`
	PrincipalID    *ids.ID    `json:"principal_id,omitempty"`
	RunID          *ids.ID    `json:"run_id,omitempty"`
	Version        *int64     `json:"version,omitempty"`
	AgentID        *ids.ID    `json:"agent_id,omitempty"`
	State          *string    `json:"state,omitempty"`
	BillingMode    *string    `json:"reported_billing_mode,omitempty"`
	ActorID        *ids.ID    `json:"actor_id,omitempty"`
	DeviceID       *ids.ID    `json:"device_id,omitempty"`
	SessionID      *ids.ID    `json:"session_id,omitempty"`
	BindID         *ids.ID    `json:"bind_id,omitempty"`
	RecipientID    *ids.ID    `json:"recipient_id,omitempty"`
	NotificationID *ids.ID    `json:"nid,omitempty"`
	HintKind       *string    `json:"hint_kind,omitempty"`
	MessageID      *ids.ID    `json:"message_id,omitempty"`
	TargetID       *ids.ID    `json:"target_id,omitempty"`
	Quiet          *bool      `json:"quiet,omitempty"`
	Count          *int64     `json:"count,omitempty"`
}

// The members of a signal: kind and node in every one, the others by kind.
const (
	mKind      = "kind"
	mNode      = "node"
	mThread    = "thread_id"
	mPrincipal = "principal_id"
	mRun       = "run_id"
	mVersion   = "version"
	mAgent     = "agent_id"
	mState     = "state"
	mBilling   = "reported_billing_mode"
	mActor     = "actor_id"
	mDevice    = "device_id"
	mSession   = "session_id"
	mBind      = "bind_id"
	mRecipient = "recipient_id"
	mNID       = "nid"
	mHintKind  = "hint_kind"
	mMessage   = "message_id"
	mTarget    = "target_id"
	mQuiet     = "quiet"
	mCount     = "count"
)

// shape is the members a kind requires and those it may carry.
type shape struct {
	required, optional []string
}

// signalShapes are the members of each kind (D-M7-17-1).
var signalShapes = map[SignalKind]shape{
	SignalTyping:        {required: []string{mThread, mPrincipal}},
	SignalRun:           {required: []string{mRun, mThread, mVersion}},
	SignalAgentPresence: {required: []string{mAgent, mState}, optional: []string{mBilling, mActor}},
	SignalDevice:        {required: []string{mDevice, mSession, mBind}},
	SignalConsumer:      {required: []string{mAgent, mSession}},
	SignalHint:          {required: []string{mRecipient, mNID, mHintKind, mQuiet, mCount}, optional: []string{mThread, mMessage, mTarget}},
}

// codePattern is the form of an enumeration code a signal may carry: a lower-case code of a 05
// schema, so a signal can never carry a text.
var codePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// EncodeSignal returns the compact JSON of s with exactly the members of its kind. Errors:
// ErrMalformedSignal for an unknown kind, a zero node, a missing or zero required member, a member
// the kind does not carry, a code that is no lower-case code, a version below 1 or a negative
// count.
//
// Example:
//
//	b, err := domain.EncodeSignal(domain.Signal{Kind: domain.SignalTyping, Node: node, ThreadID: t, PrincipalID: p})
//	// {"kind":"gateway.typing","node":"...","thread_id":"...","principal_id":"..."}
func EncodeSignal(s Signal) ([]byte, error) {
	sh, ok := signalShapes[s.Kind]
	if !ok || s.Node.IsZero() {
		return nil, ErrMalformedSignal
	}
	w := wireSignal{Kind: s.Kind, Node: s.Node}
	set := w.members(s)
	for name, present := range set {
		if present && !contains(sh.required, name) && !contains(sh.optional, name) {
			return nil, ErrMalformedSignal
		}
	}
	for _, name := range sh.required {
		if !set[name] && name != mQuiet {
			return nil, ErrMalformedSignal
		}
	}
	if s.Kind == SignalHint {
		q := s.Quiet
		w.Quiet = &q
		c := s.Count
		w.Count = &c
	}
	if err := w.valid(); err != nil {
		return nil, err
	}
	b, err := json.Marshal(w)
	if err != nil || len(b) >= NotifyPayloadMax {
		return nil, ErrMalformedSignal
	}
	return b, nil
}

// members fills w from s and reports which members are present (non-zero).
func (w *wireSignal) members(s Signal) map[string]bool {
	set := map[string]bool{}
	id := func(name string, v ids.ID, dst **ids.ID) {
		if !v.IsZero() {
			c := v
			*dst, set[name] = &c, true
		}
	}
	str := func(name, v string, dst **string) {
		if v != "" {
			c := v
			*dst, set[name] = &c, true
		}
	}
	id(mThread, s.ThreadID, &w.ThreadID)
	id(mPrincipal, s.PrincipalID, &w.PrincipalID)
	id(mRun, s.RunID, &w.RunID)
	id(mAgent, s.AgentID, &w.AgentID)
	id(mActor, s.ActorID, &w.ActorID)
	id(mDevice, s.DeviceID, &w.DeviceID)
	id(mSession, s.SessionID, &w.SessionID)
	id(mBind, s.BindID, &w.BindID)
	id(mRecipient, s.RecipientID, &w.RecipientID)
	id(mNID, s.NotificationID, &w.NotificationID)
	id(mMessage, s.MessageID, &w.MessageID)
	id(mTarget, s.TargetID, &w.TargetID)
	str(mState, s.State, &w.State)
	str(mBilling, s.BillingMode, &w.BillingMode)
	str(mHintKind, s.HintKind, &w.HintKind)
	if s.Version != 0 {
		v := s.Version
		w.Version, set[mVersion] = &v, true
	}
	if s.Kind != SignalHint {
		// quiet and count belong to hints only; a hint always carries both.
		if s.Quiet {
			set[mQuiet] = true
		}
		if s.Count != 0 {
			set[mCount] = true
		}
	} else {
		set[mQuiet], set[mCount] = true, true
	}
	return set
}

// valid checks the values of w: codes and numbers.
func (w *wireSignal) valid() error {
	for _, c := range []*string{w.State, w.BillingMode, w.HintKind} {
		if c != nil && !codePattern.MatchString(*c) {
			return ErrMalformedSignal
		}
	}
	if w.Version != nil && *w.Version < 1 || w.Count != nil && *w.Count < 0 {
		return ErrMalformedSignal
	}
	return nil
}

// DecodeSignal decodes a payload of the channel ephemeral strictly. A JSON object whose kind does
// not start with SignalPrefix is a foreign kind (ours false, no error), because the terminal shares
// the channel. A gateway kind must carry exactly the members of its kind, each once: IDs as
// canonical lower-case UUID strings, codes as lower-case codes, version at least 1, count not
// negative; anything else is ErrMalformedSignal. Every receiver reads state from the database or
// from its own memory, so a signal that arrives late, twice or out of order changes nothing wrong.
func DecodeSignal(payload []byte) (s Signal, ours bool, err error) {
	var head struct {
		Kind *string `json:"kind"`
	}
	if json.Unmarshal(payload, &head) != nil || head.Kind == nil {
		return Signal{}, false, ErrMalformedSignal
	}
	if !strings.HasPrefix(*head.Kind, SignalPrefix) {
		return Signal{}, false, nil
	}
	sh, known := signalShapes[SignalKind(*head.Kind)]
	if !known || duplicateMembers(payload) {
		return Signal{}, true, ErrMalformedSignal
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(payload, &raw) != nil {
		return Signal{}, true, ErrMalformedSignal
	}
	for name := range raw {
		if name != mKind && name != mNode && !contains(sh.required, name) && !contains(sh.optional, name) {
			return Signal{}, true, ErrMalformedSignal
		}
	}
	for _, name := range append([]string{mNode}, sh.required...) {
		if _, ok := raw[name]; !ok {
			return Signal{}, true, ErrMalformedSignal
		}
	}
	s, ok := decodeMembers(raw)
	if !ok {
		return Signal{}, true, ErrMalformedSignal
	}
	s.Kind = SignalKind(*head.Kind)
	return s, true, nil
}

// decodeMembers reads every present member of raw into a signal; false for a member of the wrong
// type or form.
func decodeMembers(raw map[string]json.RawMessage) (Signal, bool) {
	var s Signal
	d := memberDecoder{raw: raw, ok: true}
	for name, dst := range map[string]*ids.ID{
		mNode: &s.Node, mThread: &s.ThreadID, mPrincipal: &s.PrincipalID, mRun: &s.RunID, mAgent: &s.AgentID, mActor: &s.ActorID,
		mDevice: &s.DeviceID, mSession: &s.SessionID, mBind: &s.BindID, mRecipient: &s.RecipientID, mNID: &s.NotificationID,
		mMessage: &s.MessageID, mTarget: &s.TargetID,
	} {
		d.id(name, dst)
	}
	d.code(mState, &s.State)
	d.code(mBilling, &s.BillingMode)
	d.code(mHintKind, &s.HintKind)
	d.number(mVersion, &s.Version, 1)
	d.number(mCount, &s.Count, 0)
	if v, present := raw[mQuiet]; present && d.ok {
		d.ok = json.Unmarshal(v, &s.Quiet) == nil
	}
	return s, d.ok
}

// memberDecoder reads members of one payload; ok turns false at the first member of the wrong
// type or form, and every later read does nothing.
type memberDecoder struct {
	raw map[string]json.RawMessage
	ok  bool
}

// present returns the member name when it is present and nothing failed yet.
func (d *memberDecoder) present(name string) (json.RawMessage, bool) {
	v, ok := d.raw[name]
	return v, ok && d.ok
}

// id reads a canonical lower-case UUID string that is not zero into dst.
func (d *memberDecoder) id(name string, dst *ids.ID) {
	v, ok := d.present(name)
	if !ok {
		return
	}
	var text string
	if json.Unmarshal(v, &text) != nil {
		d.ok = false
		return
	}
	parsed, err := ids.Parse(text)
	if err != nil || parsed.IsZero() || parsed.String() != text {
		d.ok = false
		return
	}
	*dst = parsed
}

// code reads a lower-case code of a 05 schema into dst.
func (d *memberDecoder) code(name string, dst *string) {
	if v, ok := d.present(name); ok && (json.Unmarshal(v, dst) != nil || !codePattern.MatchString(*dst)) {
		d.ok = false
	}
}

// number reads an integer of at least low into dst.
func (d *memberDecoder) number(name string, dst *int64, low int64) {
	if v, ok := d.present(name); ok && (json.Unmarshal(v, dst) != nil || *dst < low || !isInteger(v)) {
		d.ok = false
	}
}

// isInteger reports whether the JSON number v has no fraction and no exponent.
func isInteger(v json.RawMessage) bool {
	return len(v) > 0 && !bytes.ContainsAny(v, ".eE")
}

// duplicateMembers reports whether the top-level object of payload names a member twice (the
// standard decoder would keep the last one silently).
func duplicateMembers(payload []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(payload))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return true
	}
	seen := map[string]bool{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return true
		}
		name, _ := t.(string)
		if seen[name] {
			return true
		}
		seen[name] = true
		var skip json.RawMessage
		if dec.Decode(&skip) != nil {
			return true
		}
	}
	if t, err := dec.Token(); err != nil || t != json.Delim('}') {
		return true
	}
	_, err := dec.Token()
	return !errors.Is(err, io.EOF)
}

// contains reports whether list holds name.
func contains(list []string, name string) bool {
	for _, n := range list {
		if n == name {
			return true
		}
	}
	return false
}
