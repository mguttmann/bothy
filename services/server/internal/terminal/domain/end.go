// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// EndSource says who decided the end of a session (M2-46 Design note 1): the server (a rights
// change, a revocation, a limit of the relay or of terminal.sweep, the recheck), the host service
// (session.ended) or a person through closeTerminalSession. Only an end with source close_request
// keeps closed_by and writes terminal.close instead of terminal.terminate.
type EndSource string

// The sources of an end.
const (
	// EndByServer is an end the server decided.
	EndByServer EndSource = "server"
	// EndByHost is an end the host service reported.
	EndByHost EndSource = "host"
	// EndByCloseRequest is an end through closeTerminalSession.
	EndByCloseRequest EndSource = "close_request"
)

// Progress are counts of a session the relay of this node has not written yet: the transport
// bytes from and to the device since its last write and the time of the last device message (zero
// for none). The end path adds the bytes to the row and moves last_input_at only forward, so each
// byte is counted once (ruling D-M2-46-7).
type Progress struct {
	BytesIn, BytesOut int64
	LastInputAt       time.Time
}

// End is one end of a session as the one end function writes it: the reason and the detail of the
// closed lists of 24 section 10.3 (ValidateEnd), the exit code (kept only with process_exited from
// the host), the person who closed (kept only with source close_request), the source and the
// counts of the relay that ends it, if any.
type End struct {
	Reason   CloseReason
	Detail   CloseDetail
	ExitCode *int
	ClosedBy ids.ID
	Source   EndSource
	Progress *Progress
}

// ServerEnd returns the end of a session the server decided with reason, without a detail.
//
// Example:
//
//	end := domain.ServerEnd(domain.CloseAccessRevoked)
func ServerEnd(r CloseReason) End { return End{Reason: r, Source: EndByServer} }

// HostEnd returns the end the host service reported in session.ended (detail and exit code as
// reported, M2-43 Design note 10).
func HostEnd(e HostSessionEnd) End {
	return End{Reason: e.Reason, Detail: e.Detail, ExitCode: e.ExitCode, Source: EndByHost}
}

// Normalized returns e with what its source may not carry removed: an exit code only with
// process_exited reported by the host, a closer only with close_request.
func (e End) Normalized() End {
	if e.Source != EndByHost || e.Reason != CloseProcessExited {
		e.ExitCode = nil
	}
	if e.Source != EndByCloseRequest {
		e.ClosedBy = ids.ID{}
	}
	return e
}
