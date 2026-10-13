-- SPDX-License-Identifier: Apache-2.0
-- Sessions, refresh families and refresh tokens of the identity module (M1-34): auth_sessions,
-- refresh_token_families and refresh_tokens exactly as docs/spec/03-data-model.md section 4.2
-- states them. Retention class of all three: security (03 section 3). IDs are UUIDv7 from
-- kernel/ids (R-DM-01); enumerations are text with CHECK (R-DM-02). Refresh tokens and console
-- cookies are stored only as HMAC-SHA-256 with the server pepper (03 Grundsatz 9).
--
-- Deviation from 03: the foreign key auth_sessions.identity_id -> user_identities(id) ON DELETE
-- SET NULL is not created here, because user_identities arrives with M1-36; the column exists and
-- the migration of M1-36 adds the constraint (docs/DECISIONS.md M1-34).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update the three
-- tables but not delete. Sessions and families end by revoked_at, tokens by used_at, and the rows
-- are security history; the cleanup and retention jobs grant their delete in their own migration.

-- +goose Up
CREATE TABLE auth_sessions (
    id                  uuid        NOT NULL,
    user_id             uuid        NOT NULL,
    device_id           uuid        NOT NULL,
    client_component    text        NOT NULL DEFAULT 'app'
                                    CHECK (client_component IN ('app', 'share_ext', 'console')),
    parent_session_id   uuid,
    -- References user_identities(id) ON DELETE SET NULL from M1-36 on.
    identity_id         uuid,
    auth_method         text        NOT NULL
                                    CHECK (auth_method IN ('oidc', 'trusted_header', 'passkey', 'password_totp',
                                                           'enrollment_code', 'device_pairing')),
    cookie_hash         bytea,
    created_at          timestamptz NOT NULL DEFAULT now(),
    expires_at          timestamptz NOT NULL,
    step_up_at          timestamptz,
    step_up_method      text        CHECK (step_up_method IN ('passkey', 'device_key', 'password_totp', 'oidc_reauth')),
    step_up_consumed_at timestamptz,
    last_seen_at        timestamptz,
    last_ip             inet,
    revoked_at          timestamptz,
    revoke_reason       text        CHECK (revoke_reason IN ('logout', 'admin', 'device_revoked', 'refresh_reuse',
                                                             'user_disabled', 'expired', 'inactive', 'step_up_failed',
                                                             'trusted_header_mismatch')),
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (device_id) REFERENCES devices (id) ON DELETE RESTRICT,
    FOREIGN KEY (parent_session_id) REFERENCES auth_sessions (id) ON DELETE CASCADE,
    -- Only a console session has a cookie, and every console session has one.
    CONSTRAINT auth_sessions_cookie_hash_check CHECK ((client_component = 'console') = (cookie_hash IS NOT NULL)),
    -- Only a share extension session is a child.
    CONSTRAINT auth_sessions_parent_check CHECK (parent_session_id IS NULL OR client_component = 'share_ext'),
    -- A step-up has both its time and its method.
    CONSTRAINT auth_sessions_step_up_check CHECK ((step_up_at IS NULL) = (step_up_method IS NULL)),
    -- Only a step-up that happened can be consumed.
    CONSTRAINT auth_sessions_step_up_consumed_check CHECK (step_up_consumed_at IS NULL OR step_up_at IS NOT NULL)
);
-- The console looks a session up by the HMAC of its cookie.
CREATE UNIQUE INDEX auth_sessions_cookie_hash_key ON auth_sessions (cookie_hash) WHERE cookie_hash IS NOT NULL;
-- "End every session" of a person and the session lists.
CREATE INDEX auth_sessions_user_id_active_idx ON auth_sessions (user_id) WHERE revoked_at IS NULL;
-- Revoking a device ends its sessions.
CREATE INDEX auth_sessions_device_id_idx ON auth_sessions (device_id);
-- Revoking a parent ends its children.
CREATE INDEX auth_sessions_parent_session_id_idx ON auth_sessions (parent_session_id) WHERE parent_session_id IS NOT NULL;
-- The cleanup job finds expired sessions.
CREATE INDEX auth_sessions_expires_at_idx ON auth_sessions (expires_at);

CREATE TABLE refresh_token_families (
    id                 uuid        NOT NULL,
    session_id         uuid        NOT NULL,
    dpop_jkt           text        NOT NULL,
    current_generation integer     NOT NULL DEFAULT 0,
    rotated_at         timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    revoked_at         timestamptz,
    revoke_reason      text        CHECK (revoke_reason IN ('reuse_detected', 'logout', 'session_revoked', 'expired')),
    PRIMARY KEY (id),
    FOREIGN KEY (session_id) REFERENCES auth_sessions (id) ON DELETE CASCADE
);
-- Exactly one active family per session (08 section 7.4).
CREATE UNIQUE INDEX refresh_token_families_active_session_key ON refresh_token_families (session_id) WHERE revoked_at IS NULL;

CREATE TABLE refresh_tokens (
    id         uuid        NOT NULL,
    family_id  uuid        NOT NULL,
    generation integer     NOT NULL,
    token_hash bytea       NOT NULL,
    issued_at  timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (family_id) REFERENCES refresh_token_families (id) ON DELETE CASCADE,
    CONSTRAINT refresh_tokens_token_hash_key UNIQUE (token_hash),
    CONSTRAINT refresh_tokens_family_id_generation_key UNIQUE (family_id, generation)
);

GRANT SELECT, INSERT, UPDATE ON auth_sessions, refresh_token_families, refresh_tokens TO bothy_app;

-- +goose Down
DROP TABLE refresh_tokens;
DROP TABLE refresh_token_families;
DROP TABLE auth_sessions;
