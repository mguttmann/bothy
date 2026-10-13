// SPDX-License-Identifier: Apache-2.0

package domain

// FlushState is what the caller (M3-19) knows at the end of a coalescing window of (recipient,
// thread).
type FlushState struct {
	// Collected is the number of message hints that Decide collected into the window.
	Collected int
	// Unread is the unread count of the thread for the recipient by the formula "Unread" of 03
	// read_states: threads.last_message_seq minus the greater of last_read_seq and
	// threads.unread_floor_seq, computed by the caller at the flush time.
	Unread int64
}

// Flush decides the one hint at the end of a coalescing window (13 4.5, D-M3-17-5). in.Kind must be
// message and in.Now is the flush time. Collected zero gives none with nothing_collected; Unread
// zero or less gives none with read (the person caught up); otherwise the steps allowed, level,
// activity, quiet and route of Decide run again at the flush time, because rights, level and quiet
// hours may have changed. Author and Window are ignored: a bundle has no single author, and the
// window is the one that ends now. A send carries Count = Unread and opens the next window
// (OpensWindowUntil = Now + CoalesceWindow, none for a zero length), so 13 1 point 3 "One notification
// per thread and window" holds under a steady stream; a flush that sends nothing leaves the window
// closed, so the next message after a quiet period goes at once.
//
// Errors: the cases of Decide, a kind other than message and a negative Collected give an
// internal_error of the kernel and the zero Decision.
//
// Example:
//
//	d, err := domain.Flush(in, domain.FlushState{Collected: 9, Unread: 10})
//	// d.Outcome == send, d.Count == 10, d.OpensWindowUntil == in.Now + in.Policy.CoalesceWindow
func Flush(in RecipientFacts, s FlushState) (Decision, error) {
	if err := validate(in); err != nil {
		return Decision{}, err
	}
	if in.Kind != KindMessage {
		return Decision{}, invalid("kind")
	}
	if s.Collected < 0 {
		return Decision{}, invalid("flush.collected")
	}
	d := Decision{Kind: in.Kind}
	if s.Collected == 0 {
		return skip(d, SkipNothingCollected), nil
	}
	if s.Unread <= 0 {
		return skip(d, SkipRead), nil
	}
	return decide(in, true, s.Unread)
}
