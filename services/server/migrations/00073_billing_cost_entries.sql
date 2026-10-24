-- SPDX-License-Identifier: Apache-2.0
-- Cost entries of the billing module (M2-22): cost_entries exactly as
-- docs/spec/03-data-model.md section 4.9 states it, every column with its type, default and
-- nullability. One entry per model of a finished run; agent, thread, channel, space and session
-- are copies taken at booking time, so a moved thread keeps the channel of its bookings (03). A
-- correction is a new entry whose supersedes_id names the replaced one; the valid sum counts the
-- entries no other entry supersedes. Enumerations are text with CHECK (R-DM-02). Retention class:
-- cost.
--
-- agent_session_id gets its foreign key to agent_sessions here (M2-12 left it to this ticket,
-- D-W-103).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read and insert entries, never update
-- or delete them: entries are append only, because a correction is a new row (03; ruling
-- D-M2-22-1). M2-28 and M7-05 add the grants they need.

-- +goose Up
CREATE TABLE cost_entries (
    id                    uuid        NOT NULL,
    run_id                uuid        NOT NULL,
    agent_id              uuid        NOT NULL,
    thread_id             uuid        NOT NULL,
    channel_id            uuid        NOT NULL,
    space_id              uuid,
    agent_session_id      uuid,
    billing_mode          text        NOT NULL,
    provider              text        NOT NULL,
    model                 text        NOT NULL,
    tokens_in             bigint      NOT NULL DEFAULT 0,
    tokens_out            bigint      NOT NULL DEFAULT 0,
    tokens_cache_read     bigint      NOT NULL DEFAULT 0,
    tokens_cache_write    bigint      NOT NULL DEFAULT 0,
    tokens_cache_write_1h bigint      NOT NULL DEFAULT 0,
    amount_micros         bigint      NOT NULL,
    currency              text        NOT NULL DEFAULT 'USD',
    is_estimate           boolean     NOT NULL DEFAULT true,
    source                text        NOT NULL,
    occurred_at           timestamptz NOT NULL,
    reconciled_at         timestamptz,
    supersedes_id         uuid,
    CONSTRAINT cost_entries_pkey PRIMARY KEY (id),
    CONSTRAINT cost_entries_run_id_fkey FOREIGN KEY (run_id) REFERENCES runs (id) ON DELETE RESTRICT,
    CONSTRAINT cost_entries_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES agents (principal_id) ON DELETE RESTRICT,
    CONSTRAINT cost_entries_thread_id_fkey FOREIGN KEY (thread_id) REFERENCES threads (id) ON DELETE RESTRICT,
    CONSTRAINT cost_entries_channel_id_fkey FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE RESTRICT,
    CONSTRAINT cost_entries_space_id_fkey FOREIGN KEY (space_id) REFERENCES spaces (id) ON DELETE RESTRICT,
    CONSTRAINT cost_entries_agent_session_id_fkey FOREIGN KEY (agent_session_id) REFERENCES agent_sessions (id)
        ON DELETE RESTRICT,
    CONSTRAINT cost_entries_supersedes_id_fkey FOREIGN KEY (supersedes_id) REFERENCES cost_entries (id) ON DELETE RESTRICT,
    CONSTRAINT cost_entries_billing_mode_check CHECK (billing_mode IN ('personal', 'organization')),
    CONSTRAINT cost_entries_source_check CHECK (source IN ('adapter_reported', 'computed', 'unpriced', 'reconciled'))
);

-- An entry is superseded at most once.
CREATE UNIQUE INDEX cost_entries_supersedes_key ON cost_entries (supersedes_id) WHERE supersedes_id IS NOT NULL;
-- Costs per space, agent, thread and run.
CREATE INDEX cost_entries_space_occurred_idx ON cost_entries (space_id, occurred_at);
CREATE INDEX cost_entries_agent_occurred_idx ON cost_entries (agent_id, occurred_at);
CREATE INDEX cost_entries_thread_idx ON cost_entries (thread_id);
CREATE INDEX cost_entries_run_idx ON cost_entries (run_id);
-- The open estimates for the reconciliation (M2-28) and the price_in_use check of closePrice.
CREATE INDEX cost_entries_estimate_occurred_idx ON cost_entries (is_estimate, occurred_at) WHERE is_estimate;

GRANT SELECT, INSERT ON cost_entries TO bothy_app;

-- +goose Down
DROP TABLE cost_entries;
