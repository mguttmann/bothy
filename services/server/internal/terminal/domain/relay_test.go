// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// a builds an action.
func a(k domain.RelayActionKind, code int, reason string) domain.RelayAction {
	return domain.RelayAction{Kind: k, Code: code, Reason: reason}
}

// TestRelayDecide: one row per line of the table of Design note 7.
func TestRelayDecide(t *testing.T) {
	t.Parallel()
	both := domain.SlotState{DevicePresent: true, HostPresent: true, Paired: true}
	started := both
	started.Started = true
	devOnly := domain.SlotState{DevicePresent: true}
	hostOnly := domain.SlotState{HostPresent: true}
	startedDev := domain.SlotState{Started: true, DevicePresent: true}
	confirm := both
	confirm.LocalConfirm = true
	ending := started
	ending.HostEnding = true
	cases := []struct {
		name string
		s    domain.SlotState
		ev   domain.RelayEvent
		want []domain.RelayAction
	}{
		{"pairing of a started session", started, domain.RelayEvent{Kind: domain.EvPaired},
			[]domain.RelayAction{a(domain.ActActivate, 0, ""), a(domain.ActMeetDeadline, 0, "")}},
		{"pairing of an opening", both, domain.RelayEvent{Kind: domain.EvPaired}, nil},
		{"pairing of an opening with local confirmation", confirm, domain.RelayEvent{Kind: domain.EvPaired},
			[]domain.RelayAction{a(domain.ActMeetDeadline, 0, "")}},
		{"device closes, started", started, domain.RelayEvent{Kind: domain.EvDeviceClosed},
			[]domain.RelayAction{a(domain.ActCloseHost, 1000, ""), a(domain.ActDetach, 0, "")}},
		{"device closes, opening", both, domain.RelayEvent{Kind: domain.EvDeviceClosed},
			[]domain.RelayAction{a(domain.ActCloseHost, 1000, "")}},
		{"device closes, opening with local confirmation", confirm, domain.RelayEvent{Kind: domain.EvDeviceClosed},
			[]domain.RelayAction{a(domain.ActCloseHost, 1000, ""), a(domain.ActRearmDeadline, 0, "")}},
		{"host refuses 4101, opening", both, domain.RelayEvent{Kind: domain.EvHostClosed, Code: 4101, Reason: "token_replayed"},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4101, "token_replayed"), {Kind: domain.ActEndOpenFailed, Detail: "token_replayed"}}},
		{"host refuses 4101, started", started, domain.RelayEvent{Kind: domain.EvHostClosed, Code: 4101, Reason: "token_expired"},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4101, "token_expired"), a(domain.ActDetach, 0, "")}},
		{"host refuses 4104, started", started, domain.RelayEvent{Kind: domain.EvHostClosed, Code: 4104, Reason: "token_invalid"},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4104, "token_invalid"), {Kind: domain.ActEndOpenFailed, Detail: "token_invalid"}}},
		{"host closes otherwise, started", started, domain.RelayEvent{Kind: domain.EvHostClosed, Code: 1000},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4102, ""), a(domain.ActDetach, 0, "")}},
		{"host closes otherwise, opening", both, domain.RelayEvent{Kind: domain.EvHostClosed, Code: 1001},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4102, "")}},
		{"deadline, opening, host missing", devOnly, domain.RelayEvent{Kind: domain.EvDeadline},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4102, ""), {Kind: domain.ActCountUnpaired, Side: domain.SideClient},
				{Kind: domain.ActEndOpenFailed, Detail: "open_timeout"}}},
		{"deadline, opening, device missing", hostOnly, domain.RelayEvent{Kind: domain.EvDeadline},
			[]domain.RelayAction{a(domain.ActCloseHost, 4100, "open_failed"), {Kind: domain.ActCountUnpaired, Side: domain.SideHost},
				{Kind: domain.ActEndOpenFailed, Detail: "open_timeout"}}},
		{"deadline, opening, paired without start", both, domain.RelayEvent{Kind: domain.EvDeadline},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4100, "open_failed"), a(domain.ActCloseHost, 4100, "open_failed"),
				{Kind: domain.ActEndOpenFailed, Detail: "open_timeout"}}},
		{"deadline, opening, nobody", domain.SlotState{}, domain.RelayEvent{Kind: domain.EvDeadline},
			[]domain.RelayAction{{Kind: domain.ActEndOpenFailed, Detail: "open_timeout"}}},
		{"deadline, started, not paired", startedDev, domain.RelayEvent{Kind: domain.EvDeadline},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4102, ""), {Kind: domain.ActCountUnpaired, Side: domain.SideClient}}},
		{"queue toward the device full", started, domain.RelayEvent{Kind: domain.EvQueueFull, Side: domain.SideClient},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4014, ""), a(domain.ActCloseHost, 1000, ""), a(domain.ActDetach, 0, "")}},
		{"queue toward the host full", started, domain.RelayEvent{Kind: domain.EvQueueFull, Side: domain.SideHost},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4102, ""), a(domain.ActCloseHost, 4014, ""), a(domain.ActDetach, 0, "")}},
		{"text frame from the device", started, domain.RelayEvent{Kind: domain.EvProtocolError, Side: domain.SideClient},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4002, ""), a(domain.ActCloseHost, 1000, ""), a(domain.ActDetach, 0, "")}},
		{"text frame from the host, opening", both, domain.RelayEvent{Kind: domain.EvProtocolError, Side: domain.SideHost},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4102, ""), a(domain.ActCloseHost, 4002, "")}},
		{"new grant, started and paired", started, domain.RelayEvent{Kind: domain.EvNewGrant},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4103, ""), a(domain.ActCloseHost, 1000, ""), a(domain.ActDetach, 0, "")}},
		{"new grant, opening", both, domain.RelayEvent{Kind: domain.EvNewGrant},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4103, ""), a(domain.ActCloseHost, 1000, "")}},
		{"session.started without a paired device", hostOnly, domain.RelayEvent{Kind: domain.EvStarted},
			[]domain.RelayAction{a(domain.ActDetach, 0, ""), a(domain.ActMeetDeadline, 0, "")}},
		{"session.started paired", both, domain.RelayEvent{Kind: domain.EvStarted}, []domain.RelayAction{a(domain.ActMeetDeadline, 0, "")}},
		{"session ends", started, domain.RelayEvent{Kind: domain.EvSessionEnded, End: "open_failed", Detail: "token_invalid"},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4104, "open_failed"), a(domain.ActCloseHost, 4100, "open_failed"), a(domain.ActDrop, 0, "")}},
		{"control link lost", started, domain.RelayEvent{Kind: domain.EvControlLost, Code: 1006},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4102, ""), a(domain.ActCloseHost, 1000, "")}},
		{"control link lost, opening with local confirmation", confirm, domain.RelayEvent{Kind: domain.EvControlLost, Code: 1006},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4102, ""), a(domain.ActCloseHost, 1000, ""), a(domain.ActRearmDeadline, 0, "")}},
		{"control link ends with 4111", started, domain.RelayEvent{Kind: domain.EvControlLost, Code: 4111}, nil},
		// The host service ends its data connections when its control connection closes with 4110,
		// 4111 or 4112; those sessions ended in the commit of the change, whose
		// terminal_session.changed closes the device with 4100 and the reason, so the loss of the
		// host side neither closes the device with 4102 nor detaches (found by M2-48).
		{"host side ends while the host is ending", ending, domain.RelayEvent{Kind: domain.EvHostClosed, Code: 1000}, nil},
		{"host side refuses while the host is ending", ending, domain.RelayEvent{Kind: domain.EvHostClosed, Code: 4104, Reason: "token_invalid"}, nil},
		{"shutdown", started, domain.RelayEvent{Kind: domain.EvShutdown},
			[]domain.RelayAction{a(domain.ActCloseDevice, 4013, ""), a(domain.ActCloseHost, 4013, "")}},
	}
	for _, c := range cases {
		if got := domain.Decide(c.s, c.ev); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.name, got, c.want)
		}
	}
}
