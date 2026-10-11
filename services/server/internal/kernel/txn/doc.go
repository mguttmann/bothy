// SPDX-License-Identifier: Apache-2.0

// Package txn declares the unit of work, its phases before the commit and the in-transaction
// event bus (docs/spec/02-architektur.md, "Transaktionen und Unit of Work"). platform/pg
// implements UnitOfWork and creates one Unit per attempt (NewUnit, WithUnit); internal/wire registers the phase hooks of authz, sync and audit
// and every subscriber in one Registry and seals it; every app package uses Do, Publish, Load
// and Store. The package imports only the standard library, kernel/ids and kernel/errs.
//
// Phases of one attempt, in this order (02 "Ablauf"): PhaseRightsBegin (authz hook, only with
// RightsChange), PhaseWork (the function passed to Do), PhaseRightsCheck (authz hook, only with
// RightsChange), PhaseSync (sync hook), PhaseAudit (audit hook), then the commit. A phase
// without a hook is skipped. An error in any phase, also a subscriber veto, rolls the whole unit
// back. CurrentPhase tells code which phase it runs in; events may be published only in
// PhaseWork and PhaseRightsCheck.
//
// Registration: hooks and subscribers are functions or method values of the owning module. The
// registry derives the owner from the Go package of the function and the publisher from the
// package of the event type (the segment after /internal/, "kernel" for kernel events) and
// rejects what it cannot derive, so the golden file of internal/wire names the package that
// holds the function. That is the owner as long as no module exports a wrapper that turns a
// foreign closure into its own function; the golden file shows the full handler path, so such a
// wrapper is visible in review. internal/wire checks the direction against the module register
// and rejects handlers in nested internal packages.
//
// Per-unit state: Key, Load and Store keep values for one attempt (buffered durable events,
// audit records). A retry or a rollback starts from an empty state.
//
// Example (an app package):
//
//	var pending = txn.NewKey[[]Change]("pending changes")
//
//	func (s *Service) Rename(ctx context.Context, id ids.ID, title string) error {
//		return s.uow.Do(ctx, func(ctx context.Context) error {
//			if err := s.repo.Rename(ctx, id, title); err != nil {
//				return err
//			}
//			return txn.Publish(ctx, chat.ThreadUpdated{ID: id})
//		})
//	}
package txn
