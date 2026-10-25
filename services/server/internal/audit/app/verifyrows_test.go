// SPDX-License-Identifier: Apache-2.0

//go:build unix

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"iter"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// copyInput reads the chain of the database of e as the restore test reads it from its copy:
// every row of audit_events and audit_checkpoints and the public keys of server_signing_keys.
func copyInput(t *testing.T, e *pgEnv) audit.RowsInput {
	t.Helper()
	ctx := context.Background()
	rows, err := e.superConn.Query(ctx, `SELECT seq, id, occurred_at, actor_principal_id, actor_device_id, actor_session_id,
		action, decision, deny_reason, resource_kind, resource_id, details, correlation_id, prev_hash, hash
		FROM audit_events ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	var events []audit.Event
	for rows.Next() {
		var (
			ev                                        audit.Event
			id, principal, device, session, res, corr pgtype.UUID
			at                                        pgtype.Timestamptz
			deny, kind                                pgtype.Text
			details, prev, hash                       []byte
		)
		if err := rows.Scan(&ev.Seq, &id, &at, &principal, &device, &session, &ev.Action, &ev.Decision, &deny, &kind,
			&res, &details, &corr, &prev, &hash); err != nil {
			t.Fatal(err)
		}
		uid := func(u pgtype.UUID) ids.ID {
			if !u.Valid {
				return ids.ID{}
			}
			return ids.ID(u.Bytes)
		}
		ev.ID, ev.OccurredAt = uid(id), at.Time.UTC()
		ev.ActorPrincipalID, ev.ActorDeviceID, ev.ActorSessionID = uid(principal), uid(device), uid(session)
		ev.DenyReason, ev.ResourceKind, ev.ResourceID, ev.CorrelationID = deny.String, kind.String, uid(res), uid(corr)
		dec := json.NewDecoder(bytes.NewReader(details))
		dec.UseNumber()
		if err := dec.Decode(&ev.Details); err != nil {
			t.Fatal(err)
		}
		copy(ev.PrevHash[:], prev)
		copy(ev.Hash[:], hash)
		events = append(events, ev)
	}
	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}
	cps, err := e.superConn.Query(ctx, `SELECT seq, hash, signature FROM audit_checkpoints ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	checkpoints, err := pgx.CollectRows(cps, func(r pgx.CollectableRow) (audit.CheckpointRow, error) {
		var c audit.CheckpointRow
		var hash []byte
		err := r.Scan(&c.Seq, &hash, &c.Signature)
		copy(c.Hash[:], hash)
		return c, err
	})
	if err != nil {
		t.Fatal(err)
	}
	keyRows, err := e.superConn.Query(ctx, `SELECT kid, purpose, alg, public_jwk FROM server_signing_keys ORDER BY kid`)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := pgx.CollectRows(keyRows, func(r pgx.CollectableRow) (audit.VerificationKey, error) {
		var k audit.VerificationKey
		err := r.Scan(&k.KID, &k.Purpose, &k.Alg, &k.PublicJWK)
		return k, err
	})
	if err != nil {
		t.Fatal(err)
	}
	in := audit.RowsInput{InstanceID: e.inst, Keys: keys, Events: seqOf(events), Checkpoints: seqOf(checkpoints)}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Action == audit.ActionRetentionTrim {
			in.LatestAnchor = &events[i]
			break
		}
	}
	return in
}

// seqOf yields the elements of s without error.
func seqOf[T any](s []T) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for _, v := range s {
			if !yield(v, nil) {
				return
			}
		}
	}
}

// TestVerifyRowsMatchesChainVerify: the port function of the restore test (brief M7-03 design
// note 2) and the job audit.chain_verify agree on a valid chain with two signed checkpoints and
// on one changed row, which both report as hash_mismatch at its seq; the operation
// verifyAuditChain (Log.Verify) agrees with both.
func TestVerifyRowsMatchesChainVerify(t *testing.T) {
	t.Parallel()
	e := newPGEnv(t, true, false)
	e.write(5)
	if err := e.log.RunCheckpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.write(2)
	if err := e.log.RunCheckpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	if e.metrics.failures != 0 || e.metrics.successes != 2 {
		t.Fatalf("job over the valid chain: %+v", e.metrics)
	}
	var v audit.RowsVerifier = app.RowsVerifier{}
	got, err := v.VerifyRows(context.Background(), copyInput(t, e))
	if err != nil || !got.OK || got.CheckpointsCompared != 2 || got.LastCheckpointSeq == 0 || got.CheckedFromSeq != 1 {
		t.Fatalf("VerifyRows over the valid chain: %+v, %v", got, err)
	}
	api, err := e.log.Verify(e.ctx(), audit.VerifyRequest{})
	if err != nil || !api.OK || api.CheckpointsCompared != got.CheckpointsCompared || api.LastCheckpointSeq != got.LastCheckpointSeq {
		t.Fatalf("Log.Verify %+v, VerifyRows %+v", api, got)
	}

	// One changed row after the last checkpoint, the range the job walks.
	e.write(1)
	var broken int64
	if err := e.superConn.QueryRow(context.Background(), `SELECT max(seq) FROM audit_events`).Scan(&broken); err != nil {
		t.Fatal(err)
	}
	e.tamper("audit_events", `UPDATE audit_events SET details = '{"x":true}' WHERE seq = $1`, broken)
	if err := e.log.RunCheckpoint(context.Background()); err != nil || e.metrics.failures != 1 || !e.rep.has("verify_failed hash_mismatch") {
		t.Fatalf("job over the changed row: %v, %+v", err, e.metrics)
	}
	var jobSeq string
	if err := e.superConn.QueryRow(context.Background(), `SELECT details->>'first_broken_seq' FROM audit_events
		WHERE action = 'audit.verify' AND decision = 'error' ORDER BY seq DESC LIMIT 1`).Scan(&jobSeq); err != nil {
		t.Fatal(err)
	}
	got, err = v.VerifyRows(context.Background(), copyInput(t, e))
	if err != nil || got.OK || got.FailureReason != string(domain.HashMismatch) || strconv.FormatInt(got.FirstBrokenSeq, 10) != jobSeq ||
		got.FirstBrokenSeq != broken {
		t.Errorf("VerifyRows over the changed row %d: %+v, %v; the job named %s", broken, got, err, jobSeq)
	}
	api, err = e.log.Verify(e.ctx(), audit.VerifyRequest{})
	if err != nil || api.OK || api.FirstBrokenSeq != got.FirstBrokenSeq || api.FailureReason != got.FailureReason {
		t.Errorf("Log.Verify %+v, VerifyRows %+v", api, got)
	}

	// A checkpoint whose signature was replaced is not trusted.
	e.tamper("audit_checkpoints", `UPDATE audit_checkpoints SET signature = convert_to('a.b.c', 'UTF8') WHERE seq = (SELECT min(seq) FROM audit_checkpoints)`)
	got, err = v.VerifyRows(context.Background(), copyInput(t, e))
	if err != nil || got.OK || got.FailureReason != string(domain.CheckpointInvalid) || got.LastCheckpointSeq != 0 {
		t.Errorf("VerifyRows with a forged checkpoint: %+v, %v", got, err)
	}
}
