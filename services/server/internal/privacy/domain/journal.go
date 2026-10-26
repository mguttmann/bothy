// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ResourceKind is the kind of the deleted resource of a journal entry (docs/spec/20-privacy.md
// section 4.6: message, file, account).
type ResourceKind string

// The resource kinds of the deletion journal.
const (
	// ResourceMessage is a message deleted as in 20 section 4.2.
	ResourceMessage ResourceKind = "message"
	// ResourceFile is a file deleted as in 20 section 4.3.
	ResourceFile ResourceKind = "file"
	// ResourceAccount is an account deleted as in 20 section 4.4; the reason is the content mode.
	ResourceAccount ResourceKind = "account"
)

// ReasonDeleted is the reason of a message or file entry: the resource was deleted. An account
// entry carries the content mode of its deletion instead.
const ReasonDeleted = "deleted"

// The content modes of privacy.account_deletion_content (20 section 4.4 item 5), the reasons of
// an account entry. kernel/personaldata names the same values; the domain layer imports no port.
const (
	// ReasonKeepPseudonymized: contents stayed, the author became "Deleted person".
	ReasonKeepPseudonymized = "keep_pseudonymized"
	// ReasonDeleteContent: the own messages and files were deleted too.
	ReasonDeleteContent = "delete"
)

// JournalKeyPrefix is the prefix of every journal object in the object storage. The key schema of
// files (12 4.4) never produces it, and its characters are those domain.ValidateKey of files
// allows; the files port refuses every other key for the journal.
const JournalKeyPrefix = "deletion-journal/"

// journalNumberDigits is the width of the number in a key: the 19 digits of the largest int64, so
// the keys sort like their numbers.
const journalNumberDigits = 19

// ErrInvalidEntry reports an entry outside the rules of Entry.Validate or an object that is no
// journal entry. Its messages name the field, never a value.
var ErrInvalidEntry = errors.New("privacy: invalid deletion journal entry")

// Entry is one entry of the deletion journal (20 section 4.6, brief M7-07 requirement 12): the
// kind and ID of the deleted resource, the time of the deletion, a reason code and the seq of the
// event it came from. It never holds content, names or e-mail addresses.
type Entry struct {
	// ResourceKind is message, file or account.
	ResourceKind ResourceKind `json:"resource_kind"`
	// ResourceID is the ID of the deleted resource (for an account the principal).
	ResourceID ids.ID `json:"resource_id"`
	// DeletedAt is the time of the deletion in UTC.
	DeletedAt time.Time `json:"deleted_at"`
	// Reason is ReasonDeleted for a message or a file and the content mode for an account.
	Reason string `json:"reason"`
	// Seq is the global sequence of the event the entry came from.
	Seq int64 `json:"seq"`
}

// Validate checks the rules of an entry: a known kind, a resource ID, a deletion time, a seq
// above zero and the reason of its kind. Errors wrap ErrInvalidEntry and name the field.
func (e Entry) Validate() error {
	switch {
	case e.ResourceKind != ResourceMessage && e.ResourceKind != ResourceFile && e.ResourceKind != ResourceAccount:
		return fmt.Errorf("%w: resource_kind must be message, file or account", ErrInvalidEntry)
	case e.ResourceID.IsZero():
		return fmt.Errorf("%w: resource_id must be set", ErrInvalidEntry)
	case e.DeletedAt.IsZero():
		return fmt.Errorf("%w: deleted_at must be set", ErrInvalidEntry)
	case e.Seq < 1:
		return fmt.Errorf("%w: seq must be at least 1", ErrInvalidEntry)
	}
	if e.ResourceKind == ResourceAccount {
		if e.Reason != ReasonKeepPseudonymized && e.Reason != ReasonDeleteContent {
			return fmt.Errorf("%w: reason of an account must be a content mode", ErrInvalidEntry)
		}
		return nil
	}
	if e.Reason != ReasonDeleted {
		return fmt.Errorf("%w: reason of a message or a file must be %s", ErrInvalidEntry, ReasonDeleted)
	}
	return nil
}

// Marshal validates e and returns the bytes of its journal object: the JSON object with exactly
// the five members, deleted_at in UTC. The same entry always gives the same bytes, so a retry
// that finds the object of an earlier attempt can compare them.
//
// Example:
//
//	b, err := domain.Entry{ResourceKind: domain.ResourceMessage, ResourceID: id, DeletedAt: at,
//		Reason: domain.ReasonDeleted, Seq: 42}.Marshal()
func (e Entry) Marshal() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	e.DeletedAt = e.DeletedAt.UTC()
	return json.Marshal(e)
}

// ParseEntry decodes a journal object strictly (an unknown member, a second value or a wrong type
// fails) and validates it. Errors wrap ErrInvalidEntry and never repeat the bytes.
func ParseEntry(b []byte) (Entry, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var e Entry
	if err := dec.Decode(&e); err != nil {
		return Entry{}, fmt.Errorf("%w: the object is no entry", ErrInvalidEntry)
	}
	if dec.More() {
		return Entry{}, fmt.Errorf("%w: trailing data after the entry", ErrInvalidEntry)
	}
	if err := e.Validate(); err != nil {
		return Entry{}, err
	}
	return e, nil
}

// Placeholder is the object of a journal number whose entry was lost (M7-07, red team finding:
// after a host loss the objects written after the last object backup are missing). bothy restore
// finalize writes one for every missing number below the restored next_number, so the numbers
// stay dense and a later application reads past the lost range; it holds only the time it was
// written, which the retention compares like deleted_at.
type Placeholder struct {
	// Lost is always true; it tells the object apart from an entry.
	Lost bool `json:"lost"`
	// WrittenAt is the time the placeholder was written, in UTC.
	WrittenAt time.Time `json:"written_at"`
}

// MarshalPlaceholder returns the bytes of the placeholder written at at.
func MarshalPlaceholder(at time.Time) ([]byte, error) {
	if at.IsZero() {
		return nil, fmt.Errorf("%w: a placeholder needs its time", ErrInvalidEntry)
	}
	return json.Marshal(Placeholder{Lost: true, WrittenAt: at.UTC()})
}

// ParseObject decodes a journal object: an entry (ParseEntry), or a placeholder, then lost is
// true and e holds only DeletedAt (the time the placeholder was written). Errors wrap
// ErrInvalidEntry.
func ParseObject(b []byte) (e Entry, lost bool, err error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var p Placeholder
	if err := dec.Decode(&p); err == nil && !dec.More() && p.Lost && !p.WrittenAt.IsZero() {
		return Entry{DeletedAt: p.WrittenAt}, true, nil
	}
	e, err = ParseEntry(b)
	return e, false, err
}

// JournalKey returns the object key of the journal object number n: JournalKeyPrefix and n with
// leading zeros to 19 digits. Numbers start at 1 (deletion_journal_state.next_number, R-DM-52).
//
// Errors wrap ErrInvalidEntry for n below 1.
//
// Example:
//
//	k, _ := domain.JournalKey(7) // "deletion-journal/0000000000000000007"
func JournalKey(n int64) (string, error) {
	if n < 1 {
		return "", fmt.Errorf("%w: journal numbers start at 1", ErrInvalidEntry)
	}
	s := strconv.FormatInt(n, 10)
	for len(s) < journalNumberDigits {
		s = "0" + s
	}
	return JournalKeyPrefix + s, nil
}

// BackupPeriods are the periods for which a backup can still be restored (docs/spec/16-operations.md
// 8.1): the retention of the logical backup and of the object backup, and the pgBackRest
// retention, which is a count of full backups.
type BackupPeriods struct {
	// Logical is backup.logical.retention.
	Logical time.Duration
	// Objects is backup.objects.retention.
	Objects time.Duration
	// FullCount is the number of full backups pgBackRest keeps (the larger of
	// backup.pg.retention_full and backup.pg.local_retention_full).
	FullCount int
	// FullInterval is the longest interval between two runs of backup.pg.full_schedule.
	FullInterval time.Duration
}

// day is the unit the pgBackRest period is rounded up to.
const day = 24 * time.Hour

// FullBackupPeriod returns the pgBackRest retention as a period (brief M7-07 design note 4): the
// count of full backups times the interval of the full schedule, rounded up to whole days. A
// count or interval below one gives 0.
func FullBackupPeriod(count int, interval time.Duration) time.Duration {
	if count < 1 || interval <= 0 {
		return 0
	}
	p := time.Duration(count) * interval
	if r := p % day; r != 0 {
		p += day - r
	}
	return p
}

// Longest returns the longest of the three backup periods.
func (p BackupPeriods) Longest() time.Duration {
	return max(p.Logical, p.Objects, FullBackupPeriod(p.FullCount, p.FullInterval))
}

// JournalRetentionCovers reports whether the retention of the deletion journal
// (privacy.deletion_journal_retention) is at least the longest backup period (DP-T08): otherwise
// a restore of a kept backup could miss journal objects and bring back deleted data.
func JournalRetentionCovers(journal time.Duration, p BackupPeriods) bool {
	return journal >= p.Longest()
}
