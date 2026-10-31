-- SPDX-License-Identifier: Apache-2.0
-- Personal sidebar order (M4-51): user_space_states gets channel_order and user_channel_states
-- holds the personal order of the posts of a forum, exactly as docs/spec/03-data-model.md section
-- 4.5 states them (04 section 14). Retention class of user_channel_states: personal_sync.
--
-- Expand only (upgrade.md rules M1 and M5): a new nullable column without a default needs no
-- rewrite and no scan, and the new table is empty. The previous binary neither reads nor writes
-- either.
--
-- Grants (migrations/README.md, grant rule): as user_space_states, bothy_app may read, insert,
-- update and delete user_channel_states (personal data, erasure deletes it, M7-06).

-- +goose Up
ALTER TABLE user_space_states ADD COLUMN channel_order uuid[];

CREATE TABLE user_channel_states (
    user_id              uuid        NOT NULL,
    channel_id           uuid        NOT NULL,
    thread_order         uuid[]      NOT NULL DEFAULT '{}',
    version              bigint      NOT NULL DEFAULT 1,
    updated_by_device_id uuid,
    updated_at           timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, channel_id),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE CASCADE,
    FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE CASCADE,
    FOREIGN KEY (updated_by_device_id) REFERENCES devices (id) ON DELETE SET NULL
);

GRANT SELECT, INSERT, UPDATE, DELETE ON user_channel_states TO bothy_app;

-- +goose Down
DROP TABLE user_channel_states;
ALTER TABLE user_space_states DROP COLUMN channel_order;
