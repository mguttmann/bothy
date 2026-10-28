-- SPDX-License-Identifier: Apache-2.0
-- New project from the app (M4-43): validates the check of project_bindings.provision that 00082
-- added NOT VALID (upgrade.md rule M5). VALIDATE CONSTRAINT takes only SHARE UPDATE EXCLUSIVE, so
-- readiness reports and binding changes keep working meanwhile.

-- +goose Up
ALTER TABLE project_bindings VALIDATE CONSTRAINT project_bindings_provision_check;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00082 removes it.
SELECT 1;
