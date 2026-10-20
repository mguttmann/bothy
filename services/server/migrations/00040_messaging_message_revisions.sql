-- SPDX-License-Identifier: Apache-2.0
-- Message revisions (M1-58): message_revisions exactly as docs/spec/03-data-model.md states it.
-- An edit inserts the previous content as the next revision (revision 1 is the original); a
-- deletion removes every revision of the message in the same transaction (03 section 6.5). Run
-- anchors (kind = 'run') store no revisions. Retention class: content.
--
-- Grants (migrations/README.md, grant rule): bothy_app reads, inserts and deletes rows; a revision
-- is never changed.

-- +goose Up
CREATE TABLE message_revisions (
    id         uuid        NOT NULL,
    message_id uuid        NOT NULL,
    revision   integer     NOT NULL,
    content_md text        NOT NULL,
    edited_by  uuid        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    UNIQUE (message_id, revision),
    FOREIGN KEY (message_id) REFERENCES messages (id) ON DELETE CASCADE,
    FOREIGN KEY (edited_by) REFERENCES principals (id) ON DELETE RESTRICT
);

GRANT SELECT, INSERT, DELETE ON message_revisions TO bothy_app;

-- +goose Down
DROP TABLE message_revisions;
