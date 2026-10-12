// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Store is the storage of the audit module (adapters/postgres). Every method runs in the
// ambient transaction of ctx when there is one, otherwise on the pool. Errors are those of the
// database.
type Store interface {
	// InsertRows inserts rows into audit_events in one statement, in the order given.
	InsertRows(ctx context.Context, rows []domain.Row) error
	// HashAt returns the hash of row seq and true, or false when the row is missing.
	HashAt(ctx context.Context, seq int64) ([domain.HashSize]byte, bool, error)
	// CurrentIPKey returns the row of audit_ip_keys whose period contains at, and false when
	// there is none.
	CurrentIPKey(ctx context.Context, at time.Time) (domain.IPKey, bool, error)
	// InsertIPKey inserts k unless a row with the same valid_from exists
	// (ON CONFLICT (valid_from) DO NOTHING).
	InsertIPKey(ctx context.Context, k domain.IPKey) error
	// IPKeyByID returns the row id of audit_ip_keys and true, or false when it is missing.
	IPKeyByID(ctx context.Context, id ids.ID) (domain.IPKey, bool, error)
}

// KEKSource returns the raw content of the secret behind a reference: the key-encryption key of
// the IP keys (privacy.audit_ip_kek_ref, 64 hex characters). internal/wire implements it with the
// secret provider; app never sees the provider.
type KEKSource interface {
	// KEK returns the secret behind ref. The error never contains the secret.
	KEK(ctx context.Context, ref string) ([]byte, error)
}

// Reporter receives what the module must report but cannot return (02 "14.1": details only in
// the log, with the correlation ID of ctx). adapters/logs implements it with the server logger;
// no report ever carries an audit row, an IP address or a name (17 rule 6, SEC-LOG-4).
type Reporter interface {
	// ChainBroken reports that row missingSeq, the predecessor of a new row, is missing.
	ChainBroken(ctx context.Context, missingSeq int64)
	// UnknownReason reports a reason code outside the list of 07 "4.1"; the entry is written.
	UnknownReason(ctx context.Context, action, reason string)
	// DenialsDropped reports count aggregated denials that were dropped because the cap of
	// audit.deny_aggregate_max_keys was reached and their row could not be written.
	DenialsDropped(ctx context.Context, action, reason string, count int64)
	// WriteFailed reports a failed write of aggregated denials; the buckets stay for a retry.
	WriteFailed(ctx context.Context, err error)
	// IPKeyFailed reports that the IP pseudonym of a denial could not be computed; the denial is
	// aggregated without IP fields.
	IPKeyFailed(ctx context.Context, err error)
}
