-- SPDX-License-Identifier: Apache-2.0
-- Deleting spaces, channels and chats (FEAT-delete-spaces-chats): validates the checks and foreign
-- keys that 00086 added NOT VALID (upgrade.md rule M5). VALIDATE CONSTRAINT takes only SHARE UPDATE
-- EXCLUSIVE, so reads and writes go on meanwhile; every row of the previous binary has the new
-- columns NULL and passes.

-- +goose Up
ALTER TABLE spaces VALIDATE CONSTRAINT spaces_deleted_by_fkey;
ALTER TABLE spaces VALIDATE CONSTRAINT spaces_deleted_check;
ALTER TABLE spaces VALIDATE CONSTRAINT spaces_deleted_archived_check;
ALTER TABLE spaces VALIDATE CONSTRAINT spaces_purged_check;
ALTER TABLE channels VALIDATE CONSTRAINT channels_deleted_by_fkey;
ALTER TABLE channels VALIDATE CONSTRAINT channels_deleted_check;
ALTER TABLE channels VALIDATE CONSTRAINT channels_deleted_archived_check;
ALTER TABLE channels VALIDATE CONSTRAINT channels_purged_check;
ALTER TABLE threads VALIDATE CONSTRAINT threads_deleted_by_fkey;
ALTER TABLE threads VALIDATE CONSTRAINT threads_deleted_check;
ALTER TABLE threads VALIDATE CONSTRAINT threads_deleted_main_check;
ALTER TABLE threads VALIDATE CONSTRAINT threads_purged_check;
ALTER TABLE project_bindings VALIDATE CONSTRAINT project_bindings_trash_agent_id_fkey;
ALTER TABLE project_bindings VALIDATE CONSTRAINT project_bindings_trash_check_check;
ALTER TABLE project_bindings VALIDATE CONSTRAINT project_bindings_trash_check_reason_check;
ALTER TABLE project_bindings VALIDATE CONSTRAINT project_bindings_trash_state_check;
ALTER TABLE project_bindings VALIDATE CONSTRAINT project_bindings_trash_request_check;
ALTER TABLE project_bindings VALIDATE CONSTRAINT project_bindings_trash_reason_check;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00086 removes them.
SELECT 1;
