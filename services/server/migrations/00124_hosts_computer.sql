-- SPDX-License-Identifier: Apache-2.0
-- The desktop enablement mirror of computer use (CU-04, R-DM-65, docs/spec/27-computer-use.md 4):
-- hosts gets reported_computer (schema host_computer.v1 of the last report, NULL while the host
-- service reports no computer object) and computer_disabled_generation (the enablement generation
-- that computer.device.disable switched off) exactly as docs/spec/03-data-model.md section 4.14
-- states them; 03 names no check for either column.
--
-- Expand only (upgrade.md rules M1 and M3): two nullable columns without a default need no rewrite,
-- the previous binary neither reads nor writes them, and every stored host keeps its values; a host
-- without them is no desktop (27 section 4 item 3), which is the behaviour before.

-- +goose Up
ALTER TABLE hosts ADD COLUMN reported_computer jsonb;
ALTER TABLE hosts ADD COLUMN computer_disabled_generation integer;

-- +goose Down
ALTER TABLE hosts DROP COLUMN IF EXISTS computer_disabled_generation;
ALTER TABLE hosts DROP COLUMN IF EXISTS reported_computer;
