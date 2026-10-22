-- SPDX-License-Identifier: Apache-2.0
-- Thumbnails of a file (M3-05): file_thumbnails exactly as docs/spec/03-data-model.md section 4.7
-- states it. The download of a thumbnail (downloadThumbnail) reads it by (file_id, variant); the
-- image job of M3-06 writes it. The table is new and empty, so its keys and checks are created with
-- it. Only the module files reads and writes it. Retention class: content (03).

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

GRANT SELECT, INSERT, UPDATE, DELETE ON file_thumbnails TO bothy_app;

-- +goose Down
DROP TABLE file_thumbnails;
