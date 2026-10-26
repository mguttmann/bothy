// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

// The event types the consumer privacy.journal turns into entries (brief M7-07 design note 1).
const (
	eventMessageDeleted = "message.deleted"
	eventUserUpdated    = "user.updated"
)

// JournalState is the outgoing port to deletion_journal_state (03 4.12, R-DM-52); adapters/postgres
// implements it. Every method joins the ambient unit of work of ctx when there is one.
type JournalState interface {
	// Take raises next_number by one and returns the number it had: the number of the object
	// the caller writes now. The row lock it takes keeps the numbers dense until the commit.
	Take(ctx context.Context) (int64, error)
	// Numbers returns next_number and low_number.
	Numbers(ctx context.Context) (next, low int64, err error)
	// SetNext sets next_number to next (bothy restore finalize).
	SetNext(ctx context.Context, next int64) error
	// SetLow sets low_number to low; it never moves low_number down or above next_number.
	SetLow(ctx context.Context, low int64) error
}

// JournalRequests is the outgoing port of the consumer to the deletion requests (adapters/postgres).
type JournalRequests interface {
	// LockDeletionOf returns the newest deletion request of subject whose job began (running,
	// completed or failed), locked FOR UPDATE; found is false when there is none.
	LockDeletionOf(ctx context.Context, subject ids.ID) (domain.Request, bool, error)
}

// JournalMessages deletes a message again (the port messaging.JournalMessages, adapted in
// internal/wire, because privacy does not depend on messaging).
type JournalMessages interface {
	// DeleteForJournal deletes the message id without an actor and reports whether it did.
	DeleteForJournal(ctx context.Context, id ids.ID) (bool, error)
}

// BackupFloor is the outgoing port to the backups that are still kept (module status, adapted in
// internal/wire): the journal retention removes no object a restore of such a backup needs.
type BackupFloor interface {
	// JournalFloor returns the smallest deletion_journal.next_number of the manifests of the
	// backups that are still kept; limited is false when no backup is kept (no floor).
	JournalFloor(ctx context.Context) (floor int64, limited bool, err error)
}

// JournalEvent is the part of a durable event the consumer privacy.journal reads (sync.Event,
// mapped in internal/wire).
type JournalEvent struct {
	// Seq is the global sequence.
	Seq int64
	// Type is the event type.
	Type string
	// OccurredAt is the time of the transaction.
	OccurredAt time.Time
	// ResourceID is the affected object: the message of message.deleted, the person of
	// user.updated.
	ResourceID ids.ID
	// Payload is the JSON payload.
	Payload json.RawMessage
}

// JournalDeps are the collaborators of the deletion journal.
type JournalDeps struct {
	// State numbers the objects.
	State JournalState
	// Requests reads the deletion request of an account entry.
	Requests JournalRequests
	// Store writes the progress of that request.
	Store Store
	// Objects stores the objects (files.JournalObjects).
	Objects files.JournalObjects
	// Messages and Files delete a message or a file again on a restore.
	Messages JournalMessages
	Files    files.JournalFiles
	// People reads the status of a person on a restore.
	People identity.Directory
	// Eraser runs the steps of the account deletion on a restore (Service.EraseForJournal).
	Eraser *Service
	// Floor names the oldest object a kept backup needs.
	Floor BackupFloor
	// DeletionContent is privacy.account_deletion_content: the mode of an account entry whose
	// request is gone (only a request removed by hand; retention.privacy_job keeps it).
	DeletionContent personaldata.ContentMode
	// Clock nil means clock.System().
	Clock clock.Clock
}

// Journal is the deletion journal of docs/spec/20-privacy.md section 4.6 (brief M7-07): the
// consumer privacy.journal writes one object per deletion of a message or an account into the
// object storage, outside the database; ApplyJournal deletes again on a restore what the restored
// database brings back; the retention removes old objects. It is safe for concurrent use.
type Journal struct {
	d JournalDeps
}

// NewJournal returns the journal over d. Errors: a missing collaborator or an invalid content
// mode.
func NewJournal(d JournalDeps) (*Journal, error) {
	if d.State == nil || d.Requests == nil || d.Store == nil || d.Objects == nil || d.Messages == nil || d.Files == nil ||
		d.People == nil || d.Eraser == nil || d.Floor == nil {
		return nil, errors.New("privacy: the deletion journal needs every collaborator")
	}
	if !d.DeletionContent.Valid() {
		return nil, errors.New("privacy: privacy.account_deletion_content is invalid")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Journal{d: d}, nil
}

// ErrJournalConflict reports an object at the number the consumer writes that holds another
// entry: the numbering of deletion_journal_state and the objects disagree (a restore that was not
// finalized). The consumer stalls on it instead of overwriting an entry; the alarm of the
// consumer lag reports it.
var ErrJournalConflict = errs.New(errs.CodeInternalError, "privacy: a deletion journal object with another entry exists at the next number")

// HandleEvent is the handler of the consumer privacy.journal (sync.ConsumerHandler, adapted in
// internal/wire): it runs inside the transaction of the round, in seq order. For message.deleted
// and for user.updated of a person whose status became deleted it takes the next number, and
// stores the entry as the object of that number unless an object with the same bytes is there
// already (a round that stored it and then failed before its commit); an account entry also marks
// the deletion request journaled, so retention.privacy_job keeps the request until then (design
// note 1). Every other event moves the cursor without effect.
//
// The object is written inside the transaction of the round: the row lock of the cursor keeps one
// writer, and next_number and the cursor move only after the object is stored (03 4.12, R-DM-52).
// This is the documented exception to R-AR-07 for the journal (docs/decisions.d/M7-07.md).
//
// Errors (the round rolls back and is tried again): ErrJournalConflict, a malformed payload, the
// storage and the database.
func (j *Journal) HandleEvent(ctx context.Context, ev JournalEvent) error {
	e, ok, err := j.entryOf(ctx, ev)
	if err != nil || !ok {
		return err
	}
	body, err := e.Marshal()
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "privacy: a deletion journal entry")
	}
	n, err := j.d.State.Take(ctx)
	if err != nil {
		return err
	}
	key, err := domain.JournalKey(n)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "privacy: deletion_journal_state.next_number")
	}
	existing, found, err := j.d.Objects.Get(ctx, key)
	if err != nil {
		return err
	}
	if found {
		if bytes.Equal(existing, body) {
			return nil
		}
		return ErrJournalConflict
	}
	return j.d.Objects.Put(ctx, key, body)
}

// entryOf returns the entry of ev; ok is false for an event the journal does not record.
func (j *Journal) entryOf(ctx context.Context, ev JournalEvent) (domain.Entry, bool, error) {
	switch ev.Type {
	case eventMessageDeleted:
		var p struct {
			MessageID ids.ID    `json:"message_id"`
			DeletedAt time.Time `json:"deleted_at"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil || p.MessageID.IsZero() {
			return domain.Entry{}, false, errs.New(errs.CodeInternalError, fmt.Sprintf("privacy: the payload of message.deleted at seq %d", ev.Seq))
		}
		at := p.DeletedAt
		if at.IsZero() {
			at = ev.OccurredAt
		}
		return domain.Entry{ResourceKind: domain.ResourceMessage, ResourceID: p.MessageID, DeletedAt: at, Reason: domain.ReasonDeleted, Seq: ev.Seq}, true, nil
	case eventUserUpdated:
		var p struct {
			User struct {
				ID     ids.ID `json:"id"`
				Status string `json:"status"`
			} `json:"user"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil || p.User.ID.IsZero() {
			return domain.Entry{}, false, errs.New(errs.CodeInternalError, fmt.Sprintf("privacy: the payload of user.updated at seq %d", ev.Seq))
		}
		if p.User.Status != string(identity.StatusDeleted) {
			return domain.Entry{}, false, nil
		}
		mode, err := j.contentMode(ctx, p.User.ID)
		if err != nil {
			return domain.Entry{}, false, err
		}
		return domain.Entry{ResourceKind: domain.ResourceAccount, ResourceID: p.User.ID, DeletedAt: ev.OccurredAt, Reason: string(mode), Seq: ev.Seq}, true, nil
	default:
		return domain.Entry{}, false, nil
	}
}

// contentMode returns the content mode the deletion of subject used and marks its request
// journaled in the round's transaction; without a request it is privacy.account_deletion_content.
func (j *Journal) contentMode(ctx context.Context, subject ids.ID) (personaldata.ContentMode, error) {
	r, found, err := j.d.Requests.LockDeletionOf(ctx, subject)
	if err != nil {
		return "", err
	}
	if !found {
		return j.d.DeletionContent, nil
	}
	mode := personaldata.ContentMode(r.Progress.Content)
	if !mode.Valid() {
		mode = j.d.DeletionContent
	}
	if !r.Progress.Journaled {
		r.Progress.Journaled = true
		if err := j.d.Store.Update(ctx, r); err != nil {
			return "", err
		}
	}
	return mode, nil
}

// ApplyResult is what ApplyJournal did: counts per kind, never IDs.
type ApplyResult struct {
	// Entries is the number of journal objects read.
	Entries int
	// Messages, Files and Accounts count the resources deleted again.
	Messages, Files, Accounts int
	// Skipped counts entries whose resource does not exist in the restored database or is deleted
	// there already.
	Skipped int
	// Refused counts accounts whose deletion step 1 refused, because the person is the last active
	// owner of the restored database (the runbook names the way out).
	Refused int
	// Next is the number after the last object found: deletion_journal_state.next_number after the
	// restore.
	Next int64
}

// ApplyJournal applies the deletion journal after a restore (SEC-BAK-3, brief M7-07 design notes 2
// and 3): it reads the objects from the restored next_number up to the first missing number, in
// number order, and deletes again every resource that exists in the restored database and is not
// deleted there: a message through the message deletion of 20 4.2, a file through the file
// deletion of 20 4.3, an account through steps 1 to 6 of 20 4.4 with the recorded content mode.
// Each deletion runs its own units of work; a second run finds everything deleted and changes
// nothing. It stores nothing in deletion_journal_state: bothy restore finalize stores Next in the
// unit of work that sets the new sync epoch (StoreNext). It runs outside a unit of work, because
// it reads the object storage (R-AR-07).
//
// Errors: an object that is no entry (the number is named), and those of the storage, the
// database and the deletions; nothing of the epoch step has run then.
func (j *Journal) ApplyJournal(ctx context.Context) (ApplyResult, error) {
	next, _, err := j.d.State.Numbers(ctx)
	if err != nil {
		return ApplyResult{}, err
	}
	res := ApplyResult{}
	for n := next; ; n++ {
		key, err := domain.JournalKey(n)
		if err != nil {
			return ApplyResult{}, errs.Wrap(errs.CodeInternalError, err, "privacy: deletion_journal_state.next_number")
		}
		body, found, err := j.d.Objects.Get(ctx, key)
		if err != nil {
			return ApplyResult{}, err
		}
		if !found {
			res.Next = n
			return res, nil
		}
		e, err := domain.ParseEntry(body)
		if err != nil {
			return ApplyResult{}, errs.Wrap(errs.CodeInternalError, err, fmt.Sprintf("privacy: deletion journal object %d", n))
		}
		res.Entries++
		if err := j.apply(ctx, e, &res); err != nil {
			return ApplyResult{}, err
		}
	}
}

// apply deletes the resource of e again when the restored database holds it and counts the outcome.
func (j *Journal) apply(ctx context.Context, e domain.Entry, res *ApplyResult) error {
	switch e.ResourceKind {
	case domain.ResourceMessage:
		done, err := j.d.Messages.DeleteForJournal(ctx, e.ResourceID)
		count(done, &res.Messages, &res.Skipped)
		return err
	case domain.ResourceFile:
		done, err := j.d.Files.DeleteForJournal(ctx, e.ResourceID)
		count(done, &res.Files, &res.Skipped)
		return err
	default:
		return j.applyAccount(ctx, e, res)
	}
}

// applyAccount runs the account deletion for a person of the restored database who is not deleted.
func (j *Journal) applyAccount(ctx context.Context, e domain.Entry, res *ApplyResult) error {
	p, err := j.d.People.Principal(ctx, e.ResourceID)
	if ee, ok := errs.As(err); ok && ee.Code == errs.CodeNotFound {
		res.Skipped++
		return nil
	}
	if err != nil {
		return err
	}
	if p.Kind != actor.KindUser || p.Status == identity.StatusDeleted {
		res.Skipped++
		return nil
	}
	refused, err := j.d.Eraser.EraseForJournal(ctx, e.ResourceID, personaldata.ContentMode(e.Reason))
	if err != nil {
		return err
	}
	if refused {
		res.Refused++
		return nil
	}
	res.Accounts++
	return nil
}

// count adds one to done or to skipped.
func count(ok bool, done, skipped *int) {
	if ok {
		*done++
		return
	}
	*skipped++
}

// StoreNext stores next as deletion_journal_state.next_number in the unit of work of ctx: bothy
// restore finalize calls it in the unit that sets the new sync epoch, with ApplyResult.Next, so
// the consumer never writes a key that exists (R-DM-52).
func (j *Journal) StoreNext(ctx context.Context, next int64) error {
	if next < 1 {
		return errs.New(errs.CodeInternalError, "privacy: deletion journal numbers start at 1")
	}
	return j.d.State.SetNext(ctx, next)
}

// Retention is the participant of the deletion journal in the job retention.privacy_job (brief
// M7-07 requirement 16): it removes journal objects older than privacy.deletion_journal_retention
// from low_number upward and moves low_number after each, and stops at the first object that is
// younger, at next_number, and at the smallest deletion_journal.next_number of the manifests of
// the backups that are still kept, so a restore of any kept backup finds its first object (03
// 4.12). Nothing else deletes journal objects.
func (j *Journal) Retention() *JournalRetention { return &JournalRetention{j: j} }

// JournalRetention implements retention.Participant for the journal objects.
type JournalRetention struct {
	j *Journal
}

// Compile-time check.
var _ retention.Participant = (*JournalRetention)(nil)

// Class implements retention.Participant: the job retention.privacy_job (20 4.1).
func (*JournalRetention) Class() retention.Class { return retention.ClassPrivacyJob }

// Tables implements retention.Participant: the participant removes objects, no rows; it moves
// deletion_journal_state.low_number, whose own class is config.
func (*JournalRetention) Tables() []string { return nil }

// Purge implements retention.Participant: at most batch objects whose deleted_at lies before cutoff
// (now minus privacy.deletion_journal_retention), each object deleted outside a transaction and
// then low_number moved in its own statement; a crash between the two finds the object missing
// next time and only moves low_number. An object that is no entry stops the run with an error.
func (r *JournalRetention) Purge(ctx context.Context, cutoff time.Time, batch int) (int, bool, error) {
	next, low, err := r.j.d.State.Numbers(ctx)
	if err != nil {
		return 0, false, err
	}
	floor, limited, err := r.j.d.Floor.JournalFloor(ctx)
	if err != nil {
		return 0, false, err
	}
	stop := next
	if limited && floor < stop {
		stop = floor
	}
	removed := 0
	for ; low < stop && removed < batch; low++ {
		gone, err := r.removeIfOld(ctx, low, cutoff)
		if err != nil || !gone {
			return removed, false, err
		}
		if err := r.j.d.State.SetLow(ctx, low+1); err != nil {
			return removed, false, err
		}
		removed++
	}
	return removed, removed == batch && low < stop, nil
}

// removeIfOld deletes object n when it is older than cutoff or missing already; gone is false for
// a younger object, where the run stops.
func (r *JournalRetention) removeIfOld(ctx context.Context, n int64, cutoff time.Time) (bool, error) {
	key, err := domain.JournalKey(n)
	if err != nil {
		return false, errs.Wrap(errs.CodeInternalError, err, "privacy: deletion_journal_state.low_number")
	}
	body, found, err := r.j.d.Objects.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil
	}
	e, err := domain.ParseEntry(body)
	if err != nil {
		return false, errs.Wrap(errs.CodeInternalError, err, fmt.Sprintf("privacy: deletion journal object %d", n))
	}
	if !e.DeletedAt.Before(cutoff) {
		return false, nil
	}
	return true, r.j.d.Objects.Delete(ctx, key)
}
