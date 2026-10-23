// SPDX-License-Identifier: Apache-2.0

// Package veto holds the in-transaction test subscribers of the attachment tests (brief M3-07):
// armed once, a subscriber fails the unit of work it runs in, so the test proves that every write
// of that unit rolls back together. The package path ends in internal/veto, so the txn registry
// derives the module "veto" from the method values (txn.ModuleOfPackage) and the golden list of
// production registrations stays untouched; the harness registers it after every production
// subscriber (harness.Options.TxnTestSubscribers).
package veto

import (
	"context"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// Veto fails the next unit of work that publishes an armed event; the zero value vetoes nothing.
// It is safe for concurrent use.
type Veto struct {
	deleted atomic.Bool
	moved   atomic.Bool
	// hits counts the vetoes that fired.
	hits atomic.Int64
}

// Register subscribes OnMessageDeleted and OnThreadMoved of v to reg; pass it as
// harness.Options.TxnTestSubscribers. The error is a rejected registration.
func (v *Veto) Register(reg *txn.Registry) error {
	if err := txn.Subscribe(reg, v.OnMessageDeleted); err != nil {
		return err
	}
	return txn.Subscribe(reg, v.OnThreadMoved)
}

// ArmMessageDeleted makes the next messaging.MessageDeleted fail its unit of work.
func (v *Veto) ArmMessageDeleted() { v.deleted.Store(true) }

// ArmThreadMoved makes the next chat.ThreadMoved fail its unit of work; it runs after
// files.OnThreadMoved, so the copies of message_attachments were already written in the unit.
func (v *Veto) ArmThreadMoved() { v.moved.Store(true) }

// Hits returns how many units the veto failed.
func (v *Veto) Hits() int64 { return v.hits.Load() }

// OnMessageDeleted fails the unit once after ArmMessageDeleted.
func (v *Veto) OnMessageDeleted(_ context.Context, _ messaging.MessageDeleted) error {
	return v.fire(&v.deleted)
}

// OnThreadMoved fails the unit once after ArmThreadMoved.
func (v *Veto) OnThreadMoved(_ context.Context, _ chat.ThreadMoved) error {
	return v.fire(&v.moved)
}

// fire disarms flag and returns the veto error when it was armed. A conflict is not retried by
// the unit of work, so the request answers it once and nothing of the unit is stored.
func (v *Veto) fire(flag *atomic.Bool) error {
	if !flag.CompareAndSwap(true, false) {
		return nil
	}
	v.hits.Add(1)
	return errs.New(errs.CodeConflict, "vetoed by the test")
}
