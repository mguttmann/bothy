-- SPDX-License-Identifier: Apache-2.0
-- Stall marking (M2-51, docs/spec/03-data-model.md runs, R-DM-53; 09-agents.md section 5.5): the
-- server time of the last successful run.start or stored patch. runs.stall_check compares it with
-- run.stall_after and sets stall_warned_at for a running run without progress.
--
-- No backfill: an active run without a value is never marked; it gets one with its next start or
-- stored patch (ruling D-M2-51-2). A backfill from started_at would mark every long running run of
-- an old adapter at once, although nothing about its progress is known.

-- +goose Up
ALTER TABLE runs ADD COLUMN last_progress_at timestamptz;

-- +goose Down
ALTER TABLE runs DROP COLUMN last_progress_at;
