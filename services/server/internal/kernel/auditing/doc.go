// SPDX-License-Identifier: Apache-2.0

// Package auditing is the cross-cutting port of the audit log (docs/spec/02-architecture.md
// "Cross-cutting ports (`internal/kernel`)", ADR 0021): the Recorder every module uses to write
// audit entries, the Entry it takes, the closed list of reason codes of 07 "4.1" (Reason,
// KnownReasons) and the typed attribute values of the details schema audit_details.v1
// (19 SEC-AUD-5). The audit module implements Recorder; internal/wire hands it to the module
// constructors.
//
// Record writes inside the caller's unit of work (phase 3.3 of 02 "Flow"), so the entry commits
// or rolls back with the change. RecordDenial is for denials whose business transaction rolls
// back: identical denials are aggregated per audit.deny_aggregate_window and written in an own
// transaction. RecordDetached writes at once in an own transaction.
//
// Example:
//
//	err := recorder.Record(ctx, auditing.Entry{
//		Action:   "owner.add",
//		Decision: auditing.DecisionAllow,
//		Attrs:    auditing.Attrs{"reason": auditing.ReasonText("handover to Sam")},
//	})
package auditing
