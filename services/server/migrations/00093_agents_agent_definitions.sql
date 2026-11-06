-- SPDX-License-Identifier: Apache-2.0
-- Definitions of built-in agents (A1-02): agent_definitions exactly as docs/spec/03-data-model.md
-- section 4.4 states it (R-DM-59, docs/spec/26-builtin-agents.md 4.1). One row per version, never
-- updated; the highest version per agent is the current definition, older versions stay for audit
-- and undo. The use case checks that the agent is a built-in agent (adapter_kind claude_agent)
-- and writes a version in the transaction of agent.update after locking the agents row, so the
-- primary key alone orders the versions and no other index is needed. Retention class: config.
--
-- Expand only (upgrade.md rules M1 and M5): a new, empty table that the previous binary neither
-- reads nor writes; no other table changes.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read and insert. A version is never
-- updated (R-DM-59), and agents and principals are never hard deleted by the server, so the
-- restricting foreign keys never fire in operation.

-- +goose Up
CREATE TABLE agent_definitions (
    agent_id    uuid        NOT NULL,
    version     bigint      NOT NULL,
    definition  jsonb       NOT NULL,
    origin_kind text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    created_by  uuid        NOT NULL,
    CONSTRAINT agent_definitions_pkey PRIMARY KEY (agent_id, version),
    CONSTRAINT agent_definitions_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES agents (principal_id) ON DELETE RESTRICT,
    CONSTRAINT agent_definitions_created_by_fkey FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT agent_definitions_origin_kind_check CHECK (origin_kind IN ('template', 'interview', 'manual'))
);

GRANT SELECT, INSERT ON agent_definitions TO bothy_app;

-- +goose Down
DROP TABLE agent_definitions;
