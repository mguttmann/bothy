// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// maxPGTimeout is the bound of the deadlines PostgreSQL itself enforces: they are int32
// milliseconds (about 24.8 days), so 24d is the largest whole-day value it accepts.
const maxPGTimeout = "24d"

// minDeadline is the smallest deadline of a server connection: PostgreSQL reads 0 as "no
// deadline", which 02 "Deadlines" rules out for api and worker.
const minDeadline = "1ms"

// databaseConfigKeys returns the keys of the pool (M1-12): the connection data of
// docs/spec/16-operations.md "Configuration values of this chapter" and the deadlines of
// docs/spec/02-architecture.md "Deadlines". database.tx_retry_max (M1-13) and
// database.listen_reconnect_backoff_cap (M1-16) are registered with the others, as the brief
// asks for all database keys.
func databaseConfigKeys() []config.Key {
	return []config.Key{
		{Name: "database.host", Kind: config.KindString, Default: "localhost", Chapter: chapter16,
			Doc: "Host name or address of the database, or the directory of its Unix socket when it starts with a slash."},
		{Name: "database.port", Kind: config.KindInt, Default: "5432", Min: "1", Max: "65535", Chapter: chapter16,
			Doc: "Port of the database in the network of the server."},
		{Name: "database.name", Kind: config.KindString, Default: "bothy", Chapter: chapter16,
			Doc: "Name of the database."},
		{Name: "database.user", Kind: config.KindString, Default: "bothy_app", Chapter: chapter16,
			Doc: "Database role of the process: bothy_app for api and worker, bothy_owner for bothy migrate."},
		{Name: "database.password_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_app_password", Chapter: chapter16,
			Doc: "Secret reference of the password of database.user, read again for every new connection."},
		{Name: "database.sslmode", Kind: config.KindEnum, Default: "verify-full", Enum: []string{"disable", "verify-full"}, Chapter: chapter16,
			Doc: "TLS to the database: verify-full, or disable only inside the internal Compose network (SEC-TLS-5)."},
		{Name: "database.sslrootcert", Kind: config.KindString, Default: "", Chapter: chapter16,
			Doc: "PEM file with the CA certificates for verify-full; empty uses the system roots."},
		{Name: "database.max_conns", Kind: config.KindInt, Default: "20", Min: "1", Max: "2147483647", Chapter: chapter16,
			Doc: "Size of the connection pool of each process."},
		{Name: "database.acquire_timeout", Kind: config.KindDuration, Default: "2s", Min: minDeadline, Chapter: chapter02,
			Doc: "How long a request waits for a pool connection before it gets 503 unavailable."},
		{Name: "database.statement_timeout", Kind: config.KindDuration, Default: "5s", Min: minDeadline, Max: maxPGTimeout, Chapter: chapter02,
			Doc: "Deadline of a single SQL statement, overridable per transaction."},
		{Name: "database.lock_timeout", Kind: config.KindDuration, Default: "2s", Min: minDeadline, Max: maxPGTimeout, Chapter: chapter02,
			Doc: "How long a statement waits for a lock."},
		{Name: "database.idle_in_tx_timeout", Kind: config.KindDuration, Default: defaultTenSeconds, Min: minDeadline, Max: maxPGTimeout, Chapter: chapter02,
			Doc: "How long an open transaction may sit without work before the database ends the session."},
		{Name: "database.tx_retry_max", Kind: config.KindInt, Default: "2", Min: "0", Chapter: chapter02,
			Doc: "Retries of a unit of work after a deadlock or a lock timeout."},
		{Name: "database.listen_reconnect_backoff_cap", Kind: config.KindDuration, Default: defaultTenSeconds, Min: minDeadline, Chapter: chapter02,
			Doc: "Longest pause between attempts to reconnect the LISTEN connection."},
		{Name: "database.restore_guard", Kind: config.KindEnum, Default: "on", Enum: []string{"on", "off"}, Chapter: chapter16,
			Doc: "Whether a server whose database identity differs from the stored one stays not ready (restore_not_finalized) until bothy restore finalize; off only where a failover loses no committed transaction."},
		{Name: "startup.dependency_wait", Kind: config.KindDuration, Default: "60s", Min: "0s", Chapter: chapter02,
			Doc: "How long the start and bothy migrate wait for the database before they give up."},
	}
}

// migrateConfigKeys returns the keys of bothy migrate from docs/spec/16-operations.md
// "Configuration values of this chapter" (brief M1-12 D3). The password of bothy_backup and the
// recent-backup check of migrate up (16 10 item 1) came with the table backup_runs (M7-02).
func migrateConfigKeys() []config.Key {
	return []config.Key{
		{Name: "migrate.lock_timeout", Kind: config.KindDuration, Default: "5s", Min: minDeadline, Max: maxPGTimeout, Chapter: chapter16,
			Doc: "lock_timeout of every migration step, so a step that waits for a lock fails instead of blocking the application."},
		{Name: "migrate.bootstrap_user", Kind: config.KindString, Default: "postgres", Chapter: chapter16,
			Doc: "Superuser login used only by bothy migrate up --bootstrap."},
		{Name: "migrate.bootstrap_password_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_superuser_password", Chapter: chapter16,
			Doc: "Secret reference of the password of migrate.bootstrap_user."},
		{Name: "migrate.owner_password_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_owner_password", Chapter: chapter16,
			Doc: "Secret reference of the password bothy migrate up --bootstrap sets for bothy_owner."},
		{Name: "migrate.app_password_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_app_password", Chapter: chapter16,
			Doc: "Secret reference of the password bothy migrate up --bootstrap sets for bothy_app."},
		{Name: "migrate.maint_password_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_maint_password", Chapter: chapter16,
			Doc: "Secret reference of the password bothy migrate up --bootstrap sets for bothy_maint."},
		{Name: "migrate.monitor_password_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_monitor_password", Chapter: chapter16,
			Doc: "Secret reference of the password bothy migrate up --bootstrap sets for bothy_monitor."},
		{Name: "migrate.backup_password_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_backup_password", Chapter: chapter16,
			Doc: "Secret reference of the password bothy migrate up --bootstrap sets for bothy_backup."},
		{Name: "migrate.require_recent_backup", Kind: config.KindBool, Default: "true", Chapter: chapter16,
			Doc: "Whether bothy migrate up refuses pending migrations without a successful backup younger than migrate.recent_backup_max_age."},
		{Name: "migrate.recent_backup_max_age", Kind: config.KindDuration, Default: "24h", Min: "1m", Chapter: chapter16,
			Doc: "Largest age of the last successful pg_full, pg_diff or logical backup that bothy migrate up accepts."},
	}
}
