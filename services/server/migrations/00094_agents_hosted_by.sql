-- SPDX-License-Identifier: Apache-2.0
-- Hosted agents (A1-03): agents gets hosted_by as docs/spec/03-data-model.md section 4.4 states it
-- (R-DM-58): the Claude Code runner that hosts a built-in agent (adapter_kind claude_agent,
-- docs/spec/26-builtin-agents.md 3.1), NULL for every other agent.
--
-- Expand only (upgrade.md rules M1, M3 and M5): a nullable column without a default needs no
-- rewrite; the foreign key and the two checks are added NOT VALID, so they hold for every new
-- and changed row at once, and 00095 validates them without blocking writes; 00096 builds the
-- partial index concurrently (rule M4). The previous binary neither reads nor writes the column.
--
-- The check (adapter_kind = 'claude_agent') = (hosted_by IS NOT NULL) would fail on a row that
-- already carries the new adapter kind. The value is new with this stage, so such a row is not
-- expected; the precheck refuses the migration with the ids of those agents instead of changing
-- any data (docs/decisions.d/A1-03.md D-A1-03-1).

-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    found text;
BEGIN
    SELECT string_agg(principal_id::text, ', ' ORDER BY principal_id) INTO found
    FROM agents WHERE adapter_kind = 'claude_agent';
    IF found IS NOT NULL THEN
        RAISE EXCEPTION 'agents with adapter_kind claude_agent exist before hosted_by: %', found
            USING HINT = 'change their adapter_kind or delete them, then run the migration again';
    END IF;
END
$$;
-- +goose StatementEnd

ALTER TABLE agents ADD COLUMN hosted_by uuid;
ALTER TABLE agents ADD CONSTRAINT agents_hosted_by_fkey
    FOREIGN KEY (hosted_by) REFERENCES agents (principal_id) ON DELETE RESTRICT NOT VALID;
ALTER TABLE agents ADD CONSTRAINT agents_hosted_by_check
    CHECK ((adapter_kind = 'claude_agent') = (hosted_by IS NOT NULL)) NOT VALID;
ALTER TABLE agents ADD CONSTRAINT agents_hosted_by_not_self_check
    CHECK (hosted_by <> principal_id) NOT VALID;

-- +goose Down
ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_hosted_by_not_self_check;
ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_hosted_by_check;
ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_hosted_by_fkey;
ALTER TABLE agents DROP COLUMN IF EXISTS hosted_by;
