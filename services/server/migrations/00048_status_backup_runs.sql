-- SPDX-License-Identifier: Apache-2.0
-- Backup runs (M7-02): backup_runs exactly as docs/spec/03-data-model.md section 4.12 states it.
-- The table is new and empty, so its key, checks and index are created with it. Only the module
-- status reads and writes it: bothy-backupd records its runs through the port BackupRuns with
-- the role bothy_backup, which may write this table and no other (03 4.12, 16 7.1 rule 3); the
-- admin console and the status page read it as bothy_app. Retention class ops (the job follows
-- with M7-09). The index (kind, started_at DESC) serves "the last successful run per kind" of
-- bothy migrate up and of the alerts.

-- +goose Up
CREATE TABLE backup_runs (
    id          uuid        NOT NULL,
    kind        text        NOT NULL
                            CHECK (kind IN ('pg_full', 'pg_diff', 'logical', 'objects', 'restore_test')),
    state       text        NOT NULL DEFAULT 'running'
                            CHECK (state IN ('running', 'succeeded', 'failed')),
    started_at  timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    size_bytes  bigint,
    manifest    jsonb       NOT NULL DEFAULT '{}',
    checks      jsonb       NOT NULL DEFAULT '{}',
    error       text,
    PRIMARY KEY (id),
    CHECK ((state = 'running') = (finished_at IS NULL))
);
CREATE INDEX backup_runs_kind_started_at_idx ON backup_runs (kind, started_at DESC);

GRANT SELECT ON backup_runs TO bothy_app;
GRANT SELECT, INSERT, UPDATE ON backup_runs TO bothy_backup;

-- +goose Down
DROP TABLE backup_runs;
