// SPDX-License-Identifier: Apache-2.0

//go:build imagetest

package postgresimage_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	auditdomain "github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The seeded data of the restore test: enough rows that a gap lies inside a partition and the
// samples check every thread and file.
const (
	restoreEvents  = 40
	restoreAudit   = 12
	restoreMsgs    = 5
	restoreObjects = 4
	// tlsPortHex is the TLS port of the pgBackRest server of backup (8432) as /proc/net/tcp
	// writes it.
	tlsPortHex = "20F0"
	// dockerDNSHex is 127.0.0.11, the address of Docker's embedded DNS server, which listens in
	// every container of a user-defined network.
	dockerDNSHex = "0B00007F"
)

// The seeded people and structure (EXAMPLE ids).
const (
	seedPerson  = "0192f4a0-7b1c-7000-8a00-000000000101"
	seedSpace   = "0192f4a0-7b1c-7000-8a00-000000000102"
	seedChannel = "0192f4a0-7b1c-7000-8a00-000000000103"
	seedThread  = "0192f4a0-7b1c-7000-8a00-000000000104"
)

// buildHookBackupd cross compiles bothy-backupd with the build tag restoretesthook (the test hook
// of internal/status/adapters/scratch, brief M7-03 design note 3) for the platform of the Docker
// daemon and returns its directory. The image itself carries the build without the hook.
func buildHookBackupd(t *testing.T) string {
	t.Helper()
	arch := strings.TrimSpace(mustDocker(t, "info", "--format", "{{.Architecture}}"))
	goarch := map[string]string{"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}[arch]
	dir := t.TempDir()
	// The build is bounded by the timeout of go test only, not by a number of its own.
	ctx := t.Context()
	//nolint:gosec // G204: the Go toolchain of this test run builds this repository's command.
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-tags", "restoretesthook", "-o", filepath.Join(dir, "bothy-backupd"), "../../cmd/bothy-backupd")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+goarch) //nolint:forbidigo // the toolchain's environment
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build -tags restoretesthook ./cmd/bothy-backupd: %v\n%s", err, out)
	}
	if err := os.Chmod(dir, 0o755); err != nil { //nolint:gosec // G302: the container user 999 runs the binary.
		t.Fatal(err)
	}
	return dir
}

// auditInserts returns the INSERT of restoreAudit valid chain rows after the row after (its seq
// and hash), computed with the hash of the module audit, and the new head.
func auditInserts(t *testing.T, after int64, prevHash []byte) (string, int64) {
	t.Helper()
	var prev [auditdomain.HashSize]byte
	copy(prev[:], prevHash)
	var values []string
	at := time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC)
	seq := after
	for i := range restoreAudit {
		seq++
		r := auditdomain.Row{Seq: seq, ID: ids.New(), OccurredAt: at.Add(time.Duration(i) * time.Minute), Action: "grant.create",
			Decision: "allow", Details: map[string]any{}, PrevHash: prev}
		canonical, err := auditdomain.CanonicalRow(r)
		if err != nil {
			t.Fatal(err)
		}
		r.Hash = auditdomain.ChainHash(prev, canonical)
		values = append(values, fmt.Sprintf("(%d, '%s', '%s', 'grant.create', 'allow', '{}'::jsonb, '\\x%s'::bytea, '\\x%s'::bytea)",
			seq, r.ID, auditdomain.FormatTime(r.OccurredAt), hex.EncodeToString(prev[:]), hex.EncodeToString(r.Hash[:])))
		prev = r.Hash
	}
	return `INSERT INTO audit_events (seq, id, occurred_at, action, decision, details, prev_hash, hash) VALUES ` +
		strings.Join(values, ", "), seq
}

// seedRestoreData fills the database of db as the server would: a thread with restoreMsgs
// messages without gaps, restoreEvents events in one partition, restoreAudit audit rows that
// continue the chain, the counters at their heads, and restoreObjects complete files whose
// objects it returns by key.
func seedRestoreData(t *testing.T, db string) map[string]string {
	t.Helper()
	for _, sql := range []string{
		`INSERT INTO principals (id, kind, handle, display_name) VALUES ('` + seedPerson + `', 'user', 'robin', 'Robin')`,
		`INSERT INTO users (principal_id) VALUES ('` + seedPerson + `')`,
		`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ('` + seedSpace + `', 'Support', 'a0', '` + seedPerson + `')`,
		`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
			VALUES ('` + seedChannel + `', '` + seedSpace + `', 'text', 'general', 'a0', 20, '` + seedPerson + `')`,
		`INSERT INTO threads (id, channel_id, kind, created_by, last_message_seq)
			VALUES ('` + seedThread + `', '` + seedChannel + `', 'main', '` + seedPerson + `', ` + strconv.Itoa(restoreMsgs) + `)`,
		`INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind, content_md)
			SELECT gen_random_uuid(), '` + seedThread + `', '` + seedChannel + `', g, '` + seedPerson + `', gen_random_uuid(), '\x00', 'human', 'text', 'hello'
			FROM generate_series(1, ` + strconv.Itoa(restoreMsgs) + `) g`,
		`SELECT sync_event_log_ensure_partitions(1000, 1, ` + strconv.Itoa(restoreEvents) + `)`,
		`INSERT INTO event_log (seq, id, type, resource_kind, receive_action, payload, permission_version)
			SELECT x.m + g, gen_random_uuid(), 'thread.closed', 'thread', 'thread.read', '{}'::jsonb, 0
			FROM generate_series(1, ` + strconv.Itoa(restoreEvents) + `) g, (SELECT COALESCE(max(seq), 0) AS m FROM event_log) x`,
		`UPDATE sequence_counters SET value = (SELECT max(seq) FROM event_log) WHERE name = 'event_seq' AND value < (SELECT max(seq) FROM event_log)`,
	} {
		psql(t, db, sql)
	}
	var after int64
	var prevHash []byte
	if head := psql(t, db, `SELECT seq || ' ' || encode(hash, 'hex') FROM audit_events ORDER BY seq DESC LIMIT 1`); head != "" {
		f := strings.Fields(head)
		after, _ = strconv.ParseInt(f[0], 10, 64)
		prevHash, _ = hex.DecodeString(f[1])
	}
	insert, head := auditInserts(t, after, prevHash)
	psql(t, db, insert)
	psql(t, db, fmt.Sprintf(`UPDATE sequence_counters SET value = %d WHERE name = 'audit_seq' AND value < %d`, head, head))

	objects := map[string]string{}
	for i := range restoreObjects {
		key := fmt.Sprintf("f/%02x/obj-%d", i, i)
		content := fmt.Sprintf("object-%d-EXAMPLE", i)
		sum := sha256.Sum256([]byte(content))
		objects[key] = content
		psql(t, db, fmt.Sprintf(`INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, upload_state, scan_status)
			VALUES (gen_random_uuid(), '%s', '\x%s', %d, 'text/plain', 'a.txt', '%s', 'avatar', 'complete', 'clean')`,
			key, hex.EncodeToString(sum[:]), len(content), seedPerson))
	}
	return objects
}

// restoreRow is the last restore_test row of backup_runs.
type restoreRow struct {
	State  string `json:"state"`
	Error  string `json:"error"`
	Checks struct {
		Schema string `json:"schema"`
		Label  string `json:"pgbackrest_label"`
		Checks []struct {
			Name       string `json:"name"`
			Result     string `json:"result"`
			Reason     string `json:"reason"`
			DurationMS int64  `json:"duration_ms"`
			Items      int64  `json:"items"`
		} `json:"checks"`
	} `json:"checks"`
}

// lastRestoreRow reads the newest restore_test row.
func lastRestoreRow(t *testing.T, db string) restoreRow {
	t.Helper()
	raw := psql(t, db, `SELECT json_build_object('state', state, 'error', coalesce(error, ''), 'checks', checks)
		FROM backup_runs WHERE kind = 'restore_test' ORDER BY started_at DESC LIMIT 1`)
	var r restoreRow
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("restore_test row %q: %v", raw, err)
	}
	return r
}

// outcomes returns name:result[:reason] of the checks of r.
func (r restoreRow) outcomes() []string {
	var out []string
	for _, c := range r.Checks.Checks {
		s := c.Name + ":" + c.Result
		if c.Reason != "" {
			s += ":" + c.Reason
		}
		out = append(out, s)
	}
	return out
}

// listeningPorts returns the local ports in hex of the LISTEN sockets of a /proc/net/tcp dump,
// without Docker's embedded DNS server.
func listeningPorts(dump string) []string {
	var ports []string
	for _, line := range strings.Split(dump, "\n") {
		f := strings.Fields(line)
		if len(f) < 4 || f[3] != "0A" {
			continue
		}
		addr, port, ok := strings.Cut(f[1], ":")
		if !ok || strings.HasSuffix(addr, dockerDNSHex) && len(addr) == 8 {
			continue
		}
		ports = append(ports, port)
	}
	slices.Sort(ports)
	return slices.Compact(ports)
}

// TestRestoreTest runs the automatic restore test of bothy-backupd in the image (brief M7-03,
// ADR 0058 "Confirmation", docs/operations/backup-restore.md 4): after a full backup and an
// object backup of a seeded database, a clean copy passes all seven checks; while the copy runs
// the container has no TCP listener but the pgBackRest TLS server; afterwards the scratch
// directory is empty and backup_runs holds a restore_test row with restore_test_checks.v1. Each
// manipulated copy (one event_log row deleted in the middle of its partition, one stored audit
// hash changed, the bytes of one restored object replaced) fails with its check and reason; the
// deleted event row also changes the row count of event_log, which check 2 compares with the
// manifest (docs/decisions.d/M7-03.md D-M7-03-6).
func TestRestoreTest(t *testing.T) {
	w := newWorld(t)
	bin := buildServerBinary(t)
	hookBin := buildHookBackupd(t)
	vol := secrets(t)
	secPostgres := w.secretVolume("sec-postgres", filepath.Join(vol, "postgres"))
	secBackup := w.secretVolume("sec-backup", filepath.Join(vol, "backup"))
	secMigrate := w.secretVolume("sec-migrate", filepath.Join(vol, "migrate"))
	pgdata := w.volume("pgdata")
	files := w.volume("files")

	db := w.start("postgres", append(databaseArgs(secPostgres, pgdata), w.image)...)
	waitDatabase(t, db, true)
	migrateArgs := append(serverEnv("bothy_owner", "db_owner_password"),
		"-v", secMigrate+":/run/bothy/secrets:ro", "-v", bin+":/opt/bothy:ro", "--entrypoint", "/opt/bothy/bothy",
		w.image, "migrate", "up", "--bootstrap")
	if out, code := w.once(migrateArgs...); code != 0 {
		t.Fatalf("bothy migrate up --bootstrap: exit %d:\n%s", code, out)
	}
	objects := seedRestoreData(t, db)

	// The objects as the file system driver leaves them: owner 65532, directories 0700, files 0600.
	var script []string
	for _, key := range slices.Sorted(maps.Keys(objects)) {
		script = append(script, fmt.Sprintf("mkdir -p /files/%s && printf '%%s' '%s' > /files/%s", filepath.Dir(key), objects[key], key))
	}
	script = append(script, "chown -R 65532:65532 /files", "find /files -type d -exec chmod 0700 {} +", "find /files -type f -exec chmod 0600 {} +")
	if out, code := w.once("--user", "0:0", "--entrypoint", "sh", "-v", files+":/files", w.image, "-c", strings.Join(script, " && ")); code != 0 {
		t.Fatalf("seed the volume files: exit %d:\n%s", code, out)
	}

	backupArgs := append(serverEnv("bothy_backup", "db_backup_password"),
		"-e", "BOTHY_PGBACKREST_PG_HOST=postgres",
		"-e", "BOTHY_FILE__STORAGE__DRIVER=fs", "-e", "BOTHY_FILE__FS__ROOT=/var/lib/bothy/files",
		"-v", secBackup+":/run/bothy/secrets:ro", "-v", w.volume("repo")+":/var/lib/pgbackrest",
		"-v", w.volume("scratch")+":/var/lib/bothy-backup", "-v", files+":/var/lib/bothy/files:ro",
		"-v", pgdata+":/var/lib/postgresql:ro", "-v", hookBin+":/opt/test:ro", "--cap-add", "DAC_READ_SEARCH",
		w.image, "bothy-backupd", "--retry", "1s")
	backup := w.start("backup", backupArgs...)
	eventually(t, "the stanza", func() bool { return strings.Contains(logs(t, backup), "stanza "+stanza+" is ready") },
		func() string { return logs(t, backup) })
	for _, kind := range []string{"pg_full", "objects"} {
		if out, errOut, code := docker(t, "exec", backup, "bothy-backupd", "backup", kind); code != 0 {
			t.Fatalf("bothy-backupd backup %s: exit %d:\n%s%s", kind, code, out, errOut)
		}
	}

	restoreTest := func(t *testing.T, hook string) (int, string, time.Duration) {
		t.Helper()
		start := time.Now()
		out, errOut, code := docker(t, "exec", "-e", "RESTORE_TEST_HOOK="+hook, backup, "/opt/test/bothy-backupd", "restore-test")
		took := time.Since(start)
		if left := strings.TrimSpace(mustDocker(t, "exec", backup, "ls", "-A", "/var/lib/bothy-backup/restore")); left != "" {
			t.Errorf("the scratch directory is not empty after the test: %s", left)
		}
		return code, out + errOut, took
	}

	t.Run("a clean copy passes all seven checks", func(t *testing.T) {
		code, out, took := restoreTest(t, "")
		if code != 0 {
			ls, lsErr, _ := docker(t, "exec", "-e", "RESTIC_PASSWORD_FILE=/run/bothy/secrets/restic_password", backup,
				"restic", "--no-cache", "-r", "/var/lib/pgbackrest/restic", "ls", "latest", "--tag", "objects")
			t.Fatalf("restore-test: exit %d:\n%s\nobjects snapshot:\n%s%s\nlog of backup:\n%s", code, out, ls, lsErr, logs(t, backup))
		}
		row := lastRestoreRow(t, db)
		want := []string{"pgbackrest:passed", "manifest:passed", "event_log:passed", "audit_chain:passed",
			"thread_sample:passed", "file_sample:passed", "duration:passed"}
		if row.State != "succeeded" || row.Error != "" || row.Checks.Schema != "restore_test_checks.v1" || !slices.Equal(row.outcomes(), want) {
			t.Errorf("row %+v\noutcomes %v", row, row.outcomes())
		}
		t.Logf("restore test on the seeded data took %s; checks:", took)
		for _, c := range row.Checks.Checks {
			t.Logf("  %-13s %-7s %6d ms  %d items", c.Name, c.Result, c.DurationMS, c.Items)
		}
		for _, c := range row.Checks.Checks {
			if c.Name == "file_sample" && c.Items != restoreObjects {
				t.Errorf("file sample checked %d objects, want all %d", c.Items, restoreObjects)
			}
		}
		probe := mustDocker(t, "exec", backup, "cat", "/tmp/restore-test-tcp")
		if ports := listeningPorts(probe); !slices.Equal(ports, []string{tlsPortHex}) {
			t.Errorf("listening TCP ports while the copy ran %v, want only the pgBackRest TLS server %s:\n%s", ports, tlsPortHex, probe)
		}
	})

	for _, c := range []struct {
		hook string
		want []string
	}{
		{"event_log_gap", []string{"manifest:failed:row_counts_differ", "event_log:failed:event_log_gap"}},
		{"audit_hash", []string{"audit_chain:failed:hash_mismatch"}},
		{"object_bytes", []string{"file_sample:failed:object_hash_mismatch"}},
	} {
		t.Run("manipulated copy "+c.hook, func(t *testing.T) {
			code, out, _ := restoreTest(t, c.hook)
			if code != 1 {
				t.Fatalf("restore-test with %s: exit %d, want 1:\n%s", c.hook, code, out)
			}
			row := lastRestoreRow(t, db)
			var failed []string
			for _, o := range row.outcomes() {
				if strings.Contains(o, ":failed") {
					failed = append(failed, o)
				} else if !strings.HasSuffix(o, ":passed") {
					t.Errorf("check %s neither passed nor failed", o)
				}
			}
			if row.State != "failed" || row.Error != "checks_failed" || !slices.Equal(failed, c.want) {
				t.Errorf("row %s %s, failed %v, want %v", row.State, row.Error, failed, c.want)
			}
		})
	}
}
