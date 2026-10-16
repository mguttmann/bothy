// SPDX-License-Identifier: Apache-2.0

//go:build unix

package httpapi_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// checkpointRows returns the seqs of audit_checkpoints and how many have exported_at.
func (e *env) checkpointRows() ([]int64, int) {
	e.t.Helper()
	rows, err := e.super.Query(context.Background(), "SELECT seq, exported_at IS NOT NULL FROM audit_checkpoints ORDER BY seq")
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var seqs []int64
	exported := 0
	for rows.Next() {
		var s int64
		var x bool
		if err := rows.Scan(&s, &x); err != nil {
			e.t.Fatal(err)
		}
		seqs = append(seqs, s)
		if x {
			exported++
		}
	}
	return seqs, exported
}

// payloadOf decodes the payload of the sink file name into its members.
func (e *env) payloadOf(name string) (map[string]any, []byte) {
	e.t.Helper()
	raw, err := os.ReadFile(filepath.Join(e.sinkDir, e.instance.String(), name)) //nolint:gosec // G304: a file of the sink below t.TempDir
	if err != nil {
		e.t.Fatal(err)
	}
	parts := strings.Split(string(raw), ".")
	head, _ := base64.RawURLEncoding.DecodeString(parts[0])
	if !strings.Contains(string(head), `"typ":"audit-checkpoint+json"`) || !strings.Contains(string(head), `"kid":`) {
		e.t.Errorf("header %s", head)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		e.t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		e.t.Fatal(err)
	}
	return m, payload
}

// TestCheckpointJobSignsOnlyVerifiedSegments: with a broken chain the job signs no checkpoint,
// adds one to bothy.audit.verify.failures, records audit.verify with decision error and no actor,
// logs the error and ends without error (River does not retry a detected break).
func TestCheckpointJobSignsOnlyVerifiedSegments(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.write(4, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	before, _ := e.checkpointRows()
	e.write(3, actor.Actor{}, "grant.create", "grant", ids.New())
	// A row of the segment after the last checkpoint, the one the job recomputes (SEC-AUD-2).
	k := e.head() - 1
	e.attack("audit_events", "audit_events_no_update_delete", `UPDATE audit_events SET details = '{"changed":true}' WHERE seq = $1`, k)
	e.run()
	after, _ := e.checkpointRows()
	if len(after) != len(before) || len(e.sinkFiles()) != len(before) {
		t.Fatalf("checkpoints %v after the break, before %v; the job signed a broken segment", after, before)
	}
	if failures, _ := e.metrics(); failures != 1 {
		t.Errorf("bothy.audit.verify.failures = %d, want 1", failures)
	}
	if n := e.count(`action = 'audit.verify' AND decision = 'error' AND actor_principal_id IS NULL
		AND details = jsonb_build_object('first_broken_seq', $1::bigint, 'failure_reason', 'hash_mismatch')`, k); n != 1 {
		t.Errorf("%d error entries of audit.verify, want 1", n)
	}
	if e.logs.count("audit chain verification failed") != 1 || e.logs.count("bothy.reason=hash_mismatch") != 1 {
		t.Errorf("log lines %v", e.logs.lines)
	}
}

// TestCheckpointJobWritesOnceAndLinks: two runs with new rows between them give two files, the
// second links the first by previous_checkpoint_hash, exported_at is set; a third run without new
// rows writes nothing; the gauge holds the time of the last success.
func TestCheckpointJobWritesOnceAndLinks(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.write(3, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	first := e.head()
	e.write(2, actor.Actor{}, "grant.create", "grant", ids.New())
	e.clk.Advance(time.Hour)
	e.run()
	second := e.head()
	files := e.sinkFiles()
	if len(files) != 2 {
		t.Fatalf("sink files %v, want 2", files)
	}
	p1, raw1 := e.payloadOf(files[0])
	p2, _ := e.payloadOf(files[1])
	sum := sha256.Sum256(raw1)
	if p1["seq"] != float64(first) || p2["seq"] != float64(second) || p1["instance_id"] != e.instance.String() {
		t.Errorf("payloads %v and %v, want seq %d and %d", p1, p2, first, second)
	}
	if _, ok := p1["previous_checkpoint_hash"]; ok || p2["previous_checkpoint_hash"] != hex.EncodeToString(sum[:]) {
		t.Errorf("links: first %v, second %v, want the SHA-256 of the first payload", p1, p2)
	}
	var hash string
	if err := e.super.QueryRow(context.Background(), "SELECT encode(hash, 'hex') FROM audit_events WHERE seq = $1", second).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if p2["hash"] != hash {
		t.Errorf("hash %v, want the row's %s", p2["hash"], hash)
	}
	seqs, exported := e.checkpointRows()
	if len(seqs) != 2 || exported != 2 {
		t.Errorf("audit_checkpoints %v with %d exported, want 2 and 2", seqs, exported)
	}
	_, last := e.metrics()
	e.clk.Advance(time.Hour)
	e.run()
	if files := e.sinkFiles(); len(files) != 2 {
		t.Errorf("a run without new rows wrote %v", files)
	}
	if seqs, _ := e.checkpointRows(); len(seqs) != 2 {
		t.Errorf("a run without new rows inserted %v", seqs)
	}
	if _, now := e.metrics(); now != float64(e.clk.Now().Unix()) || now <= last {
		t.Errorf("last success %v, want %v (after %v)", now, e.clk.Now().Unix(), last)
	}
	if failures, _ := e.metrics(); failures != 0 {
		t.Errorf("failures %d", failures)
	}
}

// TestCheckpointJobExportsAfterTheSinkReturns: while the sink cannot be read the run fails before
// it signs (River retries); once the sink is back the next run signs and exports once.
func TestCheckpointJobExportsAfterTheSinkReturns(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.write(3, actor.Actor{}, "grant.create", "grant", ids.New())
	moved := e.sinkDir + ".away"
	if err := os.Rename(e.sinkDir, moved); err != nil {
		t.Fatal(err)
	}
	if err := e.log.RunCheckpoint(context.Background()); err == nil {
		t.Fatal("a run with a missing sink passed; River must retry it")
	}
	if seqs, _ := e.checkpointRows(); len(seqs) != 0 {
		t.Fatalf("checkpoints %v signed while the sink was missing", seqs)
	}
	if err := os.Rename(moved, e.sinkDir); err != nil {
		t.Fatal(err)
	}
	e.run()
	if seqs, exported := e.checkpointRows(); len(seqs) != 1 || exported != 1 || len(e.sinkFiles()) != 1 {
		t.Fatalf("after the sink returned: %v, %d exported, files %v", seqs, exported, e.sinkFiles())
	}
}

// TestMissingSinkWarns: without a sink every verification carries
// checkpoint_sink_not_configured and compares nothing, and every job run logs the warning;
// checkpoints are still signed into the database and exported once a sink is set.
func TestMissingSinkWarns(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	e.owner("owner")
	e.write(2, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	e.run()
	if n := e.logs.count("audit_checkpoint_sink_not_configured"); n != 2 {
		t.Errorf("%d warnings for two runs, want 2", n)
	}
	resp, res := e.verify("owner", "{}")
	if resp.StatusCode != 200 || !res.Ok || len(res.Warnings) != 1 || res.Warnings[0] != "checkpoint_sink_not_configured" || res.CheckpointsCompared != 0 {
		t.Fatalf("verify without a sink: %d %s", resp.StatusCode, resp.Body)
	}
	if seqs, exported := e.checkpointRows(); len(seqs) != 1 || exported != 0 {
		t.Errorf("checkpoints %v, %d exported; want one, not exported", seqs, exported)
	}
	e.audit.WarnWithoutSink(context.Background())
	if n := e.logs.count("audit_checkpoint_sink_not_configured"); n != 3 {
		t.Errorf("the start warning is missing: %d", n)
	}
}

// errorEntries counts the audit.verify entries with decision error naming reason at seq.
func (e *env) errorEntries(seq int64, reason string) int {
	e.t.Helper()
	return e.count(`action = 'audit.verify' AND decision = 'error'
		AND details = jsonb_build_object('first_broken_seq', $1::bigint, 'failure_reason', $2::text)`, seq, reason)
}

// TestSECT16_DeletedOldestRowsAreDetected: until the anchors of M7 the log starts at row 1;
// deleting the oldest rows below the sink checkpoints is seq_gap at 1, for verifyAuditChain and
// for the job, which signs nothing over the shortened log.
func TestSECT16_DeletedOldestRowsAreDetected(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.owner("owner")
	e.write(6, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	e.write(2, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	before, _ := e.checkpointRows()
	e.attack("audit_events", "audit_events_no_update_delete", `DELETE FROM audit_events WHERE seq < 5`)
	_, res := e.verify("owner", "{}")
	if res.Ok || reason(res) != "seq_gap" || broken(res) != 1 {
		t.Fatalf("deleted oldest rows: %+v", res)
	}
	// Reset the job to the start of the log, as if the database checkpoints were gone too.
	e.attack("audit_checkpoints", "audit_checkpoints_no_update_delete", `DELETE FROM audit_checkpoints`)
	e.run()
	if after, _ := e.checkpointRows(); len(after) != 0 || len(e.sinkFiles()) != len(before) {
		t.Errorf("the job signed over the shortened log: %v, files %v", after, e.sinkFiles())
	}
	if failures, _ := e.metrics(); failures != 1 {
		t.Errorf("failures = %d, want 1", failures)
	}
}

// TestJobDetectsDeletedDatabaseCheckpoints: a checkpoint of the sink newer than the last one of
// audit_checkpoints means the database lost signed checkpoints: the job reports
// checkpoint_mismatch at it and signs nothing, instead of forking or freezing the sink.
func TestJobDetectsDeletedDatabaseCheckpoints(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.write(3, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	e.write(2, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	seqs, _ := e.checkpointRows()
	newest := seqs[len(seqs)-1]
	e.attack("audit_checkpoints", "audit_checkpoints_no_update_delete", `DELETE FROM audit_checkpoints WHERE seq = $1`, newest)
	e.write(2, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	if after, _ := e.checkpointRows(); len(after) != len(seqs)-1 || len(e.sinkFiles()) != len(seqs) {
		t.Errorf("checkpoints %v and files %v after the loss", after, e.sinkFiles())
	}
	if n := e.errorEntries(newest, "checkpoint_mismatch"); n != 1 {
		t.Errorf("%d error entries at %d", n, newest)
	}
}

// TestJobRefusesForgedCheckpoint: a checkpoint row that the server did not sign is never handed
// to the sink; the job reports checkpoint_invalid at it.
func TestJobRefusesForgedCheckpoint(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.write(3, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	e.write(1, actor.Actor{}, "grant.create", "grant", ids.New())
	head := e.head()
	var kid string
	if err := e.super.QueryRow(context.Background(), "SELECT kid FROM audit_checkpoints LIMIT 1").Scan(&kid); err != nil {
		t.Fatal(err)
	}
	e.attack("audit_checkpoints", "audit_checkpoints_no_update_delete",
		`INSERT INTO audit_checkpoints (seq, hash, signature, kid) SELECT $1, hash, 'a.b.c', $2 FROM audit_events WHERE seq = $1`, head, kid)
	e.run()
	if n := e.errorEntries(head, "checkpoint_invalid"); n != 1 {
		t.Errorf("%d error entries at %d", n, head)
	}
	if files := e.sinkFiles(); len(files) != 1 {
		t.Errorf("the forged checkpoint reached the sink: %v", files)
	}
}

// TestWrongSizeHashIsABreak: a hash of another size at the row of the last checkpoint is a
// detected break of the job (counter, error entry, no retry), not a storage error, and
// verifyAuditChain reports it.
func TestWrongSizeHashIsABreak(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.owner("owner")
	e.write(3, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	c := e.head()
	e.write(2, actor.Actor{}, "grant.create", "grant", ids.New())
	e.attack("audit_events", "audit_events_no_update_delete", `UPDATE audit_events SET hash = '\x00' WHERE seq = $1`, c)
	e.run()
	if failures, _ := e.metrics(); failures != 1 {
		t.Errorf("failures = %d, want 1", failures)
	}
	if n := e.errorEntries(c, "checkpoint_mismatch"); n != 1 {
		t.Errorf("%d error entries at %d", n, c)
	}
	if _, res := e.verify("owner", "{}"); res.Ok || broken(res) != c {
		t.Errorf("verify: %+v", res)
	}
}

// TestDeletedNewestRowsStillAnswer: with the newest rows gone the audit hook cannot append, yet
// verifyAuditChain answers with the failure (the entry it cannot write is reported) and the job
// counts the break without a retry.
func TestDeletedNewestRowsStillAnswer(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.owner("owner")
	e.write(4, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	head := e.head()
	e.attack("audit_events", "audit_events_no_update_delete", `DELETE FROM audit_events WHERE seq >= $1`, head-1)
	resp, res := e.verify("owner", "{}")
	if resp.StatusCode != http.StatusOK || res.Ok || reason(res) != "checkpoint_mismatch" || broken(res) != head {
		t.Fatalf("verify: %d %s", resp.StatusCode, resp.Body)
	}
	if resp, res := e.verify("owner", `{"to_seq": 3}`); resp.StatusCode != http.StatusOK || !res.Ok {
		t.Errorf("a range before the gap: %d %s", resp.StatusCode, resp.Body)
	}
	if err := e.log.RunCheckpoint(context.Background()); err != nil {
		t.Errorf("the job returned %v; a detected break is no error", err)
	}
	if failures, _ := e.metrics(); failures != 1 {
		t.Errorf("failures = %d, want 1", failures)
	}
	if e.logs.count("audit entry of the chain verification not written") == 0 {
		t.Error("the failed entry was not reported")
	}
}
