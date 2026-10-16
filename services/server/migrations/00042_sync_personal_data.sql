-- SPDX-License-Identifier: Apache-2.0
-- Personal sync data (M1-61): read_states, drafts and user_settings as
-- docs/spec/03-data-model.md section 4.10 states them, without deviation. Only the module sync
-- writes them; every query names the person (user_id). Unread and mention counters are never
-- stored (R-DM-32). Retention class of all three: personal_sync.

-- +goose Up
CREATE TABLE read_states (
    user_id              uuid        NOT NULL,
    thread_id            uuid        NOT NULL,
    channel_id           uuid        NOT NULL,
    last_read_seq        bigint      NOT NULL DEFAULT 0,
    version              bigint      NOT NULL DEFAULT 1,
    updated_by_device_id uuid,
    updated_at           timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, thread_id),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE CASCADE,
    FOREIGN KEY (thread_id) REFERENCES threads (id) ON DELETE CASCADE,
    FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE CASCADE,
    FOREIGN KEY (updated_by_device_id) REFERENCES devices (id) ON DELETE SET NULL
);
-- Unread counters per channel.
CREATE INDEX read_states_user_channel_idx ON read_states (user_id, channel_id);

CREATE TABLE drafts (
    id                   uuid        NOT NULL,
    user_id              uuid        NOT NULL,
    -- NULL for a new forum post (a channel draft).
    thread_id            uuid,
    channel_id           uuid        NOT NULL,
    content_md           text        NOT NULL DEFAULT '',
    reply_to_message_id  uuid,
    file_ids             uuid[]      NOT NULL DEFAULT '{}',
    version              bigint      NOT NULL DEFAULT 1,
    updated_by_device_id uuid,
    updated_at           timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE CASCADE,
    FOREIGN KEY (thread_id) REFERENCES threads (id) ON DELETE CASCADE,
    FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE CASCADE,
    FOREIGN KEY (reply_to_message_id) REFERENCES messages (id) ON DELETE SET NULL,
    FOREIGN KEY (updated_by_device_id) REFERENCES devices (id) ON DELETE SET NULL
);
-- One draft per person and thread, and one draft of a new post per person and forum.
CREATE UNIQUE INDEX drafts_user_thread_key ON drafts (user_id, thread_id) WHERE thread_id IS NOT NULL;
CREATE UNIQUE INDEX drafts_user_channel_key ON drafts (user_id, channel_id) WHERE thread_id IS NULL;

CREATE TABLE user_settings (
    user_id              uuid        NOT NULL,
    -- Keys and values of 01 section 11 and 13 section 3.3, checked in the application.
    key                  text        NOT NULL,
    value                jsonb       NOT NULL,
    version              bigint      NOT NULL DEFAULT 1,
    updated_by_device_id uuid,
    updated_at           timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, key),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE CASCADE,
    FOREIGN KEY (updated_by_device_id) REFERENCES devices (id) ON DELETE SET NULL
);

GRANT SELECT, INSERT, UPDATE ON read_states TO bothy_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON drafts TO bothy_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON user_settings TO bothy_app;

-- +goose Down
DROP TABLE user_settings;
DROP TABLE drafts;
DROP TABLE read_states;
