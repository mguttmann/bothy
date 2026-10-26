// SPDX-License-Identifier: Apache-2.0

package load

import (
	"net/http"
	"testing"
	"time"
)

// observerConn is a closed connection of an observer that received dispatches.
func observerConn(ds ...Dispatch) *Conn {
	return &Conn{observer: true, dispatches: ds}
}

func TestFanout_CountsTheObserverDispatchesOfTheWindow(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	from, to := t0.Add(10*time.Second), t0.Add(20*time.Second)
	at := func(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }
	sends := []sent{
		{Send: Send{Scheduled: at(5), Status: http.StatusCreated}, clientMsgID: "warmup", group: 0}, // before the window
		{Send: Send{Scheduled: at(11), Status: http.StatusCreated}, clientMsgID: "a", group: 0},     // received
		{Send: Send{Scheduled: at(12), Status: http.StatusCreated}, clientMsgID: "b", group: 1},     // received by group 1
		{Send: Send{Scheduled: at(13), Status: http.StatusCreated}, clientMsgID: "c", group: 0},     // missing
		{Send: Send{Scheduled: at(14), Status: http.StatusTooManyRequests}, clientMsgID: "d"},       // failed, not expected
		{Send: Send{Scheduled: at(15), Status: http.StatusCreated}, clientMsgID: "e", group: 2},     // observer not connected
		{Send: Send{Scheduled: at(20), Status: http.StatusCreated}, clientMsgID: "after", group: 0}, // at the end, outside
	}
	conns := make([]*Conn, 3*GroupSize)
	conns[0] = observerConn(
		Dispatch{ClientMsgID: "warmup", OccurredAt: at(5), Received: at(6)},
		Dispatch{ClientMsgID: "a", OccurredAt: at(11), Received: at(11).Add(40 * time.Millisecond)},
		Dispatch{ClientMsgID: "after", OccurredAt: at(20), Received: at(21)},
	)
	conns[GroupSize] = observerConn(Dispatch{ClientMsgID: "b", OccurredAt: at(12), Received: at(12).Add(90 * time.Millisecond)})
	lat, expected := fanout(sends, conns, from, to)
	if expected != 4 {
		t.Errorf("expected %d dispatches, want 4 (a, b, c, e)", expected)
	}
	if len(lat) != 2 || lat[0] != 40*time.Millisecond || lat[1] != 90*time.Millisecond {
		t.Errorf("latencies %v, want [40ms 90ms]", lat)
	}
	f := SummarizeFanout(lat, expected)
	if f.Samples != 2 || f.Expected != 4 || f.P99 != 90*time.Millisecond {
		t.Errorf("summary %+v", f)
	}
}

func TestErrorList_KeepsTheFirstErrors(t *testing.T) {
	t.Parallel()
	var e errorList
	for i := range maxErrors + 5 {
		e.add(&StatusError{Op: "createMessage", Status: 500 + i})
	}
	if len(e.list) != maxErrors || e.list[0] != "load: createMessage answered 500 " {
		t.Errorf("list %q", e.list)
	}
}
