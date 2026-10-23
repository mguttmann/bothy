// SPDX-License-Identifier: Apache-2.0

// Package finalveto holds the in-transaction test subscriber of the hold tests (brief M3-07,
// TestHold_SameTransaction): armed for one file, it fails the unit of work that publishes
// files.FileFinalized of that file once, so the test proves that the final state of the file, its
// file.updated and the released agent.invocation roll back together. The package path ends in
// internal/finalveto, so the txn registry derives the module "finalveto" from the method value
// (txn.ModuleOfPackage) and the golden list of production registrations stays untouched; the
// harness registers it after every production subscriber (harness.Options.TxnTestSubscribers),
// that is after runs.OnFileFinalized.
package finalveto

import (
	"context"
	"sync"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Veto fails the next unit of work that publishes files.FileFinalized of the armed file; the zero
// value vetoes nothing and only counts. It is safe for concurrent use.
type Veto struct {
	mu    sync.Mutex
	armed ids.ID
	// seen counts every FileFinalized the subscriber saw, per file, also vetoed ones.
	seen map[ids.ID]int
	hits int
}

// Register subscribes OnFileFinalized of v to reg; pass it as
// harness.Options.TxnTestSubscribers. The error is a rejected registration.
func (v *Veto) Register(reg *txn.Registry) error {
	return txn.Subscribe(reg, v.OnFileFinalized)
}

// Arm makes the next files.FileFinalized of fileID fail its unit of work.
func (v *Veto) Arm(fileID ids.ID) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.armed = fileID
}

// Hits returns how many units the veto failed.
func (v *Veto) Hits() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.hits
}

// Seen returns how often files.FileFinalized of fileID was published, committed or not.
func (v *Veto) Seen(fileID ids.ID) int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.seen[fileID]
}

// OnFileFinalized counts the event and fails the unit once for the armed file. A conflict is not
// retried by the unit of work, so the caller sees the error once and nothing of the unit is
// stored.
func (v *Veto) OnFileFinalized(_ context.Context, ev files.FileFinalized) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.seen == nil {
		v.seen = map[ids.ID]int{}
	}
	v.seen[ev.FileID]++
	if v.armed.IsZero() || v.armed != ev.FileID {
		return nil
	}
	v.armed = ids.ID{}
	v.hits++
	return errs.New(errs.CodeConflict, "vetoed by the test")
}
