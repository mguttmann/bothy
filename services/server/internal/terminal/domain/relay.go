// SPDX-License-Identifier: Apache-2.0

package domain

// The pairing rules of the relay (Design note 7 of M2-44, docs/spec/24-terminal.md sections 5.4,
// 6.1 to 6.3 and 9): Decide maps an event of a session's slot to the closes of its two sides and
// the state change of the session. adapters/wsrelay executes the actions; app writes the state.

// Side is one end of a slot: the device stream or the data connection of the host service. The
// values are the attribute bothy.terminal.side.
type Side string

// The two sides.
const (
	SideClient Side = "client"
	SideHost   Side = "host"
)

// SlotState is what the relay knows of one session on this node.
type SlotState struct {
	// Started reports a session whose PTY ran (session.started was applied).
	Started bool
	// LocalConfirm reports a host whose last report says require_local_confirm.
	LocalConfirm bool
	// DevicePresent and HostPresent report a connected side; Paired both since their pairing.
	DevicePresent, HostPresent, Paired bool
}

// RelayEventKind names an event of a slot.
type RelayEventKind int

// The events of the table of Design note 7.
const (
	// EvPaired is both sides present.
	EvPaired RelayEventKind = iota + 1
	// EvDeviceClosed is the device side ending by itself.
	EvDeviceClosed
	// EvHostClosed is the host side ending by itself with Code and Reason.
	EvHostClosed
	// EvDeadline is the deadline of the slot.
	EvDeadline
	// EvQueueFull is the queue toward Side full.
	EvQueueFull
	// EvProtocolError is a text frame after identify or a message above the Noise limit from Side.
	EvProtocolError
	// EvNewGrant is a new grant of the session.
	EvNewGrant
	// EvStarted is session.started applied for the session.
	EvStarted
	// EvSessionEnded is the end of the session with Reason and Detail.
	EvSessionEnded
	// EvControlLost is the end of the control connection of the host with Code.
	EvControlLost
	// EvShutdown is the stop of the process.
	EvShutdown
)

// RelayEvent is one event of a slot.
type RelayEvent struct {
	Kind RelayEventKind
	// Side is the side of EvQueueFull (the receiver) and EvProtocolError (the offender).
	Side Side
	// Code and Reason are the close code and reason of EvHostClosed and EvControlLost.
	Code   int
	Reason string
	// End and Detail are the close reason and detail of EvSessionEnded.
	End    CloseReason
	Detail CloseDetail
}

// RelayActionKind names an action of the relay.
type RelayActionKind int

// The actions.
const (
	// ActCloseDevice closes the device side with Code and Reason.
	ActCloseDevice RelayActionKind = iota + 1
	// ActCloseHost closes the host side with Code and Reason.
	ActCloseHost
	// ActActivate turns a started, detached session active again (detached_at cleared).
	ActActivate
	// ActDetach turns an active session detached.
	ActDetach
	// ActEndOpenFailed ends the session with open_failed and Detail.
	ActEndOpenFailed
	// ActMeetDeadline stops the deadline of the slot.
	ActMeetDeadline
	// ActRearmDeadline arms a new deadline of terminal.open_timeout from now.
	ActRearmDeadline
	// ActCountUnpaired counts bothy.terminal.relay.unpaired once with Side.
	ActCountUnpaired
	// ActDrop removes the slot without writing its counters (the session ended).
	ActDrop
)

// RelayAction is one action of Decide.
type RelayAction struct {
	Kind   RelayActionKind
	Code   int
	Reason string
	Detail CloseDetail
	Side   Side
}

// The plain close codes of the relay that are no StreamCode of the device.
const (
	codeNormal       = 1000
	codeTermClosed   = 4100
	codeHostUnavail  = 4102
	codeServerRestrt = 4013
)

// Decide returns the actions of ev on a slot in state s, in order: the closes first, then the
// state change. A close of a side that is not present is left out.
//
// Example:
//
//	acts := domain.Decide(domain.SlotState{Started: true, DevicePresent: true, HostPresent: true, Paired: true},
//		domain.RelayEvent{Kind: domain.EvDeviceClosed})
func Decide(s SlotState, ev RelayEvent) []RelayAction {
	d := &decision{s: s}
	d.decide(ev)
	return d.out
}

// decision collects the actions of one event.
type decision struct {
	s   SlotState
	out []RelayAction
}

// add appends one action.
func (d *decision) add(a RelayAction) { d.out = append(d.out, a) }

// dev closes the device side when it is present.
func (d *decision) dev(code int, reason string) {
	if d.s.DevicePresent {
		d.add(RelayAction{Kind: ActCloseDevice, Code: code, Reason: reason})
	}
}

// host closes the host side when it is present.
func (d *decision) host(code int, reason string) {
	if d.s.HostPresent {
		d.add(RelayAction{Kind: ActCloseHost, Code: code, Reason: reason})
	}
}

// detach turns a started session detached.
func (d *decision) detach() {
	if d.s.Started {
		d.add(RelayAction{Kind: ActDetach})
	}
}

// rearm gives the slot a new deadline when a pairing ends before session.started on a host with
// local confirmation (ruling D-M2-44-5).
func (d *decision) rearm() {
	if !d.s.Started && d.s.LocalConfirm && d.s.Paired {
		d.add(RelayAction{Kind: ActRearmDeadline})
	}
}

// decide fills the actions of ev.
func (d *decision) decide(ev RelayEvent) {
	s, dev, host, detach, rearm := d.s, d.dev, d.host, d.detach, d.rearm
	switch ev.Kind {
	case EvPaired:
		if s.Started {
			d.add(RelayAction{Kind: ActActivate})
			d.add(RelayAction{Kind: ActMeetDeadline})
		} else if s.LocalConfirm {
			d.add(RelayAction{Kind: ActMeetDeadline})
		}
	case EvDeviceClosed:
		host(codeNormal, "")
		detach()
		rearm()
	case EvHostClosed:
		d.hostClosed(ev)
	case EvDeadline:
		d.deadline()
	case EvQueueFull:
		d.offender(ev.Side, int(StreamSlowConsumer))
	case EvProtocolError:
		d.offender(ev.Side, int(StreamDecodeError))
	case EvNewGrant:
		dev(int(StreamReplaced), "")
		host(codeNormal, "")
		if s.Paired {
			detach()
		}
	case EvStarted:
		if !s.Paired {
			d.add(RelayAction{Kind: ActDetach})
		}
		d.add(RelayAction{Kind: ActMeetDeadline})
	case EvSessionEnded:
		dev(int(StreamCodeForEnd(ev.End, ev.Detail)), string(ev.End))
		host(codeTermClosed, string(ev.End))
		d.add(RelayAction{Kind: ActDrop})
	case EvControlLost:
		if CloseCode(ev.Code).ChangesHostState() {
			return
		}
		dev(codeHostUnavail, "")
		host(codeNormal, "")
	case EvShutdown:
		dev(codeServerRestrt, "")
		host(codeServerRestrt, "")
	}
}

// offender is the row of a full queue (side the receiver, code 4014) and of a protocol error
// (side the offender, code 4002): that side gets code, the other the device 4102 or the host 1000;
// a started session detaches.
func (d *decision) offender(side Side, code int) {
	if side == SideClient {
		d.dev(code, "")
		d.host(codeNormal, "")
	} else {
		d.dev(codeHostUnavail, "")
		d.host(code, "")
	}
	d.detach()
	d.rearm()
}

// hostClosed is the row of a host side that ends by itself: a refusal of the handshake (4101 or
// 4104 with its close_detail) reaches the device with the same code and reason; any other close
// gives the device 4102.
func (d *decision) hostClosed(ev RelayEvent) {
	detail, refused := HostRefusal(ev.Code, ev.Reason)
	if !refused {
		d.dev(codeHostUnavail, "")
		d.detach()
		d.rearm()
		return
	}
	d.dev(ev.Code, string(detail))
	if !d.s.Started || StreamCode(ev.Code) == StreamHandshakeFailed {
		d.add(RelayAction{Kind: ActEndOpenFailed, Detail: detail})
		return
	}
	d.detach()
}

// deadline is the row of the deadline: a session that never started ends open_failed with
// open_timeout (the device gets 4102 without a host side, else 4100; the host 4100); a started
// one stays detached (device 4102, host 1000). A present side without its counterpart counts once
// as unpaired with its own side.
func (d *decision) deadline() {
	s := d.s
	if s.Started {
		d.dev(codeHostUnavail, "")
		d.host(codeNormal, "")
	} else {
		if s.HostPresent {
			d.dev(codeTermClosed, string(CloseOpenFailed))
		} else {
			d.dev(codeHostUnavail, "")
		}
		d.host(codeTermClosed, string(CloseOpenFailed))
	}
	switch {
	case s.DevicePresent && !s.HostPresent:
		d.add(RelayAction{Kind: ActCountUnpaired, Side: SideClient})
	case s.HostPresent && !s.DevicePresent:
		d.add(RelayAction{Kind: ActCountUnpaired, Side: SideHost})
	}
	if !s.Started {
		d.add(RelayAction{Kind: ActEndOpenFailed, Detail: DetailOpenTimeout})
	}
}
