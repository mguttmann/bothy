-- SPDX-License-Identifier: Apache-2.0
-- Messages and mentions (M1-56): messages and mentions as docs/spec/03-data-model.md section 4.6
-- states them. IDs come from kernel/ids (R-DM-01); enumerations are text with CHECK (R-DM-02).
-- thread_seq comes from threads.last_message_seq under the row lock of the thread (03 section 6.2).
-- Retention class: content.
--
-- Deliberate deviations from 03 messages, each added by a later ticket:
--   - messages.search_language, messages.search_vector and its GIN index: M3-13 adds them with
--     the search.
--   - messages.has_attachments and the CHECK deleted_at IS NULL OR NOT has_attachments: M3-07 adds
--     them together with attachments (R-DM-43); until then no message has an attachment.
--   - the foreign key messages.run_id -> runs(id): M2-05 adds it, because runs does not exist yet.
--     The column and the CHECK (kind = 'run') = (run_id IS NOT NULL) exist now.
--   - the foreign key messages.import_item_id -> import_items(id): the import ticket of M8 adds it,
--     because import_items does not exist yet. The column exists now.
-- Every other column, default, CHECK, unique and partial index of the two tables is created as 03
-- states it.
--
-- threads.parent_message_id gets its foreign key here, because messages did not exist when M1-53
-- created threads (00025). It is DEFERRABLE INITIALLY DEFERRED (03 threads) and starts NOT VALID
-- (upgrade.md rule M5); 00033 validates it.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update messages, never
-- delete them (03 principle 7: a deleted message keeps its row as a tombstone); mentions also
-- allows delete, because editing replaces the rows of a message and deleting removes them (03
-- mentions).

-- +goose Up
CREATE TABLE messages (
    id                    uuid        NOT NULL,
    thread_id             uuid        NOT NULL,
    channel_id            uuid        NOT NULL,
    thread_seq            bigint      NOT NULL,
    author_principal_id   uuid        NOT NULL,
    client_msg_id         uuid        NOT NULL,
    client_payload_sha256 bytea       NOT NULL,
    origin                text        NOT NULL,
    kind                  text        NOT NULL DEFAULT 'text',
    system_code           text,
    content_md            text        NOT NULL DEFAULT '',
    content_format        smallint    NOT NULL DEFAULT 1,
    reply_to_message_id   uuid,
    -- No foreign key to runs yet: M2-05 adds it (see above).
    run_id                uuid,
    trigger_state         text,
    trigger_state_reason  text,
    trigger_state_at      timestamptz,
    silent                boolean     NOT NULL DEFAULT false,
    -- No foreign key to import_items yet: the import ticket of M8 adds it (see above).
    import_item_id        uuid,
    version               bigint      NOT NULL DEFAULT 1,
    edited_at             timestamptz,
    deleted_at            timestamptz,
    deleted_by            uuid,
    created_at            timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    FOREIGN KEY (thread_id) REFERENCES threads (id) ON DELETE RESTRICT,
    FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE RESTRICT,
    FOREIGN KEY (author_principal_id) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (deleted_by) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (reply_to_message_id) REFERENCES messages (id) ON DELETE SET NULL,
    CONSTRAINT messages_origin_check CHECK (origin IN ('human', 'agent', 'app', 'import', 'system')),
    CONSTRAINT messages_kind_check CHECK (kind IN ('text', 'voice', 'run', 'card', 'system')),
    CONSTRAINT messages_system_code_check CHECK (system_code IN ('thread.reopened')),
    CONSTRAINT messages_trigger_state_check CHECK (trigger_state IN
        ('received', 'queued', 'working', 'done', 'failed', 'cancelled', 'followup_delivered')),
    -- The anchor of a live box names its run, and only it does.
    CONSTRAINT messages_run_check CHECK ((kind = 'run') = (run_id IS NOT NULL)),
    -- Only a message of a person triggers an agent (09 section 3.2).
    CONSTRAINT messages_trigger_origin_check CHECK (trigger_state IS NULL OR origin = 'human'),
    CONSTRAINT messages_trigger_reason_check CHECK (trigger_state_reason IS NULL OR trigger_state IN ('failed', 'cancelled')),
    -- A deleted message is a tombstone without content (03 section 6.5).
    CONSTRAINT messages_deleted_content_check CHECK (deleted_at IS NULL OR content_md = ''),
    CONSTRAINT messages_system_code_kind_check CHECK ((kind = 'system') = (system_code IS NOT NULL)),
    -- A system line is silent, empty and written by the server.
    CONSTRAINT messages_system_check CHECK (kind <> 'system' OR (origin = 'system' AND silent AND content_md = ''))
);
-- The gapless thread sequence; also the history index (thread_id, thread_seq DESC).
CREATE UNIQUE INDEX messages_thread_seq_key ON messages (thread_id, thread_seq);
-- Idempotency of sending, for ever (04 section 5.1).
CREATE UNIQUE INDEX messages_author_client_msg_id_key ON messages (author_principal_id, client_msg_id);
-- One live box anchor per run.
CREATE UNIQUE INDEX messages_run_id_key ON messages (run_id) WHERE run_id IS NOT NULL;
-- Newest messages of a channel and the backfill context.
CREATE INDEX messages_channel_created_idx ON messages (channel_id, created_at DESC);
-- Export and "own messages".
CREATE INDEX messages_author_created_idx ON messages (author_principal_id, created_at DESC);
-- Resumption after a restart and "running now".
CREATE INDEX messages_trigger_state_idx ON messages (trigger_state) WHERE trigger_state IN ('received', 'queued', 'working');

CREATE TABLE mentions (
    message_id   uuid        NOT NULL,
    principal_id uuid        NOT NULL,
    thread_id    uuid        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, principal_id),
    FOREIGN KEY (message_id) REFERENCES messages (id) ON DELETE CASCADE,
    FOREIGN KEY (principal_id) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (thread_id) REFERENCES threads (id) ON DELETE CASCADE
);
-- "My mentions".
CREATE INDEX mentions_principal_created_idx ON mentions (principal_id, created_at DESC);
-- Unread mentions per thread.
CREATE INDEX mentions_principal_thread_idx ON mentions (principal_id, thread_id);

ALTER TABLE threads ADD CONSTRAINT threads_parent_message_id_fkey
    FOREIGN KEY (parent_message_id) REFERENCES messages (id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED NOT VALID;

GRANT SELECT, INSERT, UPDATE ON messages TO bothy_app;
GRANT SELECT, INSERT, DELETE ON mentions TO bothy_app;

-- +goose Down
ALTER TABLE threads DROP CONSTRAINT threads_parent_message_id_fkey;
DROP TABLE mentions;
DROP TABLE messages;
