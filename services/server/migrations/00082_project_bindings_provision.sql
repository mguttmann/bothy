-- SPDX-License-Identifier: Apache-2.0
-- New project from the app (M4-43): project_bindings gets provision and ready_reason as
-- docs/spec/03-data-model.md section 4.5 states them (R-DM-55). provision tells the serving runner
-- whether it creates the folder of the binding (none, init, clone; 09 section 18.12); every
-- existing row keeps the default none, which is the behaviour before this migration. ready_reason
-- is the code the runner reported with ready false; NULL for every existing row.
--
-- Expand only (upgrade.md rules M1 and M5): both columns are new, a NOT NULL column with a
-- constant default needs no rewrite, and the check starts NOT VALID, so adding it takes no scan
-- under the exclusive lock; 00083 validates it. The previous binary neither reads nor writes the
-- columns, and its INSERT gets the default.

-- +goose Up
ALTER TABLE project_bindings
    ADD COLUMN provision    text NOT NULL DEFAULT 'none',
    ADD COLUMN ready_reason text;

ALTER TABLE project_bindings
    ADD CONSTRAINT project_bindings_provision_check CHECK (provision IN ('none', 'init', 'clone')) NOT VALID;

-- +goose Down
ALTER TABLE project_bindings DROP CONSTRAINT project_bindings_provision_check;
ALTER TABLE project_bindings DROP COLUMN ready_reason, DROP COLUMN provision;
