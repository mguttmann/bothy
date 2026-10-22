// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/status/app"
)

// TestScratchDBEventLogAndCounters holds the facts of check 3 on a seeded database standing in
// for the restored copy (brief M7-03 "Integration"): a partition of event_log with seq 3 to 7
// reads count 5 between 3 and 7; after one row in the middle is deleted it reads count 4 with the
// same bounds, the gap of 16 8.3 item 3; sequence_counters and the head of audit_events read as
// stored.
func TestScratchDBEventLogAndCounters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	seed(t, super)
	mustExec(t, super, `UPDATE sequence_counters SET value = 6 WHERE name = 'event_seq'`)
	mustExec(t, super, `UPDATE sequence_counters SET value = 3 WHERE name = 'audit_seq'`)
	copyDB := postgres.NewScratchDB(db.OpenPool(t, db.SuperConfig()))

	f, err := copyDB.EventLog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Partitions) != 1 || f.Partitions[0].Count != 5 || f.Partitions[0].MinSeq != 3 || f.Partitions[0].MaxSeq != 7 {
		t.Errorf("partitions %+v, want one with 5 rows from 3 to 7", f.Partitions)
	}
	// event_seq 6 lies below the assigned 7: the counter is behind.
	if f.Counters["event_seq"] != 6 || f.Counters["audit_seq"] != 3 || f.AuditHead != 3 {
		t.Errorf("counters %v, audit head %d", f.Counters, f.AuditHead)
	}

	mustExec(t, super, `DELETE FROM event_log WHERE seq = 5`)
	mustExec(t, super, `UPDATE sequence_counters SET value = 7 WHERE name = 'event_seq'`)
	f, err = copyDB.EventLog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if p := f.Partitions[0]; p.Count != 4 || p.MaxSeq-p.MinSeq+1 != 5 {
		t.Errorf("partition after the gap %+v, want count 4 over 5 numbers", p)
	}
	if f.Counters["event_seq"] != f.Partitions[0].MaxSeq {
		t.Errorf("event_seq %d, want the head %d", f.Counters["event_seq"], f.Partitions[0].MaxSeq)
	}

	// A second partition with rows, and an empty one, which is left out.
	mustExec(t, super, `SELECT sync_event_log_ensure_partitions(1000, 2, 1007)`)
	mustExec(t, super, `INSERT INTO event_log (seq, id, type, resource_kind, receive_action, payload, permission_version)
		VALUES (1001, gen_random_uuid(), 'thread.closed', 'thread', 'thread.read', '{}'::jsonb, 0)`)
	if f, err = copyDB.EventLog(ctx); err != nil || len(f.Partitions) != 2 || f.Partitions[1].MinSeq != 1001 {
		t.Errorf("two partitions: %+v, %v", f.Partitions, err)
	}
}

// TestScratchDBThreadSample holds the facts of check 5: the seeded thread with messages 1 to 3
// and last_message_seq 3 reads without gaps, and with message 2 deleted reads count 2 over 3
// numbers; the sample takes all threads when there are fewer than asked and at most n otherwise.
func TestScratchDBThreadSample(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	seed(t, super)
	mustExec(t, super, `UPDATE threads SET last_message_seq = 3`)
	copyDB := postgres.NewScratchDB(db.OpenPool(t, db.SuperConfig()))

	got, err := copyDB.ThreadSample(ctx, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Messages != 3 || got[0].MinSeq != 1 || got[0].MaxSeq != 3 || got[0].LastMessageSeq != 3 {
		t.Errorf("sample %+v, want the thread with 3 messages 1 to 3", got)
	}
	mustExec(t, super, `DELETE FROM messages WHERE thread_seq = 2`)
	if got, err = copyDB.ThreadSample(ctx, 200); err != nil || got[0].Messages != 2 || got[0].MaxSeq-got[0].MinSeq+1 != 3 {
		t.Errorf("sample after the gap %+v, %v", got, err)
	}

	// Two more threads without messages: n = 2 returns two of the three.
	for _, id := range []string{"0192f4a0-7b1c-7000-8a00-000000000105", "0192f4a0-7b1c-7000-8a00-000000000106"} {
		mustExec(t, super, `INSERT INTO threads (id, channel_id, kind, created_by)
			SELECT $1, channel_id, 'thread', created_by FROM threads LIMIT 1`, id)
	}
	if got, err = copyDB.ThreadSample(ctx, 2); err != nil || len(got) != 2 {
		t.Errorf("sample of 2: %+v, %v", got, err)
	}
	if got, err = copyDB.ThreadSample(ctx, 200); err != nil || len(got) != 3 {
		t.Errorf("sample of 200 over 3 threads: %+v, %v", got, err)
	}
}

// TestScratchDBFilesAndAudit holds the facts of checks 2, 4 and 6: Facts equals the manifest
// snapshot, the file sample holds only complete, clean files that are not deleted, and the audit chain
// yields every row and checkpoint in seq order with the instance of the copy.
func TestScratchDBFilesAndAudit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	seed(t, super)
	mustExec(t, super, `UPDATE files SET upload_state = 'complete', scan_status = 'clean', sha256 = sha256(object_key::bytea) WHERE object_key <> 'objects/b'`)
	// A complete upload that the scan found infected or could not scan has no object any more
	// (the scan pass deletes it), so it never enters the sample.
	for _, scan := range []string{"infected", "error"} {
		mustExec(t, super, `INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, upload_state, scan_status)
			SELECT gen_random_uuid(), 'objects/' || $1, '\x00', 1, 'image/png', 'a.png', uploaded_by, 'avatar', 'complete', $1 FROM files LIMIT 1`, scan)
	}
	pool := db.OpenPool(t, db.SuperConfig())
	copyDB := postgres.NewScratchDB(pool)

	want, err := postgres.NewManifestSource(pool).Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got, err := copyDB.Facts(ctx)
	if err != nil || got.HeadSeq != want.HeadSeq || got.RowCounts["messages"] != want.RowCounts["messages"] {
		t.Errorf("Facts %+v, %v; want %+v", got, err, want)
	}

	files, err := copyDB.FileSample(ctx, 200)
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(files))
	for _, f := range files {
		keys = append(keys, f.ObjectKey)
		if len(f.SHA256) != 32 {
			t.Errorf("file %s sha256 of %d bytes", f.ObjectKey, len(f.SHA256))
		}
	}
	slices.Sort(keys)
	// objects/b is pending, objects/c is deleted, objects/infected and objects/error have no object.
	if !slices.Equal(keys, []string{"objects/a"}) {
		t.Errorf("file sample %v, want only objects/a", keys)
	}

	in, err := copyDB.AuditChain(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if in.InstanceID.String() != "0192f4a0-7b1c-7000-8a00-0000000000a1" {
		t.Errorf("instance %s", in.InstanceID)
	}
	var seqs []int64
	for e, err := range in.Events {
		if err != nil {
			t.Fatal(err)
		}
		seqs = append(seqs, e.Seq)
	}
	if !slices.Equal(seqs, []int64{1, 2, 3}) {
		t.Errorf("audit rows %v, want 1 2 3", seqs)
	}
	n := 0
	for _, err := range in.Checkpoints {
		if err != nil {
			t.Fatal(err)
		}
		n++
	}
	if n != 0 {
		t.Errorf("%d checkpoints, want none", n)
	}
	var _ app.ScratchDB = copyDB
}

// TestScratchDBAuditCheckpointsAndKeys: the audit chain of the copy carries the public keys of
// purpose audit_checkpoint only and every row of audit_checkpoints in seq order with its stored
// hash and signature; a hash of another size reads as zero, which the walk reports.
func TestScratchDBAuditCheckpointsAndKeys(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	seed(t, super)
	mustExec(t, super, `INSERT INTO server_signing_keys (kid, purpose, alg, public_jwk, private_key_ref, state) VALUES
		('cp-1', 'audit_checkpoint', 'EdDSA', '{"kty":"OKP","crv":"Ed25519","x":"EXAMPLE"}', 'file:EXAMPLE', 'active'),
		('at-1', 'access_token', 'EdDSA', '{"kty":"OKP","crv":"Ed25519","x":"EXAMPLE"}', 'file:EXAMPLE', 'active')`)
	mustExec(t, super, `INSERT INTO audit_checkpoints (seq, hash, signature, kid) VALUES
		(2, sha256('two'), 'sig-two', 'cp-1'), (1, sha256('one'), 'sig-one', 'cp-1'), (3, '\x00', 'sig-three', 'cp-1')`)
	in, err := postgres.NewScratchDB(db.OpenPool(t, db.SuperConfig())).AuditChain(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Keys) != 1 || in.Keys[0].KID != "cp-1" || in.Keys[0].Purpose != audit.PurposeAuditCheckpoint || in.Keys[0].Alg != "EdDSA" {
		t.Errorf("keys %+v, want only cp-1 of purpose audit_checkpoint", in.Keys)
	}
	var seqs []int64
	var sigs []string
	var zero [audit.HashSize]byte
	for c, err := range in.Checkpoints {
		if err != nil {
			t.Fatal(err)
		}
		seqs, sigs = append(seqs, c.Seq), append(sigs, string(c.Signature))
		if (c.Seq == 3) != (c.Hash == zero) {
			t.Errorf("checkpoint %d hash %x", c.Seq, c.Hash)
		}
	}
	if !slices.Equal(seqs, []int64{1, 2, 3}) || !slices.Equal(sigs, []string{"sig-one", "sig-two", "sig-three"}) {
		t.Errorf("checkpoints %v %v, want 1 2 3 in order", seqs, sigs)
	}
}
