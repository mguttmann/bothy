-- SPDX-License-Identifier: Apache-2.0
-- "View as person" on the console session (M6-01): the columns auth_sessions.view_as_user_id and
-- view_as_started_at with their foreign key and checks exactly as docs/spec/03-data-model.md
-- section 4.2 states them (R-DM-49), without deviation. Only a console session stores a view, and
-- target and start are set and cleared together. The grants of 00011 cover the new columns.

-- +goose Up
ALTER TABLE auth_sessions
    ADD COLUMN view_as_user_id    uuid,
    ADD COLUMN view_as_started_at timestamptz,
    ADD CONSTRAINT auth_sessions_view_as_user_id_fkey
        FOREIGN KEY (view_as_user_id) REFERENCES users (principal_id) ON DELETE RESTRICT,
    -- Target and start of a view are set and cleared together.
    ADD CONSTRAINT auth_sessions_view_as_check CHECK ((view_as_user_id IS NULL) = (view_as_started_at IS NULL)),
    -- Only the console stores a view.
    ADD CONSTRAINT auth_sessions_view_as_console_check CHECK (view_as_user_id IS NULL OR client_component = 'console');

-- +goose Down
ALTER TABLE auth_sessions
    DROP CONSTRAINT auth_sessions_view_as_console_check,
    DROP CONSTRAINT auth_sessions_view_as_check,
    DROP CONSTRAINT auth_sessions_view_as_user_id_fkey,
    DROP COLUMN view_as_started_at,
    DROP COLUMN view_as_user_id;
