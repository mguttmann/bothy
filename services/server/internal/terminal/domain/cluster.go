// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The rules of the terminal across several api nodes (M2-47, docs/spec/24-terminal.md section 9,
// 03 section 6.6): the signals between nodes, the order of two control connections, the presence
// view, the probation of a new control connection and the rules of a lone side of the relay.

// SignalKind names a terminal signal on the channel ephemeral. The kinds are internal signals of
// the module, not event types of 05: no catalogue entry, no asyncapi message, nothing in the
// event log.
type SignalKind string

// The kinds (Design note 3 of M2-47).
const (
	// SignalSessionOpen asks every node to expect the session and the node of the control
	// connection to send session.open (host_id, session_id).
	SignalSessionOpen SignalKind = "terminal.session_open"
	// SignalHostClose asks every node to close the control connections of the host with the code
	// of its state (host_id).
	SignalHostClose SignalKind = "terminal.host_close"
	// SignalHostPresent announces an eligible control connection (host_id, conn_id).
	SignalHostPresent SignalKind = "terminal.host_present"
	// SignalHostGone withdraws one control connection (host_id, conn_id).
	SignalHostGone SignalKind = "terminal.host_gone"
	// SignalSessionPaired tells every node that the session paired on some node (host_id,
	// session_id); only for a session in opening on a host with require_local_confirm.
	SignalSessionPaired SignalKind = "terminal.session_paired"
)

// SignalPrefix is the namespace of the terminal kinds; M7-17 adds its own kinds to the same
// channel, which every terminal receiver ignores.
const SignalPrefix = "terminal."

// NotifyPayloadMax is the protocol limit of a PostgreSQL NOTIFY payload: the payload must be
// shorter than 8000 bytes in the default configuration (PostgreSQL documentation, NOTIFY). It is
// a protocol constant, not a configuration key.
const NotifyPayloadMax = 8000

// Signal is one terminal signal: a kind and IDs, nothing else (03 section 6.6, R-SY-25). Which IDs
// a kind carries is fixed (signalFields).
type Signal struct {
	Kind      SignalKind
	HostID    ids.ID
	SessionID ids.ID
	ConnID    ids.ID
}

// The JSON members of a signal.
const (
	fieldKind    = "kind"
	fieldHost    = "host_id"
	fieldSession = "session_id"
	fieldConn    = "conn_id"
)

// signalFields are the ID members of each kind besides host_id.
var signalFields = map[SignalKind]string{
	SignalSessionOpen: fieldSession, SignalHostClose: "", SignalHostPresent: fieldConn, SignalHostGone: fieldConn,
	SignalSessionPaired: fieldSession,
}

// ErrMalformedSignal is a terminal payload that is no valid signal: the receiver drops it with one
// log record.
var ErrMalformedSignal = errors.New("terminal: malformed signal")

// EncodeSignal returns the compact JSON of s with exactly the members of its kind: kind, host_id
// and session_id or conn_id, each a UUID string. Errors: ErrMalformedSignal for an unknown kind, a
// zero ID the kind needs or an ID the kind does not carry.
//
// Example:
//
//	b, err := domain.EncodeSignal(domain.Signal{Kind: domain.SignalHostClose, HostID: hostID})
//	// {"kind":"terminal.host_close","host_id":"..."}
func EncodeSignal(s Signal) ([]byte, error) {
	extra, ok := signalFields[s.Kind]
	if !ok || s.HostID.IsZero() {
		return nil, ErrMalformedSignal
	}
	session, conn := !s.SessionID.IsZero(), !s.ConnID.IsZero()
	if session != (extra == fieldSession) || conn != (extra == fieldConn) {
		return nil, ErrMalformedSignal
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, `{"%s":"%s","%s":"%s"`, fieldKind, s.Kind, fieldHost, s.HostID)
	switch extra {
	case fieldSession:
		fmt.Fprintf(&b, `,"%s":"%s"`, fieldSession, s.SessionID)
	case fieldConn:
		fmt.Fprintf(&b, `,"%s":"%s"`, fieldConn, s.ConnID)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// DecodeSignal decodes a payload of the channel ephemeral strictly. A JSON object whose kind does
// not start with SignalPrefix is a foreign kind (ours false, no error), because M7-17 shares the
// channel. A terminal kind must carry exactly its members, each once, each a canonical lower-case
// UUIDv7 string; anything else is ErrMalformedSignal. Every receiver reads state from the database,
// so a signal that arrives late, twice or out of order changes nothing wrong.
func DecodeSignal(payload []byte) (s Signal, ours bool, err error) {
	members, err := objectMembers(payload)
	if err != nil {
		return Signal{}, false, ErrMalformedSignal
	}
	kind, ok := members[fieldKind]
	if !ok || kind == notString {
		return Signal{}, false, ErrMalformedSignal
	}
	if !strings.HasPrefix(kind, SignalPrefix) {
		return Signal{}, false, nil
	}
	s.Kind = SignalKind(kind)
	extra, known := signalFields[s.Kind]
	if !known {
		return Signal{}, true, ErrMalformedSignal
	}
	want := 2
	if extra != "" {
		want = 3
	}
	if len(members) != want {
		return Signal{}, true, ErrMalformedSignal
	}
	if s.HostID, ok = canonicalID(members[fieldHost]); !ok {
		return Signal{}, true, ErrMalformedSignal
	}
	if extra == "" {
		return s, true, nil
	}
	v, present := members[extra]
	id, ok := canonicalID(v)
	if !present || !ok {
		return Signal{}, true, ErrMalformedSignal
	}
	if extra == fieldSession {
		s.SessionID = id
	} else {
		s.ConnID = id
	}
	return s, true, nil
}

// objectMembers reads one JSON object whose members are all strings, each name once, with nothing
// after it. The kind of a foreign signal may carry members of any type, so a non-string member
// fails only when the object is a terminal one; objectMembers keeps non-string members as an empty
// marker the caller rejects.
func objectMembers(payload []byte) (map[string]string, error) {
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, ErrMalformedSignal
	}
	out := map[string]string{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, ErrMalformedSignal
		}
		name, _ := t.(string)
		if _, dup := out[name]; dup {
			return nil, ErrMalformedSignal
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, ErrMalformedSignal
		}
		var v string
		if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &v) != nil {
			v = notString
		}
		out[name] = v
	}
	if t, err := dec.Token(); err != nil || t != json.Delim('}') {
		return nil, ErrMalformedSignal
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, ErrMalformedSignal
	}
	return out, nil
}

// notString marks a member that is no JSON string: never a valid kind or ID.
const notString = "\x00"

// canonicalID parses v as a UUIDv7 in its canonical lower-case text form.
func canonicalID(v string) (ids.ID, bool) {
	id, err := ids.Parse(v)
	if err != nil || id.IsZero() || id.String() != v {
		return ids.ID{}, false
	}
	return id, true
}

// OlderConnection reports whether the control connection a was accepted before b: both are
// UUIDv7 values of the accepting node's clock (kernel/ids), so their byte order is their accept
// order. Equal IDs are not older. Across nodes the order rests on synchronized clocks (24 3.4).
func OlderConnection(a, b ids.ID) bool { return bytes.Compare(a[:], b[:]) < 0 }

// PresenceView is what one node knows of the control connections of other nodes (Design note 6):
// per host the connection IDs with the time of their last terminal.host_present. It is safe for
// concurrent use.
type PresenceView struct {
	mu    sync.Mutex
	hosts map[ids.ID]map[ids.ID]time.Time
}

// NewPresenceView returns an empty view.
func NewPresenceView() *PresenceView { return &PresenceView{hosts: map[ids.ID]map[ids.ID]time.Time{}} }

// Seen records terminal.host_present of conn of host at at; an older time never replaces a newer.
func (v *PresenceView) Seen(host, conn ids.ID, at time.Time) {
	v.mu.Lock()
	defer v.mu.Unlock()
	m := v.hosts[host]
	if m == nil {
		m = map[ids.ID]time.Time{}
		v.hosts[host] = m
	}
	if prev, ok := m[conn]; !ok || at.After(prev) {
		m[conn] = at
	}
}

// Gone removes conn of host and only that connection.
func (v *PresenceView) Gone(host, conn ids.ID) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if m := v.hosts[host]; m != nil {
		delete(m, conn)
		if len(m) == 0 {
			delete(v.hosts, host)
		}
	}
}

// Connected reports whether an entry of host counts at now: no more than window
// (terminal.host_heartbeat_interval times terminal.host_heartbeat_miss_limit, the silence after
// which the holder closes the connection itself) has passed since its last Seen.
func (v *PresenceView) Connected(host ids.ID, now time.Time, window time.Duration) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, at := range v.hosts[host] {
		if !now.After(at.Add(window)) {
			return true
		}
	}
	return false
}

// ProbationEnd returns the end of the probation of a control connection that was first announced
// at firstPresent: twice terminal.host_duplicate_probe_timeout, the probe window of another node
// plus the same again for notification and commit (Design note 7 d). Before it the connection
// receives no session.open.
func ProbationEnd(firstPresent time.Time, probe time.Duration) time.Time {
	return firstPresent.Add(2 * probe)
}

// LoneOutcome is what happens to a side of the relay that found no counterpart on its node
// (Design note 8): Timer says whether the side gets the lone timer at all; at its deadline the
// side closes with Code, counts bothy.terminal.relay.unpaired with Side, and End says whether a
// session still in opening ends open_failed with open_timeout.
type LoneOutcome struct {
	Timer bool
	Code  int
	End   bool
	Side  Side
}

// LoneSideOutcome returns the outcome of a lone side of kind side for a session in state: a
// device stream closes with 4102 in every open state and ends an opening; a host data connection
// of an opening closes with 4102 and ends it; a host data connection of an active or detached
// session waits for an attaching device until the session ends (no timer); a closed session has
// no lone side.
func LoneSideOutcome(side Side, state string) LoneOutcome {
	switch {
	case state == SessionClosed:
		return LoneOutcome{}
	case side == SideClient:
		return LoneOutcome{Timer: true, Code: int(StreamHostUnavailable), End: state == SessionOpening, Side: SideClient}
	case state == SessionOpening:
		return LoneOutcome{Timer: true, Code: int(StreamHostUnavailable), End: true, Side: SideHost}
	}
	return LoneOutcome{}
}

// LoneDeadline returns the instant a lone side becomes unpaired: the acceptance of its identify
// plus terminal.open_timeout, inclusive (LoneExpired).
func LoneDeadline(identifiedAt time.Time, openTimeout time.Duration) time.Time {
	return identifiedAt.Add(openTimeout)
}

// LoneExpired reports whether a side with the lone deadline is unpaired at now: from the deadline
// on, inclusive.
func LoneExpired(deadline, now time.Time) bool { return !now.Before(deadline) }

// HostCloseCode returns the close code of the control connections of a host after
// terminal.host_close, from the state read after the signal (the codes of M2-43's single-node
// path): 4112 for disabled with duplicate_connection, 4111 for disabled and retired, 4110 for a
// host back in pairing (re-paired), 0 for a host that may stay connected (a stale or forged signal
// closes nothing).
func HostCloseCode(state State, disabledReason string) CloseCode {
	switch {
	case state == StateDisabled && disabledReason == DisabledDuplicateConnection:
		return CodeHostDuplicate
	case state == StateDisabled || state == StateRetired:
		return CodeHostDisabled
	case state == StatePairing:
		return CodeHostUnpaired
	}
	return 0
}
