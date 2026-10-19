// SPDX-License-Identifier: Apache-2.0

package files

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// FileFinalized is the in-transaction event of a file that became final (docs/spec/12-dateien.md
// 2 "endgültig", 10; 02 "Ports, Ereignisse und Quellen je Modul"): the scan pass of M3-04
// publishes it in the unit of work that writes the final state, so a subscriber (the release of
// held agent invocations, M3-07) sees the state and the event together. It carries only the id;
// the subscriber reads what it needs.
type FileFinalized struct {
	// FileID is the file that is now deliverable, failed or blocked.
	FileID ids.ID
}

// The values of ScannerState.Status (12 5.3).
const (
	// ScannerOK: the last PING was answered with PONG.
	ScannerOK = "ok"
	// ScannerUnreachable: the last PING failed; files stay pending.
	ScannerUnreachable = "unreachable"
	// ScannerOff: file.scan.mode = off, no PING is sent.
	ScannerOff = "off"
	// ScannerUnknown: no PING has been sent yet in this process.
	ScannerUnknown = "unknown"
)

// ScannerState is the last result of the PING of clamd in this worker process.
type ScannerState struct {
	// Status is ScannerOK, ScannerUnreachable, ScannerOff or ScannerUnknown.
	Status string
	// CheckedAt is the time of the last check; zero while Status is ScannerUnknown or ScannerOff.
	CheckedAt time.Time
}

// ScannerHealth reports the reachability of the virus scanner as the worker sees it (12 5.3,
// health_source files.scan of 16 "Statusseite"). The state is kept in memory per process
// (docs/DECISIONS.md D-M3-04-11); M7-09 reads it for the status component. It is safe for
// concurrent use.
type ScannerHealth interface {
	// ScannerState returns the current state.
	ScannerState() ScannerState
}
