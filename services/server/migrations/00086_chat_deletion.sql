-- SPDX-License-Identifier: Apache-2.0
-- Deleting spaces, channels and chats (FEAT-delete-spaces-chats, docs/design/delete.md,
-- docs/spec/03-data-model.md R-DM-57): spaces, channels and threads get deleted_at, deleted_by and
-- purged_at; project_bindings gets the last folder check of the serving runner (trash_check_*) and
-- the folder move request (trash_*) of 09 section 18.15.
--
-- A deleted space or channel is also archived (D-DEL-2), so deleted_at implies archived_at there.
-- deleted_at and deleted_by are set together; purged_at only on a deleted row. On a binding the
-- check columns go together, a reason only with a failed check; the request columns go together, a
-- refusal reason only with trash_state refused.
--
-- Expand only (upgrade.md rules M1, M3 and M5): every column is new and nullable without a
-- default, so no rewrite; the checks and foreign keys start NOT VALID and 00087 validates them.
-- The previous binary neither reads nor writes the columns. 00088 builds the partial indexes of
-- the purge job CONCURRENTLY.

-- +goose Up
ALTER TABLE spaces
    ADD COLUMN deleted_at timestamptz,
    ADD COLUMN deleted_by uuid,
    ADD COLUMN purged_at  timestamptz;
ALTER TABLE spaces
    ADD CONSTRAINT spaces_deleted_by_fkey FOREIGN KEY (deleted_by) REFERENCES principals (id) ON DELETE RESTRICT NOT VALID,
    ADD CONSTRAINT spaces_deleted_check CHECK ((deleted_at IS NULL) = (deleted_by IS NULL)) NOT VALID,
    ADD CONSTRAINT spaces_deleted_archived_check CHECK (deleted_at IS NULL OR archived_at IS NOT NULL) NOT VALID,
    ADD CONSTRAINT spaces_purged_check CHECK (purged_at IS NULL OR deleted_at IS NOT NULL) NOT VALID;

ALTER TABLE channels
    ADD COLUMN deleted_at timestamptz,
    ADD COLUMN deleted_by uuid,
    ADD COLUMN purged_at  timestamptz;
ALTER TABLE channels
    ADD CONSTRAINT channels_deleted_by_fkey FOREIGN KEY (deleted_by) REFERENCES principals (id) ON DELETE RESTRICT NOT VALID,
    ADD CONSTRAINT channels_deleted_check CHECK ((deleted_at IS NULL) = (deleted_by IS NULL)) NOT VALID,
    ADD CONSTRAINT channels_deleted_archived_check CHECK (deleted_at IS NULL OR archived_at IS NOT NULL) NOT VALID,
    ADD CONSTRAINT channels_purged_check CHECK (purged_at IS NULL OR deleted_at IS NOT NULL) NOT VALID;

ALTER TABLE threads
    ADD COLUMN deleted_at timestamptz,
    ADD COLUMN deleted_by uuid,
    ADD COLUMN purged_at  timestamptz;
ALTER TABLE threads
    ADD CONSTRAINT threads_deleted_by_fkey FOREIGN KEY (deleted_by) REFERENCES principals (id) ON DELETE RESTRICT NOT VALID,
    ADD CONSTRAINT threads_deleted_check CHECK ((deleted_at IS NULL) = (deleted_by IS NULL)) NOT VALID,
    ADD CONSTRAINT threads_deleted_main_check CHECK (deleted_at IS NULL OR kind <> 'main') NOT VALID,
    ADD CONSTRAINT threads_purged_check CHECK (purged_at IS NULL OR deleted_at IS NOT NULL) NOT VALID;

ALTER TABLE project_bindings
    ADD COLUMN trash_check_ok     boolean,
    ADD COLUMN trash_check_reason text,
    ADD COLUMN trash_checked_at   timestamptz,
    ADD COLUMN trash_state        text,
    ADD COLUMN trash_reason       text,
    ADD COLUMN trash_agent_id     uuid,
    ADD COLUMN trash_requested_at timestamptz;
ALTER TABLE project_bindings
    ADD CONSTRAINT project_bindings_trash_agent_id_fkey FOREIGN KEY (trash_agent_id) REFERENCES agents (principal_id) ON DELETE RESTRICT NOT VALID,
    ADD CONSTRAINT project_bindings_trash_check_check CHECK ((trash_check_ok IS NULL) = (trash_checked_at IS NULL)) NOT VALID,
    ADD CONSTRAINT project_bindings_trash_check_reason_check CHECK (trash_check_reason IS NULL OR trash_check_ok = false) NOT VALID,
    ADD CONSTRAINT project_bindings_trash_state_check CHECK (trash_state IN ('requested', 'trashed', 'refused')) NOT VALID,
    ADD CONSTRAINT project_bindings_trash_request_check CHECK (
        (trash_state IS NULL) = (trash_agent_id IS NULL) AND (trash_state IS NULL) = (trash_requested_at IS NULL)) NOT VALID,
    ADD CONSTRAINT project_bindings_trash_reason_check CHECK (trash_reason IS NULL OR trash_state = 'refused') NOT VALID;

-- +goose Down
ALTER TABLE project_bindings
    DROP CONSTRAINT project_bindings_trash_reason_check,
    DROP CONSTRAINT project_bindings_trash_request_check,
    DROP CONSTRAINT project_bindings_trash_state_check,
    DROP CONSTRAINT project_bindings_trash_check_reason_check,
    DROP CONSTRAINT project_bindings_trash_check_check,
    DROP CONSTRAINT project_bindings_trash_agent_id_fkey;
ALTER TABLE project_bindings
    DROP COLUMN trash_requested_at,
    DROP COLUMN trash_agent_id,
    DROP COLUMN trash_reason,
    DROP COLUMN trash_state,
    DROP COLUMN trash_checked_at,
    DROP COLUMN trash_check_reason,
    DROP COLUMN trash_check_ok;
ALTER TABLE threads
    DROP CONSTRAINT threads_purged_check,
    DROP CONSTRAINT threads_deleted_main_check,
    DROP CONSTRAINT threads_deleted_check,
    DROP CONSTRAINT threads_deleted_by_fkey;
ALTER TABLE threads DROP COLUMN purged_at, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE channels
    DROP CONSTRAINT channels_purged_check,
    DROP CONSTRAINT channels_deleted_archived_check,
    DROP CONSTRAINT channels_deleted_check,
    DROP CONSTRAINT channels_deleted_by_fkey;
ALTER TABLE channels DROP COLUMN purged_at, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE spaces
    DROP CONSTRAINT spaces_purged_check,
    DROP CONSTRAINT spaces_deleted_archived_check,
    DROP CONSTRAINT spaces_deleted_check,
    DROP CONSTRAINT spaces_deleted_by_fkey;
ALTER TABLE spaces DROP COLUMN purged_at, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
