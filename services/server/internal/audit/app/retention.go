// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
)

// MaintUnits runs units of work on a connection of the database role bothy_maint (16 7.1) whose
// registry holds Chain.Hook in the audit phase: the trim writes its anchor entry and removes the
// trimmed rows in one transaction. internal/wire opens the connection for each unit and closes it
// afterwards, so the role is connected only for the trim.
type MaintUnits interface {
	// Do runs fn in one unit of work as bothy_maint. Errors: the connection and those of fn.
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// Trim is the participant of audit in the job retention.audit that trims the log (03
// audit_events, 20 section 4.5, brief M7-05 design note 4). In one transaction as bothy_maint it
// takes the checkpoint lock, finds the prefix of the log from the first retained row whose rows
// occurred before the cutoff, writes the anchor entry (audit.retention_trim with the last removed
// seq and hash) through the recorder, deletes that prefix and the checkpoints up to it. It never
// removes the head row (the anchor links to it) nor the row of the newest checkpoint the sink
// holds or a row after it: audit.chain_verify continues from its checkpoints, no row leaves before
// a checkpoint outside the database sealed it, and without audit.checkpoint_sink (nothing is
// exported) nothing is trimmed. audit.verify and the restore test start at the anchor.
type Trim struct {
	units MaintUnits
	store RetentionStore
	rec   auditing.Recorder
}

// Compile-time check: Trim is a participant of kernel/retention.
var _ retention.Participant = (*Trim)(nil)

// NewTrim returns the trim over units, the store whose statements join the ambient unit, and the
// recorder whose Record adds the anchor entry to that unit. Errors: a nil dependency.
func NewTrim(units MaintUnits, store RetentionStore, rec auditing.Recorder) (*Trim, error) {
	if units == nil || store == nil || rec == nil {
		return nil, errors.New("audit: the trim needs maintenance units, a store and a recorder")
	}
	return &Trim{units: units, store: store, rec: rec}, nil
}

// Class returns retention.ClassAudit.
func (*Trim) Class() retention.Class { return retention.ClassAudit }

// Tables returns audit_events and audit_checkpoints.
func (*Trim) Tables() []string { return []string{"audit_events", "audit_checkpoints"} }

// Purge implements retention.Participant: one trim of at most batch rows. more is true when the
// whole batch was removed. Errors: the connection, the storage and the recorder.
func (t *Trim) Purge(ctx context.Context, cutoff time.Time, batch int) (int, bool, error) {
	var removed int64
	var more bool
	err := t.units.Do(ctx, func(ctx context.Context) error {
		removed, more = 0, false
		last, ok, err := t.end(ctx, cutoff, batch)
		if err != nil || !ok {
			return err
		}
		hash, found, err := t.store.HashAt(ctx, last.to)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("audit: row %d of the trim is missing", last.to)
		}
		if err := t.rec.Record(ctx, auditing.Entry{
			Action: domain.ActionRetentionTrim, Decision: auditing.DecisionAllow,
			Attrs: auditing.Attrs{
				domain.DetailAnchorSeq: auditing.Int(last.to), domain.DetailAnchorHash: auditing.Code(domain.HashText(hash)),
			},
		}); err != nil {
			return err
		}
		if removed, err = t.store.TrimEvents(ctx, last.from, last.to); err != nil {
			return err
		}
		if _, err := t.store.TrimCheckpoints(ctx, last.to); err != nil {
			return err
		}
		more = last.full
		return nil
	})
	if err != nil {
		return 0, false, err
	}
	return int(removed), more, nil
}

// trimRange is the prefix one trim removes.
type trimRange struct {
	from, to int64
	// full reports that the batch bounded the prefix, not the cutoff, the head or a checkpoint.
	full bool
}

// end finds the prefix: from the first retained row up to the last row before the first one that
// occurred at or after the cutoff, at most batch rows, below the head and below the newest
// exported checkpoint. false when there is nothing to trim. It takes the checkpoint lock first, so no
// checkpoint is signed over rows the trim is removing.
func (t *Trim) end(ctx context.Context, cutoff time.Time, batch int) (trimRange, bool, error) {
	if err := t.store.LockCheckpoints(ctx); err != nil {
		return trimRange{}, false, err
	}
	first, head, ok, err := t.store.Bounds(ctx)
	if err != nil || !ok {
		return trimRange{}, false, err
	}
	cp, hasCP, err := t.store.LastExportedCheckpoint(ctx)
	if err != nil || !hasCP {
		return trimRange{}, false, err
	}
	byBatch := first + int64(batch) - 1
	upper := min(head-1, cp.Seq-1, byBatch)
	if upper < first {
		return trimRange{}, false, nil
	}
	rows, err := t.store.TrimCandidates(ctx, first, upper)
	if err != nil {
		return trimRange{}, false, err
	}
	to := first - 1
	for _, r := range rows {
		if r.Seq != to+1 || !r.OccurredAt.Before(cutoff) {
			break
		}
		to = r.Seq
	}
	if to < first {
		return trimRange{}, false, nil
	}
	return trimRange{from: first, to: to, full: to == byBatch}, true, nil
}

// IPKeys is the participant of audit in the job retention.audit that destroys the IP keys after
// privacy.audit_ip_key_retention (20 section 4.5, 03 audit_ip_keys, DP-T06): a key whose
// valid_until lies before the cutoff is emptied and its destroyed_at set; the row stays as
// evidence. After that no ip_hmac of its period can be recomputed for any IP; the chain is
// unchanged.
type IPKeys struct {
	store RetentionStore
	clock clock.Clock
}

// Compile-time check: IPKeys is a participant of kernel/retention.
var _ retention.Participant = (*IPKeys)(nil)

// NewIPKeys returns the participant over store; c nil means clock.System().
func NewIPKeys(store RetentionStore, c clock.Clock) *IPKeys {
	if c == nil {
		c = clock.System()
	}
	return &IPKeys{store: store, clock: c}
}

// Class returns retention.ClassAudit.
func (*IPKeys) Class() retention.Class { return retention.ClassAudit }

// Tables returns audit_ip_keys.
func (*IPKeys) Tables() []string { return []string{"audit_ip_keys"} }

// Purge implements retention.Participant: destroys at most batch keys in one statement. Errors:
// those of the storage.
func (k *IPKeys) Purge(ctx context.Context, cutoff time.Time, batch int) (int, bool, error) {
	n, err := k.store.DestroyIPKeys(ctx, cutoff, k.clock.Now(), batch)
	if err != nil {
		return 0, false, err
	}
	return int(n), n >= int64(batch), nil
}
