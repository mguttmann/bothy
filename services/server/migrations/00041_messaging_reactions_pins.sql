-- SPDX-License-Identifier: Apache-2.0
-- Reactions and pins (M1-59): reactions and pins exactly as docs/spec/03-data-model.md states
-- them. A reaction is one normalised Unicode grapheme cluster of at most reaction.emoji_max_bytes
-- bytes, checked in the application; it is never a status. A pin is a moderation of a holder of
-- manage, at most one per message. Deleting a message removes its reactions and its pin in the
-- same transaction (03 messages); the cascades cover a hard delete. Retention class: content.
--
-- Grants (migrations/README.md, grant rule): bothy_app reads, inserts and deletes rows of both
-- tables; a row is never changed.

-- +goose Up
CREATE TABLE reactions (
    message_id   uuid        NOT NULL,
    principal_id uuid        NOT NULL,
    emoji        text        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, principal_id, emoji),
    FOREIGN KEY (message_id) REFERENCES messages (id) ON DELETE CASCADE,
    FOREIGN KEY (principal_id) REFERENCES principals (id) ON DELETE RESTRICT
);

CREATE TABLE pins (
    message_id uuid        NOT NULL,
    thread_id  uuid        NOT NULL,
    pinned_by  uuid        NOT NULL,
    pinned_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id),
    FOREIGN KEY (message_id) REFERENCES messages (id) ON DELETE CASCADE,
    FOREIGN KEY (thread_id) REFERENCES threads (id) ON DELETE CASCADE,
    FOREIGN KEY (pinned_by) REFERENCES principals (id) ON DELETE RESTRICT
);

CREATE INDEX pins_thread_pinned_at_idx ON pins (thread_id, pinned_at DESC);

GRANT SELECT, INSERT, DELETE ON reactions TO bothy_app;
GRANT SELECT, INSERT, DELETE ON pins TO bothy_app;

-- +goose Down
DROP TABLE pins;
DROP TABLE reactions;
