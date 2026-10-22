// SPDX-License-Identifier: Apache-2.0

//go:build imagetest

package postgresimage_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// serveArgs are the docker run arguments of bothy serve --role=api against postgres: the server
// configuration as bothy_app, the secrets of the service server, a key store and a file root
// owned by the image's user 999 (the user the secret volumes belong to), telemetry and the scan
// off because the test network has neither a collector nor clamd.
func serveArgs(image, bin, secServer, keys, files string) []string {
	return append(serverEnv("bothy_app", "db_app_password"),
		"--user", "999:999",
		"-e", "BOTHY_TELEMETRY__ENABLED=false", "-e", "BOTHY_TELEMETRY__LOGS__EXPORTER=stdout",
		"-e", "BOTHY_FILE__SCAN__MODE=off", "-e", "BOTHY_FILE__STORAGE__DRIVER=fs",
		"-v", secServer+":/run/bothy/secrets:ro", "-v", keys+":/var/lib/bothy/signing-keys",
		"-v", files+":/var/lib/bothy/files", "-v", bin+":/opt/bothy:ro",
		"--entrypoint", "/opt/bothy/bothy", image, "serve", "--role=api")
}

// readyz returns the status line and body of /readyz of the internal listener of a serve
// container, read with bash inside it (the image has no HTTP client and no port is published).
func readyz(t *testing.T, container string) string {
	t.Helper()
	out, _, _ := docker(t, "exec", container, "bash", "-c",
		`exec 3<>/dev/tcp/127.0.0.1/8081 && printf 'GET /readyz HTTP/1.0\r\nHost: 127.0.0.1\r\n\r\n' >&3 && cat <&3`)
	return out
}

// TestStartAfterPITRWithoutFinalize (brief M7-04, R-DM-50): a point-in-time restore with the
// image of M7-01 raises the timeline, so bothy serve against the restored database stays not
// ready with restore_not_finalized; once api is stopped, bothy restore finalize stores the new
// identity and a new epoch, and the next start becomes ready.
func TestStartAfterPITRWithoutFinalize(t *testing.T) {
	w := newWorld(t)
	bin := buildServerBinary(t)
	vol := secrets(t)
	secPostgres := w.secretVolume("sec-postgres", filepath.Join(vol, "postgres"))
	secBackup := w.secretVolume("sec-backup", filepath.Join(vol, "backup"))
	secMigrate := w.secretVolume("sec-migrate", filepath.Join(vol, "migrate"))
	secServer := w.secretVolume("sec-server", filepath.Join(vol, "server"))
	pgdata := w.volume("pgdata")
	keys, files := w.volume("signing-keys"), w.volume("files")
	if out, code := w.once("--user", "0:0", "--entrypoint", "sh", "-v", keys+":/k", "-v", files+":/f", w.image, "-c",
		"chown 999:999 /k /f && chmod 0700 /k /f"); code != 0 {
		t.Fatalf("prepare the key store and the file root: exit %d:\n%s", code, out)
	}

	backup := w.start("backup", append(serverEnv("bothy_backup", "db_backup_password"),
		"-e", "BOTHY_PGBACKREST_PG_HOST=postgres",
		"-v", secBackup+":/run/bothy/secrets:ro", "-v", w.volume("repo")+":/var/lib/pgbackrest",
		"-v", w.volume("backup-pg")+":/var/lib/postgresql",
		w.image, "bothy-backupd", "--retry", "1s")...)
	db := w.start("postgres", append(databaseArgs(secPostgres, pgdata), w.image)...)
	waitDatabase(t, db, true)
	eventually(t, "the stanza", func() bool { return strings.Contains(logs(t, backup), "stanza "+stanza+" is ready") },
		func() string { return logs(t, backup) })

	migrateArgs := append(serverEnv("bothy_owner", "db_owner_password"),
		"-v", secMigrate+":/run/bothy/secrets:ro", "-v", bin+":/opt/bothy:ro", "--entrypoint", "/opt/bothy/bothy", w.image)
	if out, code := w.once(append(migrateArgs, "migrate", "up", "--bootstrap")...); code != 0 {
		t.Fatalf("bothy migrate up --bootstrap: exit %d:\n%s", code, out)
	}
	stored := psql(t, db, "select pg_system_identifier || '/' || pg_timeline_id from instance")
	if cluster := psql(t, db, "select s.system_identifier || '/' || c.timeline_id from pg_control_system() s, pg_control_checkpoint() c"); stored != cluster {
		t.Fatalf("migrate up stored %q, the cluster is %q", stored, cluster)
	}
	epoch := psql(t, db, "select sync_epoch from instance")

	out := mustDocker(t, "exec", backup, "pgbackrest", "--stanza=bothy", "backup", "--type=full", "--start-fast")
	if !strings.Contains(out, "backup command end: completed successfully") {
		t.Fatalf("full backup output:\n%s", out)
	}
	// pgBackRest picks the backup by its stop time in whole seconds (see the PITR of M7-01).
	time.Sleep(1500 * time.Millisecond)
	target := psql(t, db, `select to_char(clock_timestamp() at time zone 'UTC', 'YYYY-MM-DD HH24:MI:SS.US') || '+00'`)
	time.Sleep(1500 * time.Millisecond)
	// A commit after the target, so recovery reaches it (it stops at the first record past it).
	psql(t, db, "create table after_target (id int)")
	seg := psql(t, db, "select pg_walfile_name(pg_switch_wal())")
	eventually(t, "segment "+seg+" in the repository", func() bool { return archived(t, backup, seg) },
		func() string { return logs(t, db) })

	mustDocker(t, "stop", db)
	restored := w.volume("pgdata-restored")
	if out, code := w.once(append(databaseArgs(secPostgres, restored), w.image,
		"pgbackrest", "--stanza=bothy", "restore", "--type=time", "--target="+target, "--target-action=promote")...); code != 0 {
		t.Fatalf("restore failed (%d):\n%s", code, out)
	}
	mustDocker(t, "rm", "--force", "--volumes", db)
	db2 := w.start("postgres", append(databaseArgs(secPostgres, restored), w.image)...)
	waitDatabase(t, db2, false)
	cluster := psql(t, db2, "select s.system_identifier || '/' || c.timeline_id from pg_control_system() s, pg_control_checkpoint() c")
	if cluster == stored || psql(t, db2, "select pg_system_identifier || '/' || pg_timeline_id from instance") != stored {
		t.Fatalf("after the PITR the cluster is %q and the row stores %q; want a new timeline and the old row", cluster, stored)
	}

	serve := w.start("api", serveArgs(w.image, bin, secServer, keys, files)...)
	eventually(t, "the start of api", func() bool { return strings.Contains(logs(t, serve), "server started, not ready") },
		func() string { return logs(t, serve) })
	if got := readyz(t, serve); !strings.Contains(got, " 503 ") || !strings.Contains(got, `{"status":"unavailable","failed":["restore_not_finalized"]}`) {
		t.Fatalf("/readyz before finalize:\n%s\nlog:\n%s", got, logs(t, serve))
	}
	if l := logs(t, serve); !strings.Contains(l, "database identity differs from the stored one") || !strings.Contains(l, "bothy restore finalize") {
		t.Fatalf("no log line with the runbook step:\n%s", l)
	}

	// restore finalize refuses while api is connected, then runs once it is stopped.
	if out, code := w.once(append(migrateArgs, "restore", "finalize")...); code != 1 || !strings.Contains(out, "bothy_app") {
		t.Fatalf("restore finalize with api running: exit %d:\n%s", code, out)
	}
	mustDocker(t, "stop", serve)
	mustDocker(t, "rm", "--force", serve)
	// A stopped process does not wait for its backends to end, and the command counts them.
	eventually(t, "no bothy_app session after the stop of api", func() bool {
		return psql(t, db2, "select count(*) from pg_stat_activity where usename = 'bothy_app'") == "0"
	}, func() string { return psql(t, db2, "select usename, state from pg_stat_activity") })
	out, code := w.once(append(migrateArgs, "restore", "finalize")...)
	if code != 0 || !regexp.MustCompile(`sync epoch `+epoch+` -> [0-9a-f-]{36}, cursors at head_seq [0-9]+`).MatchString(out) {
		t.Fatalf("restore finalize: exit %d:\n%s", code, out)
	}
	if got := psql(t, db2, "select pg_system_identifier || '/' || pg_timeline_id from instance"); got != cluster {
		t.Fatalf("stored identity %q after finalize, want %q", got, cluster)
	}
	if got := psql(t, db2, "select sync_epoch from instance"); got == epoch {
		t.Fatal("the epoch did not change")
	}

	serve2 := w.start("api", serveArgs(w.image, bin, secServer, keys, files)...)
	eventually(t, "api ready after finalize", func() bool { return strings.Contains(readyz(t, serve2), " 200 ") },
		func() string { return readyz(t, serve2) + "\n" + logs(t, serve2) })
}
