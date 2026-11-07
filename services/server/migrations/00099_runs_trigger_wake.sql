-- SPDX-License-Identifier: Apache-2.0
-- Wake runs (AR-04, R-DM-60, docs/spec/09-agents.md 5.8.3): runs.trigger_kind accepts 'wake' and
-- runs gets triggering_wake_id as docs/spec/03-data-model.md section 4.8 states it, with its foreign
-- key to run_wakes (ON DELETE SET NULL); like the other triggering_* columns it may be set only on a
-- run of its kind.
--
-- Expand only (upgrade.md rules M1, M3 and M5): the nullable column without a default needs no
-- rewrite and the previous binary neither reads nor writes it; the wider check replaces the old one
-- NOT VALID, so it holds for every new and changed row at once, and 00100 validates the new
-- constraints without blocking writes. Every stored row keeps its trigger kind.
--
-- Down: the old check comes back only while no run has trigger_kind 'wake'; the step refuses
-- otherwise and names the count, because such a run would break it (the rollback note of
-- services/server/internal/runs/README.md: let pending wake-ups fire or cancel them first).

-- +goose Up
ALTER TABLE runs ADD COLUMN triggering_wake_id uuid;
ALTER TABLE runs ADD CONSTRAINT runs_triggering_wake_id_fkey
    FOREIGN KEY (triggering_wake_id) REFERENCES run_wakes (id) ON DELETE SET NULL NOT VALID;
ALTER TABLE runs ADD CONSTRAINT runs_triggering_wake_check
    CHECK (triggering_wake_id IS NULL OR trigger_kind = 'wake') NOT VALID;
ALTER TABLE runs DROP CONSTRAINT runs_trigger_kind_check;
ALTER TABLE runs ADD CONSTRAINT runs_trigger_kind_check
    CHECK (trigger_kind IN ('message', 'command', 'interaction', 'wake')) NOT VALID;

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    found bigint;
BEGIN
    SELECT count(*) INTO found FROM runs WHERE trigger_kind = 'wake';
    IF found > 0 THEN
        RAISE EXCEPTION 'runs with trigger_kind wake exist: %', found
            USING HINT = 'the rollback of stage AR keeps them; see the rollback note of the runs README';
    END IF;
END
$$;
-- +goose StatementEnd
ALTER TABLE runs DROP CONSTRAINT runs_trigger_kind_check;
ALTER TABLE runs ADD CONSTRAINT runs_trigger_kind_check CHECK (trigger_kind IN ('message', 'command', 'interaction'));
ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_triggering_wake_check;
ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_triggering_wake_id_fkey;
ALTER TABLE runs DROP COLUMN IF EXISTS triggering_wake_id;
