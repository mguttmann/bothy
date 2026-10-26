// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// Kind is gdpr_requests.kind.
type Kind string

// The kinds of 03 gdpr_requests.
const (
	// KindExport is the access and portability request (Art. 15 and 20 GDPR).
	KindExport Kind = "export"
	// KindDeletion is the erasure of the account (Art. 17 GDPR, 20 section 4.4).
	KindDeletion Kind = "deletion"
	// KindRectification documents a correction by the owner (Art. 16 GDPR, 20 section 5.3).
	KindRectification Kind = "rectification"
)

// Valid reports one of the three kinds.
func (k Kind) Valid() bool {
	return k == KindExport || k == KindDeletion || k == KindRectification
}

// State is gdpr_requests.state.
type State string

// The states of 03 gdpr_requests.
const (
	StateRequested State = "requested"
	StateApproved  State = "approved"
	StateRunning   State = "running"
	StateCompleted State = "completed"
	StateFailed    State = "failed"
	StateRejected  State = "rejected"
	StateCancelled State = "cancelled"
)

// Valid reports one of the seven states.
func (s State) Valid() bool {
	return slices.Contains([]State{StateRequested, StateApproved, StateRunning, StateCompleted, StateFailed, StateRejected,
		StateCancelled}, s)
}

// Open reports a state of the partial unique index of 03: requested, approved or running.
func (s State) Open() bool { return s == StateRequested || s == StateApproved || s == StateRunning }

// transitions are the allowed changes of state (03 gdpr_requests, brief M7-06): an owner (or the
// system for the export of one's own data) approves or rejects a requested request; the job runs
// an approved export or deletion and ends it completed or failed; an approved rectification is
// completed by the decision itself (20 section 5.3); an open request that is not running can be
// cancelled. completed, failed, rejected and cancelled are final.
var transitions = map[State][]State{
	StateRequested: {StateApproved, StateRejected, StateCancelled},
	StateApproved:  {StateRunning, StateCompleted, StateCancelled},
	StateRunning:   {StateCompleted, StateFailed},
}

// CanMove reports whether from may change to to.
func CanMove(from, to State) bool { return slices.Contains(transitions[from], to) }

// Move returns to when from may change to it, else 409 invalid_state_transition.
func Move(from, to State) (State, error) {
	if !CanMove(from, to) {
		return from, errs.New(errs.CodeInvalidStateTransition, fmt.Sprintf("a %s request cannot become %s", from, to))
	}
	return to, nil
}

// Request is one row of gdpr_requests.
type Request struct {
	ID              ids.ID
	SubjectUserID   ids.ID
	Kind            Kind
	State           State
	RequestedBy     ids.ID
	ApprovedBy      ids.ID // zero with State approved or later: approved by the system
	Reason          string
	DueAt           time.Time
	ResultFileID    ids.ID // zero when there is no package (any more)
	ResultExpiresAt time.Time
	Progress        Progress
	Error           string
	CreatedAt       time.Time
	CompletedAt     time.Time
}

// Progress is gdpr_requests.progress: the steps and counters of a request, never content (03).
type Progress struct {
	// LastStep is the last finished step of the deletion (personaldata.Step); 0 before the first.
	LastStep int `json:"last_step,omitempty"`
	// ReminderSent records that gdpr.request_due went out for this request (R-AZ-35, once).
	ReminderSent bool `json:"reminder_sent,omitempty"`
	// ExtendedMonths is the extension of Art. 12(3) GDPR (1 or 2); 0 without one.
	ExtendedMonths int `json:"extended_months,omitempty"`
	// Files and Bytes count the files and the size of the export package.
	Files int   `json:"files,omitempty"`
	Bytes int64 `json:"bytes,omitempty"`
	// Content is privacy.account_deletion_content at the start of the deletion, so a resumed
	// deletion keeps the mode it began with.
	Content string `json:"content,omitempty"`
}

// MarshalProgress encodes p for the column.
func MarshalProgress(p Progress) ([]byte, error) { return json.Marshal(p) }

// UnmarshalProgress decodes the column; an empty or unknown member is ignored.
func UnmarshalProgress(b []byte) (Progress, error) {
	var p Progress
	if len(b) == 0 {
		return p, nil
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return Progress{}, err
	}
	return p, nil
}

// AutoApproved reports whether the system approves a new request at once (20 section 5.1 item
// 2): an export of one's own data (the subject files it) while privacy.export_requires_approval
// is off. An export an owner files for another person, a deletion and a rectification always
// need the decision of an owner.
func AutoApproved(kind Kind, subject, requestedBy ids.ID, requiresApproval bool) bool {
	return kind == KindExport && subject == requestedBy && !requiresApproval
}

// MaxReasonRunes bounds the reason of a request and of a decision.
const MaxReasonRunes = 1000

// CheckReason validates the reason of a request or a decision at pointer (/reason): not blank,
// at most MaxReasonRunes runes, free of control and bidi characters (SEC-IN-4, SEC-AUD-5),
// because it goes into the audit log and is communicated to the person.
//
// Errors: 422 validation_failed with the code required, too_long or the textcheck rule.
func CheckReason(reason, pointer string) error {
	if err := textcheck.DisplayText(reason); err != nil {
		var te *textcheck.Error
		rule := string(textcheck.RuleInvalidUTF8)
		if errors.As(err, &te) {
			rule = string(te.Rule)
		}
		return fieldError(pointer, rule, "the reason contains a forbidden character")
	}
	if strings.TrimFunc(reason, func(r rune) bool { return unicode.IsSpace(r) || unicode.Is(unicode.Cf, r) }) == "" {
		return fieldError(pointer, "required", "the reason is required")
	}
	if utf8.RuneCountInString(reason) > MaxReasonRunes {
		return fieldError(pointer, "too_long", "the reason is too long")
	}
	return nil
}

// fieldError is the validation_failed error with one field error.
func fieldError(pointer, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}
