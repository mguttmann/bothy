-- SPDX-License-Identifier: Apache-2.0
-- Upload sessions (M3-03): files, file_upload_sessions and file_upload_parts exactly as
-- docs/spec/03-data-model.md section 4.7 states them, with files.search_vector of section 5. The
-- tables are new and empty, so their keys, checks and indexes are created with them. Only the
-- module files reads and writes them. Retention classes: content for files, transient for the
-- sessions and their parts (the jobs follow with M7-05). The range of part_number (1 to
-- part_count) is a rule of the application, not a check (03 names none).
--
-- The deferred foreign key of principals.avatar_file_id (M1-26) is added NOT VALID here and
-- validated by the next migration (docs/operations/upgrade.md rule M5).

-- +goose Up
CREATE TABLE files (
    id                  uuid        NOT NULL,
    object_key          text        NOT NULL,
    sha256              bytea       NOT NULL,
    size_bytes          bigint      NOT NULL,
    mime_type           text        NOT NULL,
    declared_mime_type  text,
    original_name       text        NOT NULL,
    uploaded_by         uuid        NOT NULL,
    purpose             text        NOT NULL DEFAULT 'attachment'
                                    CHECK (purpose IN ('attachment', 'avatar', 'branding', 'gdpr_export')),
    scope_channel_id    uuid,
    upload_state        text        NOT NULL DEFAULT 'pending'
                                    CHECK (upload_state IN ('pending', 'complete', 'failed')),
    scan_status         text        NOT NULL DEFAULT 'pending'
                                    CHECK (scan_status IN ('pending', 'clean', 'infected', 'error', 'skipped')),
    scan_engine         text,
    scanned_at          timestamptz,
    width               integer,
    height              integer,
    duration_ms         bigint,
    transcript_status   text        NOT NULL DEFAULT 'none'
                                    CHECK (transcript_status IN ('none', 'pending', 'done', 'failed')),
    transcript_text     text,
    transcript_language text,
    transcript_source   text        CHECK (transcript_source IN ('agent', 'service')),
    created_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz,
    search_vector       tsvector    NOT NULL GENERATED ALWAYS AS (
        setweight(to_tsvector('simple'::regconfig, original_name), 'A') ||
        setweight(to_tsvector('simple'::regconfig, coalesce(transcript_text, '')), 'B')
    ) STORED,
    PRIMARY KEY (id),
    FOREIGN KEY (uploaded_by) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (scope_channel_id) REFERENCES channels (id) ON DELETE RESTRICT,
    CHECK ((purpose = 'attachment') = (scope_channel_id IS NOT NULL)),
    UNIQUE (object_key)
);
CREATE INDEX files_uploaded_by_created_at_idx ON files (uploaded_by, created_at DESC);
CREATE INDEX files_scan_status_pending_idx ON files (scan_status) WHERE scan_status = 'pending';
CREATE INDEX files_upload_state_pending_idx ON files (upload_state, created_at) WHERE upload_state = 'pending';
CREATE INDEX files_search_vector_idx ON files USING gin (search_vector);

CREATE TABLE file_upload_sessions (
    file_id             uuid        NOT NULL,
    mode                text        NOT NULL DEFAULT 'direct' CHECK (mode IN ('direct', 'presigned')),
    state               text        NOT NULL DEFAULT 'open'
                                    CHECK (state IN ('open', 'completed', 'aborted', 'expired')),
    declared_size_bytes bigint      NOT NULL,
    declared_sha256     bytea       NOT NULL,
    part_size_bytes     bigint      NOT NULL,
    part_count          integer     NOT NULL,
    storage_upload_id   text,
    failure_reason      text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    completed_at        timestamptz,
    expires_at          timestamptz NOT NULL,
    PRIMARY KEY (file_id),
    FOREIGN KEY (file_id) REFERENCES files (id) ON DELETE CASCADE
);
CREATE INDEX file_upload_sessions_open_expires_at_idx ON file_upload_sessions (expires_at) WHERE state = 'open';

CREATE TABLE file_upload_parts (
    file_id     uuid        NOT NULL,
    part_number integer     NOT NULL,
    size_bytes  bigint      NOT NULL,
    sha256      bytea       NOT NULL,
    etag        text,
    received_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (file_id, part_number),
    FOREIGN KEY (file_id) REFERENCES file_upload_sessions (file_id) ON DELETE CASCADE
);

ALTER TABLE principals ADD CONSTRAINT principals_avatar_file_id_fkey
    FOREIGN KEY (avatar_file_id) REFERENCES files (id) ON DELETE SET NULL NOT VALID;

GRANT SELECT, INSERT, UPDATE, DELETE ON files, file_upload_sessions, file_upload_parts TO bothy_app;

-- +goose Down
ALTER TABLE principals DROP CONSTRAINT principals_avatar_file_id_fkey;
DROP TABLE file_upload_parts;
DROP TABLE file_upload_sessions;
DROP TABLE files;
