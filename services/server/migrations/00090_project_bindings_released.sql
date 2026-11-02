-- SPDX-License-Identifier: Apache-2.0
-- The project binding of a deleted forum gives its folder free once the deletion is final
-- (FIX-delete-releases-binding, docs/decisions.d/FIX-delete-releases-binding.md, 03
-- project_bindings, R-DM-57): released_at marks a binding of a purged forum that no folder move
-- waits for. The row stays (principle 7); 00091 limits the unique index on lower(workspace_ref)
-- to bindings that are not released.
--
-- Expand only (upgrade.md rules M1 and M3): a new nullable column without a default, so no
-- rewrite; the previous binary neither reads nor writes it. No backfill here (rule M7): the job
-- messaging.purge_deleted releases the bindings of forums purged before this change on its
-- first run.

-- +goose Up
ALTER TABLE project_bindings ADD COLUMN released_at timestamptz;

-- +goose Down
ALTER TABLE project_bindings DROP COLUMN released_at;
