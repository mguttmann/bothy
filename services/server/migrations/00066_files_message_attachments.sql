-- SPDX-License-Identifier: Apache-2.0
-- Attachments of messages (M3-07): message_attachments exactly as docs/spec/03-data-model.md
-- section 4.7 states it. Each row binds one file to one message at a position; thread_id and
-- channel_id are copies of the message's thread and channel (R-DM-43), written by files when it
-- attaches and changed along by files in the transaction of a thread move, so files filters
-- attached files by the place of their message without reading messages (03 principle 12). The
-- copies deliberately have no foreign key: the row hangs on its message through message_id (ON
-- DELETE CASCADE), whose own foreign keys hold thread and channel. The table is new and empty,
-- so every constraint and index is created with it.
-- Retention class: content (03 section 3); the rows go with their message (12 section 12).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert, update (channel_id on a
-- move) and delete (the deletion of a message) the rows.

-- +goose Up
CREATE TABLE message_attachments (
    message_id uuid     NOT NULL,
    file_id    uuid     NOT NULL,
    position   smallint NOT NULL,
    thread_id  uuid     NOT NULL,
    channel_id uuid     NOT NULL,
    PRIMARY KEY (message_id, position),
    UNIQUE (file_id),
    FOREIGN KEY (message_id) REFERENCES messages (id) ON DELETE CASCADE,
    FOREIGN KEY (file_id) REFERENCES files (id) ON DELETE RESTRICT
);

-- Following a thread move: the copies of one thread.
CREATE INDEX message_attachments_thread_id_idx ON message_attachments (thread_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON message_attachments TO bothy_app;

-- +goose Down
DROP TABLE message_attachments;
