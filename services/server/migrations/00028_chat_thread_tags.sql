-- SPDX-License-Identifier: Apache-2.0
-- Forum tags of threads (M1-55): thread_tags exactly as docs/spec/03-datenmodell.md section 4.5
-- states it. That a tag belongs to the channel of its thread and that a thread holds at most
-- thread.max_tags tags is checked by the application (03 thread_tags, "Anwendung").
-- Retention class: content.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and delete rows, because
-- setThreadTags replaces the set of a thread and deleteForumTag drops the rows of its tag in the
-- same transaction (brief M1-55 D5); a row is never updated.

-- +goose Up
CREATE TABLE thread_tags (
    thread_id uuid        NOT NULL,
    tag_id    uuid        NOT NULL,
    added_by  uuid        NOT NULL,
    added_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (thread_id, tag_id),
    FOREIGN KEY (thread_id) REFERENCES threads (id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES forum_tags (id) ON DELETE CASCADE,
    FOREIGN KEY (added_by) REFERENCES principals (id) ON DELETE RESTRICT
);
-- The tag filter of the post list.
CREATE INDEX thread_tags_tag_id_idx ON thread_tags (tag_id);

GRANT SELECT, INSERT, DELETE ON thread_tags TO bothy_app;

-- +goose Down
DROP TABLE thread_tags;
