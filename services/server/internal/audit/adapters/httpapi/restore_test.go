// SPDX-License-Identifier: Apache-2.0

//go:build unix

package httpapi_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/audit/adapters/filesink"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// restoreTo puts the audit tables back to the state of a backup taken at head and newest
// checkpoint cp, as a point-in-time restore does: later rows and checkpoints are gone and the
// counter audit_seq stands at head again, so new rows reuse the seqs of the lost ones.
func (e *env) restoreTo(head, cp int64) {
	e.t.Helper()
	e.attack("audit_events", "audit_events_no_update_delete", `DELETE FROM audit_events WHERE seq > $1`, head)
	e.attack("audit_checkpoints", "audit_checkpoints_no_update_delete", `DELETE FROM audit_checkpoints WHERE seq > $1`, cp)
	e.attack("sequence_counters", "sequence_counters_monotonic", `UPDATE sequence_counters SET value = $1 WHERE name = 'audit_seq'`, head)
}

// copySinkUpTo copies the checkpoint files of the instance with a seq up to cp into a new sink
// root and returns the root: the operator path after a restore (docs/operations/backup-restore.md
// 5.2 and 8): a new sink location that holds the checkpoints of the restored database only, the
// old one kept as evidence.
func (e *env) copySinkUpTo(cp int64) string {
	e.t.Helper()
	root := e.t.TempDir()
	dst := filepath.Join(root, e.instance.String())
	if err := os.Mkdir(dst, 0o700); err != nil {
		e.t.Fatal(err)
	}
	for _, name := range e.sinkFiles() {
		if e.headOfFile(name) > cp {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(e.sinkDir, e.instance.String(), name)) //nolint:gosec // G304: a file of the sink below t.TempDir
		if err != nil {
			e.t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, name), raw, 0o600); err != nil {
			e.t.Fatal(err)
		}
	}
	return root
}

// TestRestoreBreaksTheChainUntilTheSinkMoves (srv-kernel-audit-privacy-2): after a restore to an
// earlier state the sink holds checkpoints newer than the database, so audit.chain_verify reports
// checkpoint_mismatch at every run and signs nothing. The documented operator path, a new sink
// location with the checkpoints up to the newest one of the restored database, makes the job sign
// again and verifyAuditChain verify the whole log; the old location stays untouched.
func TestRestoreBreaksTheChainUntilTheSinkMoves(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	owner := e.owner("owner")
	e.write(3, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	seqs, _ := e.checkpointRows()
	backupCP := seqs[len(seqs)-1]
	e.write(2, actor.Actor{}, "grant.create", "grant", ids.New())
	backupHead := e.head()
	// After the backup: more rows and a checkpoint that reaches the sink.
	e.write(4, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	oldFiles := e.sinkFiles()
	seqs, _ = e.checkpointRows()
	lostCP := seqs[len(seqs)-1]

	e.restoreTo(backupHead, backupCP)
	e.write(5, actor.Actor{}, "grant.create", "grant", ids.New())
	e.run()
	if n := e.errorEntries(lostCP, "checkpoint_mismatch"); n != 1 {
		t.Fatalf("%d checkpoint_mismatch entries at %d after the restore, want 1", n, lostCP)
	}

	root := e.copySinkUpTo(backupCP)
	sink, err := filesink.New(root, e.instance)
	if err != nil {
		t.Fatal(err)
	}
	moved := e.logWithSink(sink)
	if err := moved.RunCheckpoint(context.Background()); err != nil {
		t.Fatalf("RunCheckpoint over the new sink: %v", err)
	}
	after, _ := e.checkpointRows()
	if after[len(after)-1] != e.head() && after[len(after)-1] != e.head()-1 {
		t.Fatalf("checkpoints %v, head %d: the job did not sign over the new sink", after, e.head())
	}
	res, err := moved.Verify(actor.WithActor(context.Background(), owner), audit.VerifyRequest{})
	if err != nil || !res.OK {
		t.Fatalf("verify over the new sink: %+v, %v", res, err)
	}
	if got := e.sinkFiles(); len(got) != len(oldFiles) {
		t.Fatalf("the old sink changed: %v, was %v", got, oldFiles)
	}
}
