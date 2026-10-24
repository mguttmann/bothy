-- SPDX-License-Identifier: Apache-2.0
-- Text extracted from files for search (M3-15): file_text_extracts exactly as
-- docs/spec/03-data-model.md section 4.7 states it, with the generated search_vector of section 5
-- (stems of the instance language weight C, words weight D). The job files.extract_text writes it;
-- SearchFiles reads its vector next to the name and the transcript of the file. The table is new
-- and empty, so its indexes are created with it. Only the module files reads and writes it.
-- Retention class: content (03), deleted with the file.

-- +goose Up
CREATE TABLE file_text_extracts (
    file_id         uuid        NOT NULL,
    state           text        NOT NULL DEFAULT 'pending'
        CHECK (state IN ('pending', 'done', 'failed', 'unsupported')),
    extractor       text,
    text_content    text,
    truncated       boolean     NOT NULL DEFAULT false,
    search_language regconfig   NOT NULL DEFAULT 'simple',
    extracted_at    timestamptz,
    error           text,
    search_vector   tsvector    NOT NULL GENERATED ALWAYS AS (
        setweight(to_tsvector(search_language, coalesce(text_content, '')), 'C') ||
        setweight(to_tsvector('simple'::regconfig, coalesce(text_content, '')), 'D')
    ) STORED,
    PRIMARY KEY (file_id),
    FOREIGN KEY (file_id) REFERENCES files (id) ON DELETE CASCADE
);

CREATE INDEX file_text_extracts_search_vector_idx ON file_text_extracts USING gin (search_vector);
CREATE INDEX file_text_extracts_pending_idx ON file_text_extracts (state) WHERE state = 'pending';

GRANT SELECT, INSERT, UPDATE, DELETE ON file_text_extracts TO bothy_app;

-- +goose Down
DROP TABLE file_text_extracts;
