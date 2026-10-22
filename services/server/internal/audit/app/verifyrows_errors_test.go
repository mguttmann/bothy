// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

var errRows = errors.New("fake: the copy cannot be read")

// failing yields one error.
func failing[T any]() func(func(T, error) bool) {
	return func(yield func(T, error) bool) {
		var zero T
		yield(zero, errRows)
	}
}

// TestVerifyRowsInputs: VerifyRows returns the first error of either iterator; an empty copy
// without events and checkpoints is ok and checks nothing; a checkpoint row whose signature is no
// JWS of the supplied keys is checkpoint_invalid; a row below 1 is outside the chain (seq_gap); a
// context that ends between batches stops the walk.
func TestVerifyRowsInputs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	inst := ids.ID{1}
	if _, err := app.VerifyRows(ctx, audit.RowsInput{InstanceID: inst, Events: failing[audit.Event]()}); !errors.Is(err, errRows) {
		t.Errorf("events error: %v", err)
	}
	if _, err := app.VerifyRows(ctx, audit.RowsInput{InstanceID: inst, Checkpoints: failing[audit.CheckpointRow]()}); !errors.Is(err, errRows) {
		t.Errorf("checkpoints error: %v", err)
	}
	res, err := app.VerifyRows(ctx, audit.RowsInput{InstanceID: inst})
	if err != nil || !res.OK || res.CheckedToSeq != 0 {
		t.Errorf("empty copy: %+v, %v; want ok without rows", res, err)
	}
	forged := func(yield func(audit.CheckpointRow, error) bool) {
		yield(audit.CheckpointRow{Seq: 1, Signature: []byte("not-a-jws")}, nil)
	}
	keys := []audit.VerificationKey{{KID: "cp-1", Purpose: audit.PurposeAuditCheckpoint, Alg: "EdDSA", PublicJWK: []byte(`{}`)}}
	res, err = app.VerifyRows(ctx, audit.RowsInput{InstanceID: inst, Keys: keys, Checkpoints: forged})
	if err != nil || res.OK || res.FailureReason != "checkpoint_invalid" {
		t.Errorf("forged checkpoint: %+v, %v; want checkpoint_invalid", res, err)
	}
	below := func(yield func(audit.Event, error) bool) {
		yield(audit.Event{Seq: 0}, nil)
	}
	res, err = app.VerifyRows(ctx, audit.RowsInput{InstanceID: inst, Events: below})
	if err != nil || res.OK || res.FailureReason != "seq_gap" || res.FirstBrokenSeq != 0 {
		t.Errorf("row below 1: %+v, %v; want seq_gap at 0", res, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	one := func(yield func(audit.Event, error) bool) {
		yield(audit.Event{Seq: 1}, nil)
	}
	if _, err := app.VerifyRows(canceled, audit.RowsInput{InstanceID: inst, Events: one}); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled context: %v", err)
	}
}
