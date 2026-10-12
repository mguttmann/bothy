// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// TestUnitOfWork_LateRightsChange: a unit opened with txn.AllowLateRightsChange (the
// Idempotency-Key stage) becomes a rights change through its first nested Do that declares one:
// the rights begin hook runs before the nested work, the rights check right after it, and the
// sync and audit hooks once at the end of the outer unit (docs/DECISIONS.md M1-31).
func TestUnitOfWork_LateRightsChange(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	var l phaseLog
	uow := e.uow(recordingRegistry(t, &l, nil))
	p := ids.New()
	err := uow.Do(context.Background(), func(ctx context.Context) error {
		l.add(ctx)
		if txn.InRightsChange(ctx) {
			t.Error("the outer unit is a rights change before the nested declaration")
		}
		if err := uow.Do(ctx, func(ctx context.Context) error {
			l.add(ctx)
			spec, ok := txn.RightsChangeOf(ctx)
			if !ok || spec.Action != "owner.add" || !slices.Equal(spec.Principals, []ids.ID{p}) {
				t.Errorf("RightsChangeOf = %+v, %v", spec, ok)
			}
			return insert(ctx, 7, "late")
		}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: "owner.add", Principals: []ids.ID{p}})); err != nil {
			return err
		}
		l.add(ctx)
		// A second rights change in the same unit would skip its own begin hook.
		err := uow.Do(ctx, func(context.Context) error { return nil }, txn.RightsChange(p))
		if errs.KindOf(err) != errs.KindInternal {
			t.Errorf("second late rights change = %v, want kind internal", err)
		}
		return nil
	}, txn.AllowLateRightsChange())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"work", "rights_begin", "work", "rights_check", "work", "sync", "audit"}
	if !slices.Equal(l.get(), want) {
		t.Fatalf("phases = %v, want %v", l.get(), want)
	}
	if !e.rowExists(t, 7) {
		t.Fatal("the row of the late rights change was not committed")
	}
}

// TestUnitOfWork_LateRightsChangeCheckFails: a failing rights check reaches the nested caller,
// and the outer unit rolls back with it.
func TestUnitOfWork_LateRightsChangeCheckFails(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	var l phaseLog
	veto := errors.New("veto")
	uow := e.uow(recordingRegistry(t, &l, map[txn.Phase]error{txn.PhaseRightsCheck: veto}))
	var nested error
	err := uow.Do(context.Background(), func(ctx context.Context) error {
		nested = uow.Do(ctx, func(ctx context.Context) error { return insert(ctx, 8, "x") }, txn.RightsChange(ids.New()))
		return nested
	}, txn.AllowLateRightsChange())
	if !errors.Is(nested, veto) || !errors.Is(err, veto) {
		t.Fatalf("nested = %v, outer = %v, want the veto", nested, err)
	}
	if e.rowExists(t, 8) {
		t.Fatal("the row survived the veto")
	}
}

// TestUnitOfWork_LateRightsChangeOnlyWhenAllowed: without the option a nested rights change
// stays an internal error.
func TestUnitOfWork_LateRightsChangeOnlyWhenAllowed(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	uow := e.uow(txn.NewRegistry())
	err := uow.Do(context.Background(), func(ctx context.Context) error {
		return uow.Do(ctx, func(context.Context) error { return nil }, txn.RightsChange(ids.New()))
	})
	if errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("err = %v, want kind internal", err)
	}
}

// TestUnitOfWork_LateRightsChangeErrorFailsTheUnit (red team M1-31): a failed late rights change
// fails the whole unit even when its caller swallows the error, so neither the raised rights
// version nor the writes of its function commit without a passed rights check.
func TestUnitOfWork_LateRightsChangeErrorFailsTheUnit(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	var l phaseLog
	veto := errors.New("veto")
	uow := e.uow(recordingRegistry(t, &l, map[txn.Phase]error{txn.PhaseRightsCheck: veto}))
	err := uow.Do(context.Background(), func(ctx context.Context) error {
		_ = uow.Do(ctx, func(ctx context.Context) error { return insert(ctx, 9, "x") }, txn.RightsChange(ids.New()))
		return nil // swallowed
	}, txn.AllowLateRightsChange())
	if !errors.Is(err, veto) {
		t.Fatalf("Do = %v, want the veto of the late rights change", err)
	}
	if e.rowExists(t, 9) {
		t.Fatal("the writes of a failed late rights change committed")
	}
}

// TestUnitOfWork_LateRightsChangeAllowsCoveredNesting: inside the function of a late rights
// change a nested Do that the declaration covers joins as in any rights change.
func TestUnitOfWork_LateRightsChangeAllowsCoveredNesting(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	var l phaseLog
	uow := e.uow(recordingRegistry(t, &l, nil))
	p := ids.New()
	err := uow.Do(context.Background(), func(ctx context.Context) error {
		return uow.Do(ctx, func(ctx context.Context) error {
			return uow.Do(ctx, func(ctx context.Context) error { return insert(ctx, 10, "covered") }, txn.RightsChange(p))
		}, txn.RightsChange(p))
	}, txn.AllowLateRightsChange())
	if err != nil || !e.rowExists(t, 10) {
		t.Fatalf("Do = %v, row %v", err, e.rowExists(t, 10))
	}
}

// TestUnitOfWork_LateRightsChangePanicFailsTheUnit: a panic in a late rights change that a caller
// recovers still fails the unit.
func TestUnitOfWork_LateRightsChangePanicFailsTheUnit(t *testing.T) {
	t.Parallel()
	e := newUnitEnv(t)
	var l phaseLog
	uow := e.uow(recordingRegistry(t, &l, nil))
	err := uow.Do(context.Background(), func(ctx context.Context) error {
		func() {
			defer func() { _ = recover() }()
			_ = uow.Do(ctx, func(ctx context.Context) error {
				if err := insert(ctx, 11, "x"); err != nil {
					return err
				}
				panic("boom")
			}, txn.RightsChange(ids.New()))
		}()
		return nil
	}, txn.AllowLateRightsChange())
	if errs.KindOf(err) != errs.KindInternal || e.rowExists(t, 11) {
		t.Fatalf("Do = %v, row %v; want an internal error and no row", err, e.rowExists(t, 11))
	}
}
