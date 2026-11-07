-- SPDX-License-Identifier: Apache-2.0
-- Wake-ups of the runs module (AR-04): run_wakes exactly as docs/spec/03-data-model.md section 4.8
-- states it (R-DM-60, docs/spec/09-agents.md 5.8), every column with its type, default and
-- nullability. A wake-up continues a thread later without a new message; the server fires `at`
-- wake-ups with the job runs.wake_due, the runner fires `pr_checks` and `command` wake-ups after
-- run.wake.check. Enumerations are text with CHECK (R-DM-02). Retention class: content (03 section 3).
--
-- runs.triggering_wake_id and its foreign key come with 00099, which widens runs.trigger_kind.
--
-- Expand only (upgrade.md rule M1): a new table that the previous binary never reads; its indexes
-- are built in the same transaction, because the table is empty.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update run_wakes, not
-- delete them: a wake-up ends with a state other than pending and stays as the history of its
-- thread; the rows go with their thread or source run (ON DELETE CASCADE).

-- +goose Up
CREATE TABLE run_wakes (
    id                uuid        NOT NULL,
    agent_id          uuid        NOT NULL,
    thread_id         uuid        NOT NULL,
    source_run_id     uuid        NOT NULL,
    triggered_by      uuid        NOT NULL,
    kind              text        NOT NULL,
    condition         jsonb       NOT NULL,
    label             text        NOT NULL DEFAULT '',
    reason            text        NOT NULL,
    outage            integer,
    not_before        timestamptz NOT NULL,
    deadline          timestamptz NOT NULL,
    state             text        NOT NULL DEFAULT 'pending',
    state_reason      text,
    result_text       text        NOT NULL DEFAULT '',
    fired_run_id      uuid,
    fired_into_run_id uuid,
    created_at        timestamptz NOT NULL DEFAULT now(),
    ended_at          timestamptz,
    version           bigint      NOT NULL DEFAULT 1,
    CONSTRAINT run_wakes_pkey PRIMARY KEY (id),
    CONSTRAINT run_wakes_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES agents (principal_id) ON DELETE RESTRICT,
    CONSTRAINT run_wakes_thread_id_fkey FOREIGN KEY (thread_id) REFERENCES threads (id) ON DELETE CASCADE,
    CONSTRAINT run_wakes_source_run_id_fkey FOREIGN KEY (source_run_id) REFERENCES runs (id) ON DELETE CASCADE,
    CONSTRAINT run_wakes_triggered_by_fkey FOREIGN KEY (triggered_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT run_wakes_fired_run_id_fkey FOREIGN KEY (fired_run_id) REFERENCES runs (id) ON DELETE SET NULL,
    CONSTRAINT run_wakes_fired_into_run_id_fkey FOREIGN KEY (fired_into_run_id) REFERENCES runs (id) ON DELETE SET NULL,
    CONSTRAINT run_wakes_kind_check CHECK (kind IN ('at', 'pr_checks', 'command')),
    CONSTRAINT run_wakes_reason_check CHECK (reason IN ('agent', 'recovery', 'usage_limit')),
    CONSTRAINT run_wakes_state_check CHECK (state IN ('pending', 'fired', 'cancelled', 'expired', 'refused', 'failed')),
    CONSTRAINT run_wakes_ended_check CHECK ((state = 'pending') = (ended_at IS NULL)),
    CONSTRAINT run_wakes_window_check CHECK (not_before <= deadline),
    CONSTRAINT run_wakes_outage_check CHECK ((reason = 'usage_limit') OR outage IS NULL)
);

-- One run.wake.recover per source run and reason (its idempotency key, 09 5.8.2).
CREATE UNIQUE INDEX run_wakes_recover_key ON run_wakes (source_run_id, reason)
    WHERE reason = 'recovery' OR (reason = 'usage_limit' AND outage IS NULL);
-- One usage-limit wake-up per outage of a run (18.19.10).
CREATE UNIQUE INDEX run_wakes_outage_key ON run_wakes (source_run_id, outage) WHERE outage IS NOT NULL;
-- The due `at` wake-ups of runs.wake_due.
CREATE INDEX run_wakes_due_idx ON run_wakes (not_before) WHERE state = 'pending' AND kind = 'at';
-- The expiry of runs.wake_due.
CREATE INDEX run_wakes_deadline_idx ON run_wakes (deadline) WHERE state = 'pending';
-- run.wake.list_pending of an agent.
CREATE INDEX run_wakes_agent_pending_idx ON run_wakes (agent_id) WHERE state = 'pending';
-- The wake-ups of a thread (run.wake.read) and the pending cap.
CREATE INDEX run_wakes_thread_created_idx ON run_wakes (thread_id, created_at DESC);

GRANT SELECT, INSERT, UPDATE ON run_wakes TO bothy_app;

-- +goose Down
DROP TABLE run_wakes;
