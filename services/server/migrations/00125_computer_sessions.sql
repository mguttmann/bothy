-- SPDX-License-Identifier: Apache-2.0
-- Computer sessions of the terminal module (CU-04, R-DM-65, docs/spec/27-computer-use.md): the
-- table computer_sessions exactly as docs/spec/03-data-model.md section 4.14 states it. Metadata
-- only: never an image, a coordinate, typed text or a key name. Retention class security
-- (retention.security_after_end after closed_at), deleted by the participant of the terminal
-- module, hence DELETE for bothy_app.
--
-- A new table: its checks are valid at once and its indexes are built in the same transaction,
-- because no row and no writer exists yet (upgrade.md rules M4 and M5 protect existing tables).
-- The unique index on host_id holds one controller per host (27 8.5), the one on run_id one
-- session per run (R-CU-06).

-- +goose Up
CREATE TABLE computer_sessions (
    id             uuid        NOT NULL,
    host_id        uuid        NOT NULL,
    user_id        uuid        NOT NULL,
    agent_id       uuid        NOT NULL,
    run_id         uuid        NOT NULL,
    state          text        NOT NULL DEFAULT 'opening',
    pause_reason   text,
    token_jti      uuid        NOT NULL,
    generation     integer     NOT NULL,
    key_generation integer     NOT NULL,
    attached_at    timestamptz,
    opened_at      timestamptz NOT NULL DEFAULT now(),
    started_at     timestamptz,
    paused_at      timestamptz,
    paused_ms      bigint      NOT NULL DEFAULT 0,
    closed_at      timestamptz,
    stopped_by     uuid,
    close_reason   text,
    close_detail   text,
    action_count   integer     NOT NULL DEFAULT 0,
    front_app      text,
    version        bigint      NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    FOREIGN KEY (host_id) REFERENCES hosts (id) ON DELETE RESTRICT,
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (agent_id) REFERENCES agents (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (run_id) REFERENCES runs (id) ON DELETE RESTRICT,
    FOREIGN KEY (stopped_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT computer_sessions_token_jti_key UNIQUE (token_jti),
    -- The states of 27 section 8.1.
    CONSTRAINT computer_sessions_state_check CHECK (state IN ('opening', 'active', 'paused', 'closed')),
    CONSTRAINT computer_sessions_pause_reason_check CHECK (pause_reason IN ('locked')),
    -- The closed list of 27 section 8.3.
    CONSTRAINT computer_sessions_close_reason_check CHECK (close_reason IN ('done', 'stopped_by_user',
        'device_disabled', 'run_ended', 'access_revoked', 'host_disabled', 'host_retired', 'host_rekeyed',
        'host_restarted', 'connection_lost', 'open_failed', 'error')),
    CONSTRAINT computer_sessions_closed_check CHECK ((state = 'closed') = (closed_at IS NOT NULL)),
    CONSTRAINT computer_sessions_reason_check CHECK ((state = 'closed') = (close_reason IS NOT NULL)),
    CONSTRAINT computer_sessions_detail_check CHECK (close_detail IS NULL OR state = 'closed'),
    CONSTRAINT computer_sessions_paused_reason_check CHECK ((state = 'paused') = (pause_reason IS NOT NULL)),
    CONSTRAINT computer_sessions_paused_at_check CHECK ((state = 'paused') = (paused_at IS NOT NULL))
);
-- One controller per host and one session per run (27 8.5, R-CU-06).
CREATE UNIQUE INDEX computer_sessions_host_open_key ON computer_sessions (host_id) WHERE state <> 'closed';
CREATE UNIQUE INDEX computer_sessions_run_open_key ON computer_sessions (run_id) WHERE state <> 'closed';
-- The end at a revocation and the status chip read the open sessions of a person.
CREATE INDEX computer_sessions_user_open_idx ON computer_sessions (user_id) WHERE state <> 'closed';
-- The retention of class security reads by age and the candidates by end time.
CREATE INDEX computer_sessions_opened_at_brin ON computer_sessions USING brin (opened_at);
CREATE INDEX computer_sessions_closed_at_idx ON computer_sessions (closed_at, id) WHERE closed_at IS NOT NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON computer_sessions TO bothy_app;

-- +goose Down
DROP TABLE computer_sessions;
