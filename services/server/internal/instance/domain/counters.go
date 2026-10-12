// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"fmt"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// The names of the counter rows of sequence_counters (docs/spec/03-data-model.md
// "sequence_counters"); the table CHECK allows exactly these.
const (
	// CounterEventSeq numbers the durable events (event_log.seq).
	CounterEventSeq = "event_seq"
	// CounterAuditSeq numbers the audit chain (audit_events.seq).
	CounterAuditSeq = "audit_seq"
	// CounterPermissionVersion is the rights version.
	CounterPermissionVersion = "permission_version"
)

// The phase names of kernel/txn.Phase.String that the rules below use. The domain layer does
// not import kernel/txn (a port of kernel, not kernel-core), so the app layer passes the name.
const (
	phaseRightsBegin = "rights_begin"
	phaseRightsCheck = "rights_check"
	phaseSync        = "sync"
	phaseAudit       = "audit"
)

// advancePhase is the only phase in which each counter may be locked and advanced
// (02 "Lock order"; 03 "Lock order"): permission_version first (rights begin hook
// of authz), event_seq in the sync hook, audit_seq last in the audit hook.
var advancePhase = map[string]string{
	CounterPermissionVersion: phaseRightsBegin,
	CounterEventSeq:          phaseSync,
	CounterAuditSeq:          phaseAudit,
}

// CounterNames returns the three counter names in the lock order of 03 "Lock order".
func CounterNames() []string {
	return []string{CounterPermissionVersion, CounterEventSeq, CounterAuditSeq}
}

// checkName returns kind internal for a name that is not one of CounterNames: a caller with an
// unknown name is a programming error, rejected before any SQL (the table CHECK is the second
// line).
func checkName(counter string) error {
	if !slices.Contains(CounterNames(), counter) {
		return errs.New(errs.CodeInternalError, fmt.Sprintf("instance: unknown counter %q", printable(counter)))
	}
	return nil
}

// CheckAdvance reports whether counter may be advanced in the unit-of-work phase phase (the
// String of kernel/txn.Phase). Only one phase is allowed per counter, see advancePhase; this
// guard enforces the lock order at run time instead of trusting review.
//
// The error has kind internal and names counter and phase, for an unknown counter as well.
//
// Example:
//
//	err := domain.CheckAdvance(domain.CounterEventSeq, "work") // kind internal
func CheckAdvance(counter, phase string) error {
	if err := checkName(counter); err != nil {
		return err
	}
	if want := advancePhase[counter]; phase != want {
		return errs.New(errs.CodeInternalError,
			fmt.Sprintf("instance: Advance(%s) in phase %s; allowed only in phase %s (02 Sperrreihenfolge)", counter, phase, want))
	}
	return nil
}

// CheckRead reports whether counter may be read with Counters.Current in phase phase: only in
// one of the four hook phases (rights_begin, rights_check, sync, audit), where the hook either
// holds the lock or reads the committed value on purpose. Use cases never read through Counters.
//
// The error has kind internal and names counter and phase, for an unknown counter as well.
func CheckRead(counter, phase string) error {
	if err := checkName(counter); err != nil {
		return err
	}
	switch phase {
	case phaseRightsBegin, phaseRightsCheck, phaseSync, phaseAudit:
		return nil
	default:
		return errs.New(errs.CodeInternalError,
			fmt.Sprintf("instance: Current(%s) in phase %s; allowed only in the phase hooks (rights_begin, rights_check, sync, audit)", counter, phase))
	}
}

// CheckAdvanceCount reports whether n numbers may be taken: n must be at least 1, because
// values only increase (03 "sequence_counters"). The error has kind internal.
func CheckAdvanceCount(n int64) error {
	if n < 1 {
		return errs.New(errs.CodeInternalError, fmt.Sprintf("instance: Advance by %d; n must be at least 1", n))
	}
	return nil
}
