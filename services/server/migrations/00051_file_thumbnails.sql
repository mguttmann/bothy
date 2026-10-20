-- SPDX-License-Identifier: Apache-2.0
-- Thumbnails of images (M3-06): file_thumbnails exactly as docs/spec/03-datenmodell.md defines it.
-- Only the module files reads and writes it. Retention class: content, like files. A variant row
-- is inserted once and never updated, so the object of a variant is never written again
-- (docs/spec/12-dateien.md 4.4); bothy_app therefore has no UPDATE on it. The rows go with their
-- file (ON DELETE CASCADE).

-- +goose Up
CREATE TABLE file_thumbnails (
    file_id    uuid    NOT NULL,
    variant    text    NOT NULL CHECK (variant IN ('small', 'medium', 'large', 'poster')),
    object_key text    NOT NULL,
    mime_type  text    NOT NULL,
    width      integer NOT NULL,
    height     integer NOT NULL,
    size_bytes bigint  NOT NULL,
    PRIMARY KEY (file_id, variant),
    FOREIGN KEY (file_id) REFERENCES files (id) ON DELETE CASCADE,
    UNIQUE (object_key)
);

GRANT SELECT, INSERT, DELETE ON file_thumbnails TO bothy_app;

-- +goose Down
DROP TABLE file_thumbnails;
