// SPDX-License-Identifier: Apache-2.0

//go:build unix

package httpapi_test

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// reason returns the failure reason of res, "" when ok.
func reason(res rest.AuditVerifyResult) string {
	if res.FailureReason == nil {
		return ""
	}
	return string(*res.FailureReason)
}

// broken returns first_broken_seq of res, 0 when ok.
func broken(res rest.AuditVerifyResult) int64 {
	if res.FirstBrokenSeq == nil {
		return 0
	}
	return *res.FirstBrokenSeq
}

// TestSECT16_ChangedRowIsDetected: an attacker with write access to the database changes the
// details of row k; verifyAuditChain answers ok false, first_broken_seq k, hash_mismatch.
func TestSECT16_ChangedRowIsDetected(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.owner("owner")
	e.write(9, e.owner("alex"), "grant.create", "grant", ids.New())
	e.run()
	if _, res := e.verify("owner", "{}"); !res.Ok {
		t.Fatalf("intact chain: %+v", res)
	}
	const k = 5
	e.attack("audit_events", "audit_events_no_update_delete", `UPDATE audit_events SET details = '{"changed":true}' WHERE seq = $1`, k)
	resp, res := e.verify("owner", "{}")
	if resp.StatusCode != http.StatusOK || res.Ok || broken(res) != k || reason(res) != "hash_mismatch" {
		t.Fatalf("after the change: %d %s", resp.StatusCode, resp.Body)
	}
}

// TestSECT16_RewrittenChainIsDetectedByCheckpoint: after a checkpoint at c >= k the attacker
// changes row k, recomputes every later hash consistently and overwrites audit_checkpoints too;
// the walk alone finds nothing, the checkpoint of the sink gives checkpoint_mismatch at c.
func TestSECT16_RewrittenChainIsDetectedByCheckpoint(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.owner("owner")
	e.write(9, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	c := e.head()
	e.write(3, actor.Actor{}, "grant.create", "grant", ids.New())
	const k = 4
	e.attack("audit_events", "audit_events_no_update_delete", `UPDATE audit_events SET details = '{"changed":true}' WHERE seq = $1`, k)
	e.rewriteFrom(k)
	// The database checkpoint is rewritten to the new hash, so only the sink still knows the old.
	e.attack("audit_checkpoints", "audit_checkpoints_no_update_delete",
		"UPDATE audit_checkpoints SET hash = (SELECT hash FROM audit_events WHERE seq = $1) WHERE seq = $1", c)
	resp, res := e.verify("owner", "{}")
	if resp.StatusCode != http.StatusOK || res.Ok || broken(res) != c || reason(res) != "checkpoint_mismatch" || res.CheckpointsCompared != 1 {
		t.Fatalf("rewritten chain: %d %s, want checkpoint_mismatch at %d", resp.StatusCode, resp.Body, c)
	}
}

// TestVerifyIgnoresDatabaseCheckpoints: verification reads checkpoints only from the sink.
// Deleting or altering audit_checkpoints changes nothing while the sink is intact; with an empty
// sink a rewritten chain is not reported as protected (checkpoints_compared 0).
func TestVerifyIgnoresDatabaseCheckpoints(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.owner("owner")
	e.write(6, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	e.write(2, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	_, before := e.verify("owner", "{}")
	if !before.Ok || before.CheckpointsCompared != 2 {
		t.Fatalf("before: %+v", before)
	}
	e.attack("audit_checkpoints", "audit_checkpoints_no_update_delete", `UPDATE audit_checkpoints SET hash = '\x00', signature = 'x'`)
	_, altered := e.verify("owner", "{}")
	e.attack("audit_checkpoints", "audit_checkpoints_no_update_delete", `DELETE FROM audit_checkpoints`)
	_, deleted := e.verify("owner", "{}")
	for name, res := range map[string]rest.AuditVerifyResult{"altered": altered, "deleted": deleted} {
		if !res.Ok || res.CheckpointsCompared != before.CheckpointsCompared || *res.LastCheckpointSeq != *before.LastCheckpointSeq {
			t.Errorf("%s audit_checkpoints changed the result: %+v, before %+v", name, res, before)
		}
	}

	// An empty sink: a rewritten chain verifies, and the result says that nothing protected it.
	for _, f := range e.sinkFiles() {
		if err := os.Remove(filepath.Join(e.sinkDir, e.instance.String(), f)); err != nil {
			t.Fatal(err)
		}
	}
	e.attack("audit_events", "audit_events_no_update_delete", `UPDATE audit_events SET details = '{"changed":true}' WHERE seq = 2`)
	e.rewriteFrom(2)
	_, res := e.verify("owner", "{}")
	if !res.Ok || res.CheckpointsCompared != 0 || res.LastCheckpointSeq != nil {
		t.Fatalf("empty sink: %+v; want ok with nothing compared", res)
	}
}

// TestCheckpointFileTamperIsInvalid: one changed byte in the payload of a sink file gives
// checkpoint_invalid at its seq; a validly signed file of another instance_id too.
func TestCheckpointFileTamperIsInvalid(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.owner("owner")
	e.write(5, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	files := e.sinkFiles()
	if len(files) != 1 {
		t.Fatalf("sink files %v", files)
	}
	path := filepath.Join(e.sinkDir, e.instance.String(), files[0])
	orig, err := os.ReadFile(path) //nolint:gosec // G304: a file of the sink below t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	dot := slices.Index(orig, '.')
	changed := slices.Clone(orig)
	changed[dot+3] ^= 0x01
	if err := os.WriteFile(path, changed, 0o600); err != nil { //nolint:gosec // G703: a file of the sink below t.TempDir
		t.Fatal(err)
	}
	_, res := e.verify("owner", "{}")
	seq := e.headOfFile(files[0])
	if res.Ok || reason(res) != "checkpoint_invalid" || broken(res) != seq {
		t.Fatalf("changed byte: %+v, want checkpoint_invalid at %d", res, seq)
	}

	// The same file, validly signed by this server, but for another instance.
	if err := os.WriteFile(path, e.signForeign(t, orig), 0o600); err != nil { //nolint:gosec // G703: a file of the sink below t.TempDir
		t.Fatal(err)
	}
	_, res = e.verify("owner", "{}")
	if res.Ok || reason(res) != "checkpoint_invalid" || broken(res) != seq {
		t.Fatalf("foreign instance: %+v", res)
	}
	if err := os.WriteFile(path, orig, 0o600); err != nil { //nolint:gosec // G703: a file of the sink below t.TempDir
		t.Fatal(err)
	}
	if _, res := e.verify("owner", "{}"); !res.Ok {
		t.Fatalf("restored file: %+v", res)
	}
}

// TestVerifyRange: from_seq above to_seq is 400; to_seq above the head is capped; a range after
// the first row starts at the stored hash of the row before it.
func TestVerifyRange(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	e.owner("owner")
	e.write(6, actor.Actor{}, "grant.create", "grant", ids.New())
	head := e.head()
	for _, body := range []string{`{"from_seq": 5, "to_seq": 3}`, `{"from_seq": 0}`, `{"to_seq": -1}`, `{"from_seq": 999999}`} {
		if resp, _ := e.verify("owner", body); resp.StatusCode != http.StatusBadRequest || resp.Problem(t)["code"] != "validation_failed" {
			t.Errorf("%s: %d %s, want 400 validation_failed", body, resp.StatusCode, resp.Body)
		}
	}
	resp, res := e.verify("owner", `{"from_seq": 3, "to_seq": 999999}`)
	if resp.StatusCode != http.StatusOK || !res.Ok || res.CheckedFromSeq != 3 || res.CheckedToSeq != head {
		t.Fatalf("capped range: %d %s, want 3 .. %d", resp.StatusCode, resp.Body, head)
	}
	// A change before the range is outside it; a change inside it is found.
	e.attack("audit_events", "audit_events_no_update_delete", `UPDATE audit_events SET details = '{"changed":true}' WHERE seq = 2`)
	if _, res := e.verify("owner", `{"from_seq": 4}`); !res.Ok {
		t.Errorf("a change before the range: %+v", res)
	}
	if _, res := e.verify("owner", `{"from_seq": 2, "to_seq": 3}`); res.Ok || broken(res) != 2 {
		t.Errorf("a change inside the range: %+v", res)
	}
}
