// SPDX-License-Identifier: Apache-2.0

package auditing

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Decision is audit_events.decision.
type Decision string

// The decisions of audit_events (03 "4.12", CHECK on the column).
const (
	// DecisionAllow records an allowed action (column "Audit" = ja in 05).
	DecisionAllow Decision = "allow"
	// DecisionDeny records a denial.
	DecisionDeny Decision = "deny"
	// DecisionError records a decision that failed, for example unknown_action.
	DecisionError Decision = "error"
)

// Entry is one auditable decision. Actor, client IP and correlation ID come from the context of
// the Recorder call (kernel/actor, kernel/reqmeta), never from the entry.
type Entry struct {
	// Action is the action id of 05, system actions included; a code (SEC-AUD-5).
	Action string
	// Decision is allow, deny or error.
	Decision Decision
	// Reason is the reason code; empty for allow, required for deny.
	Reason Reason
	// ResourceKind is the kind of the resource, a code; optional.
	ResourceKind string
	// ResourceID is the resource; the zero value means none.
	ResourceID ids.ID
	// Attrs are the flat attribute members of details (19 SEC-AUD-5); optional.
	Attrs Attrs
}

// Attrs are the attribute members of details. Keys match ^[a-z][a-z0-9_]*$; ip_hmac, ip_key_id
// and aggregate belong to the recorder and are refused; reason takes only a ReasonText value and
// violations only a Violations value, os_user only an OSUserName value, and those three values
// are refused under any other key.
type Attrs map[string]Value

// Kind is the kind of a Value.
type Kind uint8

// The kinds of Value. KindInvalid is the zero Value, which the recorder refuses.
const (
	// KindInvalid is the zero Value.
	KindInvalid Kind = iota
	// KindID is a UUID.
	KindID
	// KindCode is a short machine code, ^[a-z0-9][a-z0-9_.:-]*$.
	KindCode
	// KindInt is an integer within 2^53 - 1.
	KindInt
	// KindBool is a boolean.
	KindBool
	// KindTime is an instant, stored in the canonical time form with milliseconds.
	KindTime
	// KindList is a list of values of one of the kinds above.
	KindList
	// KindReasonText is the free-text reason of a management action, only under "reason".
	KindReasonText
	// KindViolations is the list of I1 violations, only under "violations".
	KindViolations
	// KindOSUserName is the name of a session user of a terminal host, only under "os_user"
	// (M2-39).
	KindOSUserName
	// KindOSUserNames is a list of names of session users of a terminal host, only under the
	// members of host.connect that name them (M2-43).
	KindOSUserNames
)

// Value is one attribute value of details. Build it with ID, Code, Int, Bool, Time, List,
// ReasonText or Violations; the recorder checks the content against audit_details.v1 when the
// entry is recorded, so a free text in a Code is refused there.
type Value struct {
	kind  Kind
	str   string
	n     int64
	b     bool
	t     time.Time
	list  []Value
	viols []Violation
}

// ID returns a UUID value.
func ID(v ids.ID) Value { return Value{kind: KindID, str: v.String()} }

// Code returns a short machine code, never free text.
func Code(s string) Value { return Value{kind: KindCode, str: s} }

// Int returns an integer value.
func Int(n int64) Value { return Value{kind: KindInt, n: n} }

// Bool returns a boolean value.
func Bool(b bool) Value { return Value{kind: KindBool, b: b} }

// Time returns an instant; it is stored in UTC, cut to milliseconds.
func Time(t time.Time) Value { return Value{kind: KindTime, t: t} }

// List returns a list of values of one of the kinds ID, Code, Int, Bool or Time, all of the same
// kind.
func List(vs ...Value) Value { return Value{kind: KindList, list: slices.Clone(vs)} }

// ReasonText returns the reason a management action requires, the only free text of details. It
// is accepted only under the key "reason" and checked with kernel/textcheck (SEC-IN-4); its length
// bound belongs to the request validation of the action (audit.reason_max_chars).
func ReasonText(s string) Value { return Value{kind: KindReasonText, str: s} }

// OSUserName returns the name of a session user of a terminal host (host_session_users.os_user,
// 24-terminal.md section 10.1), the value of the reserved member os_user. The audit module accepts
// it only under the key "os_user" and only in the form ^[A-Za-z0-9_][A-Za-z0-9_.-]*$, the form of
// the column; its length bound (host.os_user_max_chars) belongs to the request validation. M2-43
// reuses it for host.connect.
func OSUserName(s string) Value { return Value{kind: KindOSUserName, str: s} }

// OSUserNames returns a list of names of session users of a terminal host (M2-43 Design note 8b):
// the value of the members hd6_users, hd9_users, hd10_users, hd11_users, session_users_added and
// session_users_removed of host.connect. The audit module accepts it only under those keys and each
// name only in the form of OSUserName; the length bound (host.os_user_max_chars) belongs to the
// validation of the report.
//
// Example:
//
//	attrs := auditing.Attrs{"hd6_users": auditing.OSUserNames("bothy-s-alex", "bothy-s-sam")}
func OSUserNames(names ...string) Value {
	list := make([]Value, 0, len(names))
	for _, n := range names {
		list = append(list, OSUserName(n))
	}
	return Value{kind: KindOSUserNames, list: list}
}

// Violation is one entry of the reserved member violations of i1_violation (19 SEC-AUD-5); M1-29
// fills it.
type Violation struct {
	// Code is the violation code, for example i1_violation.
	Code string
	// AgentID is the agent whose backend policy is violated (i1_violation).
	AgentID ids.ID
	// HostID is the host with a personal LLM login whose grants violate I6 (i6_violation, M2-40);
	// exactly one of AgentID and HostID is set.
	HostID ids.ID
	// CredentialHolderID is the credential holder of that agent or host.
	CredentialHolderID ids.ID
	// PrincipalIDs are the principals that would gain trigger on a target of the agent.
	PrincipalIDs []ids.ID
}

// Violations returns the value of the reserved member violations; it is accepted only under the
// key "violations".
func Violations(vs ...Violation) Value {
	cp := make([]Violation, len(vs))
	for i, v := range vs {
		v.PrincipalIDs = slices.Clone(v.PrincipalIDs)
		cp[i] = v
	}
	return Value{kind: KindViolations, viols: cp}
}

// Kind returns the kind of v.
func (v Value) Kind() Kind { return v.kind }

// Data returns the content of v for the audit module: a string for KindID (canonical UUID text),
// KindCode, KindReasonText and KindOSUserName, int64, bool, time.Time, a copy of the elements ([]Value) for
// KindList and KindOSUserNames, a copy of the violations ([]Violation) for KindViolations, and nil
// for KindInvalid.
func (v Value) Data() any {
	switch v.kind {
	case KindID, KindCode, KindReasonText, KindOSUserName:
		return v.str
	case KindInt:
		return v.n
	case KindBool:
		return v.b
	case KindTime:
		return v.t
	case KindList, KindOSUserNames:
		return slices.Clone(v.list)
	case KindViolations:
		return Violations(v.viols...).viols
	case KindInvalid:
		return nil
	default:
		return nil
	}
}

// Recorder writes audit entries (ADR 0021). The audit module implements it; every module uses
// it through its constructor. It is safe for concurrent use.
//
// Example (an app package, inside its unit of work):
//
//	err := s.uow.Do(ctx, func(ctx context.Context) error {
//		if err := s.repo.InsertGrant(ctx, g); err != nil {
//			return err
//		}
//		return s.audit.Record(ctx, auditing.Entry{
//			Action: "grant.create", Decision: auditing.DecisionAllow,
//			ResourceKind: "grant", ResourceID: g.ID,
//			Attrs: auditing.Attrs{"level": auditing.Code("read"), "reason": auditing.ReasonText(reason)},
//		})
//	})
//	if errors.Is(err, errDenied) {
//		_ = s.audit.RecordDenial(ctx, auditing.Entry{Action: "grant.create",
//			Decision: auditing.DecisionDeny, Reason: auditing.ReasonI1Violation})
//	}
type Recorder interface {
	// Record adds e to the caller's unit of work; the audit phase hook writes it before the
	// commit, so the entry exists exactly when the change does. Errors: kind internal outside a
	// unit of work or in the audit phase, kind internal for an invalid entry (a programming
	// error, for example free text in a Code), and the storage error of the IP pseudonym key.
	Record(ctx context.Context, e Entry) error
	// RecordDenial aggregates a denial whose business transaction rolls back and writes it in
	// an own transaction later (05 "2.4", R-AZ-04). It returns an error only for an invalid
	// entry (kind internal), never for storage.
	RecordDenial(ctx context.Context, e Entry) error
	// RecordDetached writes e at once in an own transaction, never joining an ambient one
	// (decision error, for example unknown_action). Errors: as Record, plus the error of that
	// transaction.
	RecordDetached(ctx context.Context, e Entry) error
}
