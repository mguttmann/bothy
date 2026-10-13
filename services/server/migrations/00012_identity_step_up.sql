-- SPDX-License-Identifier: Apache-2.0
-- Single-use challenges of the identity module (M1-35): auth_challenges exactly as
-- docs/spec/03-datenmodell.md section 4.2 states it, for the step-up with the device key (this
-- ticket) and the WebAuthn ceremonies (M1-40), all purposes in the CHECK (R-DM-31). Retention
-- class: transient (03 section 3); the job identity.auth_cleanup deletes expired rows. The value
-- of a challenge is stored only as HMAC-SHA-256 with the token pepper (03 Grundsatz 9).
--
-- Deviation from 03: the foreign key auth_challenges.enrollment_code_id -> enrollment_codes(id)
-- ON DELETE CASCADE is not created here, because enrollment_codes arrives with M1-37; the column
-- and its pairing check exist, and the migration that creates enrollment_codes adds the
-- constraint (docs/DECISIONS.md M1-35).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert, update and delete; the
-- rows are transient and the cleanup job deletes them.

-- +goose Up
CREATE TABLE auth_challenges (
    id                 uuid        NOT NULL,
    purpose            text        NOT NULL
                                   CHECK (purpose IN ('step_up', 'passkey_login', 'passkey_enroll', 'passkey_register')),
    session_id         uuid,
    -- References enrollment_codes(id) ON DELETE CASCADE from M1-37 on.
    enrollment_code_id uuid,
    challenge_hash     bytea       NOT NULL,
    webauthn_session   jsonb,
    status             text        NOT NULL DEFAULT 'issued'
                                   CHECK (status IN ('issued', 'used', 'failed')),
    created_at         timestamptz NOT NULL DEFAULT now(),
    expires_at         timestamptz NOT NULL,
    finished_at        timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (session_id) REFERENCES auth_sessions (id) ON DELETE CASCADE,
    CONSTRAINT auth_challenges_challenge_hash_key UNIQUE (challenge_hash),
    -- A challenge is finished exactly when it is used or failed.
    CONSTRAINT auth_challenges_finished_check CHECK ((status = 'issued') = (finished_at IS NULL)),
    -- Step-up and passkey registration belong to a session, the other ceremonies to none.
    CONSTRAINT auth_challenges_session_check CHECK ((session_id IS NOT NULL) = (purpose IN ('step_up', 'passkey_register'))),
    -- Only the first setup with a passkey carries the checked enrollment code.
    CONSTRAINT auth_challenges_enrollment_check CHECK ((enrollment_code_id IS NOT NULL) = (purpose = 'passkey_enroll'))
);
-- Counting the failed step-ups of a session inside auth.login_fail_window (08 section 13).
CREATE INDEX auth_challenges_session_failed_idx ON auth_challenges (session_id, finished_at) WHERE status = 'failed';
-- The cleanup job finds expired challenges.
CREATE INDEX auth_challenges_expires_at_idx ON auth_challenges (expires_at);

GRANT SELECT, INSERT, UPDATE, DELETE ON auth_challenges TO bothy_app;

-- +goose Down
DROP TABLE auth_challenges;
