// SPDX-License-Identifier: Apache-2.0

package domain

// ReleaseDelayed decides a push_delayed route when its NotBefore has come (13 4.4 "Desktop
// aktiv", D-M3-17-6): true sends the push, false drops it because the person has read the hint
// in the meantime.
//
// hintSeq is the thread_seq of the message the hint points at; for a flush hint it is
// threads.last_message_seq at the flush. readSeq is the read position of 03 read_states, the
// greater of last_read_seq and threads.unread_floor_seq. The push is dropped exactly when
// readSeq >= hintSeq. A hint without a message (hintSeq zero or less: security, admin, or a run
// hint without a message) is always released.
//
// Example:
//
//	domain.ReleaseDelayed(5, 4) // true: message 5 is still unread
//	domain.ReleaseDelayed(5, 5) // false: read on the desktop meanwhile
func ReleaseDelayed(hintSeq, readSeq int64) bool {
	if hintSeq <= 0 {
		return true
	}
	return readSeq < hintSeq
}
