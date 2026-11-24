// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Keyring hands out the IP pseudonym key of the current period (20 "Audit log and IP addresses"):
// one row of audit_ip_keys per privacy.audit_ip_key_period, created on first need and cached
// decrypted in memory until its valid_until. It touches only PostgreSQL (02 "8.5") and always in
// its own short transactions, never in the caller's: a key row that rolled back with a business
// transaction would stay cached while missing from the table. It is safe for concurrent use.
type Keyring struct {
	store  Store
	uow    txn.UnitOfWork
	kek    KEKSource
	ref    string
	period time.Duration
	random io.Reader

	mu  sync.Mutex
	cur *cachedKey
}

// cachedKey is the decrypted key of one period.
type cachedKey struct {
	id          ids.ID
	key         []byte
	from, until time.Time
}

// NewKeyring returns the keyring for privacy.audit_ip_kek_ref ref and the period. It does no I/O.
func NewKeyring(store Store, uow txn.UnitOfWork, kek KEKSource, ref string, period time.Duration) *Keyring {
	return &Keyring{store: store, uow: uow, kek: kek, ref: ref, period: period, random: rand.Reader}
}

// errDestroyed reports a key of the current period that was already destroyed.
var errDestroyed = errors.New("audit: the IP key of the current period is destroyed")

// Current returns the id and the key of the period that contains now. On a cache miss it reads
// the row covering now or, when there is none, creates one: 32 random bytes from crypto/rand,
// encrypted with AES-256-GCM under the key behind privacy.audit_ip_kek_ref (row id as additional
// data), valid_from aligned to the period in Unix time and moved past older keys that cover it
// (periodStart), inserted with ON CONFLICT (valid_from) DO NOTHING and read again, so concurrent
// processes agree on one key. A destroyed key is never
// used.
//
// Errors (kind internal): the database errors, a destroyed key, a key-encryption key that does not
// resolve, is not 32 bytes of hex or does not decrypt the row. No error carries key material.
func (k *Keyring) Current(ctx context.Context, now time.Time) (ids.ID, []byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if c := k.cur; c != nil && !now.Before(c.from) && now.Before(c.until) {
		return c.id, c.key, nil
	}
	ctx = txn.Detach(ctx)
	row, ok, err := k.store.CurrentIPKey(ctx, now)
	if err != nil {
		return ids.ID{}, nil, errs.Wrap(errs.CodeInternalError, err, "audit: read the IP key")
	}
	if !ok {
		if row, err = k.create(ctx, now); err != nil {
			return ids.ID{}, nil, err
		}
	}
	key, err := k.open(ctx, row)
	if err != nil {
		return ids.ID{}, nil, err
	}
	k.cur = &cachedKey{id: row.ID, key: key, from: row.ValidFrom, until: row.ValidUntil}
	return row.ID, key, nil
}

// create inserts the key of the period of now and reads the row that won.
func (k *Keyring) create(ctx context.Context, now time.Time) (domain.IPKey, error) {
	kek, err := k.kekFor(ctx, k.ref)
	if err != nil {
		return domain.IPKey{}, err
	}
	key := make([]byte, domain.IPKeyBytes)
	if _, err := io.ReadFull(k.random, key); err != nil {
		return domain.IPKey{}, internalError(err)
	}
	from, until, err := k.periodStart(ctx, now)
	if err != nil {
		return domain.IPKey{}, err
	}
	row := domain.IPKey{ID: ids.New(), ValidFrom: from, ValidUntil: until, KeyRef: k.ref}
	if row.Ciphertext, err = domain.SealIPKey(kek, key, row.ID, k.random); err != nil {
		return domain.IPKey{}, internalError(err)
	}
	if err := k.uow.Do(ctx, func(ctx context.Context) error { return k.store.InsertIPKey(ctx, row) }); err != nil {
		return domain.IPKey{}, errs.Wrap(errs.CodeInternalError, err, "audit: insert the IP key")
	}
	got, ok, err := k.store.CurrentIPKey(ctx, now)
	if err != nil {
		return domain.IPKey{}, errs.Wrap(errs.CodeInternalError, err, "audit: read the IP key")
	}
	if !ok {
		return domain.IPKey{}, errs.New(errs.CodeInternalError, "audit: no IP key covers now after the insert")
	}
	return got, nil
}

// periodStart returns the period of the new key for now: the period of privacy.audit_ip_key_period
// that contains now, aligned in Unix time, its start moved past every key that covers it. Keys
// end at or before now when none covers now, so the start stays at or before now; after the period
// grew (30 to 90 days) the aligned start can hold an expired key of the old period, whose
// valid_from would make the insert a no-op. Every process computes the same start from the same
// rows, so ON CONFLICT (valid_from) still lets one of them win.
func (k *Keyring) periodStart(ctx context.Context, now time.Time) (time.Time, time.Time, error) {
	from, until := domain.KeyPeriod(now, k.period)
	for {
		row, ok, err := k.store.CurrentIPKey(ctx, from)
		if err != nil {
			return time.Time{}, time.Time{}, errs.Wrap(errs.CodeInternalError, err, "audit: read the IP key")
		}
		if !ok || !row.ValidUntil.After(from) || row.ValidUntil.After(now) {
			return from, until, nil
		}
		from = row.ValidUntil
	}
}

// open decrypts row with the key-encryption key of its key_ref.
func (k *Keyring) open(ctx context.Context, row domain.IPKey) ([]byte, error) {
	if row.Destroyed() {
		return nil, internalError(errDestroyed)
	}
	kek, err := k.kekFor(ctx, row.KeyRef)
	if err != nil {
		return nil, err
	}
	key, err := domain.OpenIPKey(kek, row.Ciphertext, row.ID)
	return key, internalError(err)
}

// kekFor resolves and decodes the key-encryption key behind ref.
func (k *Keyring) kekFor(ctx context.Context, ref string) ([]byte, error) {
	raw, err := k.kek.KEK(ctx, ref)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "audit: resolve privacy.audit_ip_kek_ref")
	}
	kek, err := domain.DecodeKEK(raw)
	return kek, internalError(err)
}
