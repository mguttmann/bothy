-- SPDX-License-Identifier: Apache-2.0
-- Notification levels per space, channel or thread (M3-18): notification_preferences as
-- docs/spec/03-data-model.md section 4.10 states it, without deviation. Only the module notify
-- writes it; every query names the person (user_id) or the explicit list of people. The target
-- is polymorphic and has no foreign key (R-DM-16): the application removes the rows when a space
-- or channel is archived. Retention class: personal_sync.

-- +goose Up
CREATE TABLE notification_preferences (
    user_id     uuid        NOT NULL,
    target_kind text        NOT NULL CHECK (target_kind IN ('space', 'channel', 'thread')),
    target_id   uuid        NOT NULL,
    level       text        NOT NULL CHECK (level IN ('all', 'mentions', 'mute')),
    -- NULL means no temporary mute; a time in the past has no effect (13 section 3.1).
    muted_until timestamptz,
    version     bigint      NOT NULL DEFAULT 1,
    updated_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, target_kind, target_id),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE CASCADE
);
-- The cleanup on archiving finds the rows of a target.
CREATE INDEX notification_preferences_target_idx ON notification_preferences (target_kind, target_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON notification_preferences TO bothy_app;

-- +goose Down
DROP TABLE notification_preferences;
