// SPDX-License-Identifier: Apache-2.0

package txn

import "strconv"

// Phase is a step of a unit of work (docs/spec/02-architektur.md "Ablauf"). platform/pg moves a
// unit through the phases in the order of the constants; CurrentPhase reads the phase from the
// context, so a counter guard (instance.Counters, M1-15) or Publish can refuse the wrong phase.
type Phase uint8

// The phases in the order platform/pg runs them. PhaseNone is the zero value: outside a unit.
const (
	// PhaseNone means no unit of work.
	PhaseNone Phase = iota
	// PhaseRightsBegin runs the rights begin hook of authz; only with RightsChange.
	PhaseRightsBegin
	// PhaseWork runs the function of the use case.
	PhaseWork
	// PhaseRightsCheck runs the rights check hook of authz (I1); only with RightsChange.
	PhaseRightsCheck
	// PhaseSync runs the hook of sync (event_seq, event_log, pg_notify).
	PhaseSync
	// PhaseAudit runs the hook of audit (audit_seq, hash chain).
	PhaseAudit
)

// phaseNames is indexed by Phase.
var phaseNames = [...]string{
	PhaseNone:        "none",
	PhaseRightsBegin: "rights_begin",
	PhaseWork:        "work",
	PhaseRightsCheck: "rights_check",
	PhaseSync:        "sync",
	PhaseAudit:       "audit",
}

// String returns the snake_case name of the phase ("rights_begin", "work", ...), or
// "phase(<n>)" for a value outside the constants, so a corrupt value stays visible in errors.
func (p Phase) String() string {
	if int(p) < len(phaseNames) {
		return phaseNames[p]
	}
	return "phase(" + strconv.Itoa(int(p)) + ")"
}

// hookPhase reports whether a hook may be registered for p: the two rights phases, sync and
// audit. The work phase belongs to the use case.
func hookPhase(p Phase) bool {
	switch p {
	case PhaseRightsBegin, PhaseRightsCheck, PhaseSync, PhaseAudit:
		return true
	case PhaseNone, PhaseWork:
		return false
	default:
		return false
	}
}

// eventPhase reports whether in-transaction events may be published in p: only the work and
// the rights check phase (02 "Ablauf": events arise only in phase 2 and 3.1), so nothing can be
// published after sync has numbered the durable events.
func eventPhase(p Phase) bool { return p == PhaseWork || p == PhaseRightsCheck }
