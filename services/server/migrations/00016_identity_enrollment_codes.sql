-- SPDX-License-Identifier: Apache-2.0
-- Enrollment codes of the identity module (M1-37): enrollment_codes exactly as
-- docs/spec/03-data-model.md section 4.2 states it, plus the foreign key
-- auth_challenges.enrollment_code_id that M1-35 deferred. Retention class: security (03 section 3).
-- A code is stored only as HMAC-SHA-256 with the token pepper (03 Grundsatz 9); the code itself
-- is shown once and never stored.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update enrollment
-- codes but not delete them; a code ends by used_at, expires_at or its attempts, and the rows are
-- security history, deleted only by the retention job, which grants its delete (M7).
--
-- bothy setup bootstrap-owner runs as bothy_app (docs/spec/16-operations.md section 7.1) and locks
-- the instance row and sets setup_completed_at (08 section 4), so bothy_app gets the column right
-- UPDATE (setup_completed_at) on instance; every other column stays read only.

-- +goose Up
CREATE TABLE enrollment_codes (
    id                uuid        NOT NULL,
    user_id           uuid        NOT NULL,
    purpose           text        NOT NULL CHECK (purpose IN ('first_login', 'identity_link', 'recovery')),
    code_hash         bytea       NOT NULL,
    expires_at        timestamptz NOT NULL,
    attempts          integer     NOT NULL DEFAULT 0,
    created_by        uuid,
    created_at        timestamptz NOT NULL DEFAULT now(),
    used_at           timestamptz,
    used_by_device_id uuid,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (used_by_device_id) REFERENCES devices (id) ON DELETE SET NULL,
    CONSTRAINT enrollment_codes_code_hash_key UNIQUE (code_hash)
);
-- The unused codes of a person: issuing a code ends the others (08 section 3.2).
CREATE INDEX enrollment_codes_user_id_unused_idx ON enrollment_codes (user_id) WHERE used_at IS NULL;
-- The cleanup job finds expired codes.
CREATE INDEX enrollment_codes_expires_at_idx ON enrollment_codes (expires_at);

-- Deferred by M1-35: the checked code of a passkey enrollment.
ALTER TABLE auth_challenges ADD CONSTRAINT auth_challenges_enrollment_code_id_fkey
    FOREIGN KEY (enrollment_code_id) REFERENCES enrollment_codes (id) ON DELETE CASCADE;

GRANT SELECT, INSERT, UPDATE ON enrollment_codes TO bothy_app;
GRANT UPDATE (setup_completed_at) ON instance TO bothy_app;

-- +goose Down
REVOKE UPDATE (setup_completed_at) ON instance FROM bothy_app;
ALTER TABLE auth_challenges DROP CONSTRAINT auth_challenges_enrollment_code_id_fkey;
DROP TABLE enrollment_codes;
