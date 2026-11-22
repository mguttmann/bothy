-- SPDX-License-Identifier: Apache-2.0
-- Relayed attachments of team copies (FIX-TEAM-CHANNEL-ATTACHMENTS, docs/spec/03-data-model.md
-- section 4.7, docs/spec/09-agents.md 3.9.2): message_attachment_relays puts the files of a
-- message on a team copy of it without a second file. Each row binds one file that hangs on the
-- copy's source to the copy at the source's position; thread_id and channel_id are the copy's
-- (R-DM-43), so file.read of a member's agent decides on the copy's place. message_attachments
-- keeps exactly one row per file, the original; nothing that reads it changes.
-- Retention class: content (03 section 3); the rows go with the copy (ON DELETE CASCADE) and with
-- the file row (ON DELETE CASCADE), and the deletion of the source removes them first
-- (DeleteForMessage).
--
-- Expand only (upgrade.md rules M1 and M3): a new table that the previous binary never reads. The
-- table is new and empty, so its indexes are created with it.
--
-- Grants (migrations/README.md, grant rule): bothy_app reads the rows, inserts them with a copy,
-- changes channel_id on a thread move and deletes them with the copy or its source.

-- +goose Up
CREATE TABLE message_attachment_relays (
    message_id uuid     NOT NULL,
    file_id    uuid     NOT NULL,
    position   smallint NOT NULL,
    thread_id  uuid     NOT NULL,
    channel_id uuid     NOT NULL,
    PRIMARY KEY (message_id, position),
    UNIQUE (message_id, file_id),
    FOREIGN KEY (message_id) REFERENCES messages (id) ON DELETE CASCADE,
    FOREIGN KEY (file_id) REFERENCES files (id) ON DELETE CASCADE
);

-- file.read of a relayed file and the deletion of its source: the relays of one file.
CREATE INDEX message_attachment_relays_file_id_idx ON message_attachment_relays (file_id);
-- Following a thread move: the relays of one thread.
CREATE INDEX message_attachment_relays_thread_id_idx ON message_attachment_relays (thread_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON message_attachment_relays TO bothy_app;

-- +goose Down
DROP TABLE message_attachment_relays;
