-- SPDX-License-Identifier: Apache-2.0
-- Identity providers and linked identities of the identity module (M1-36): identity_providers and
-- user_identities exactly as docs/spec/03-datenmodell.md section 4.2 states them, plus the two
-- foreign keys earlier migrations deferred because these tables did not exist:
-- auth_sessions.identity_id (M1-34) and groups.provider_id (M1-27). Retention classes: config for
-- identity_providers, account for user_identities (03 section 3). IDs are UUIDv7 from kernel/ids
-- (R-DM-01); enumerations are text with CHECK (R-DM-02). identity_providers.config holds secrets
-- only as references (client_secret_ref, client_key_ref; 03 Grundsatz 9).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert, update and delete
-- identity_providers (a provider without identities is deleted for good, 08 section 3.3) and may
-- read, insert and update user_identities (an identity ends by unlinked_at and stays history).

-- +goose Up
CREATE TABLE identity_providers (
    id                 uuid        NOT NULL,
    kind               text        NOT NULL CHECK (kind IN ('oidc', 'trusted_header', 'local')),
    name               text        NOT NULL,
    config             jsonb       NOT NULL,
    enabled            boolean     NOT NULL DEFAULT true,
    allow_provisioning boolean     NOT NULL DEFAULT false,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id)
);
-- Names are unique without regard to case (03 Grundsatz 6).
CREATE UNIQUE INDEX identity_providers_name_lower_key ON identity_providers (lower(name));
-- At most one trusted_header and one local driver per instance (08 section 2.1).
CREATE UNIQUE INDEX identity_providers_single_kind_key ON identity_providers (kind) WHERE kind IN ('trusted_header', 'local');

CREATE TABLE user_identities (
    id            uuid        NOT NULL,
    user_id       uuid        NOT NULL,
    provider_id   uuid        NOT NULL,
    subject       text        NOT NULL,
    issuer        text,
    email_at_link text,
    linked_at     timestamptz NOT NULL DEFAULT now(),
    last_login_at timestamptz,
    unlinked_at   timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE RESTRICT,
    -- Also an unlinked identity keeps its provider: disable instead of delete (08 section 3.3).
    FOREIGN KEY (provider_id) REFERENCES identity_providers (id) ON DELETE RESTRICT
);
-- One active link per subject of a provider; a subject linked before may be linked again.
CREATE UNIQUE INDEX user_identities_active_subject_key ON user_identities (provider_id, subject) WHERE unlinked_at IS NULL;
-- The credential overview of a person.
CREATE INDEX user_identities_user_id_idx ON user_identities (user_id);

-- Deferred by M1-34: the identity a session was started with.
ALTER TABLE auth_sessions ADD CONSTRAINT auth_sessions_identity_id_fkey
    FOREIGN KEY (identity_id) REFERENCES user_identities (id) ON DELETE SET NULL;
-- Deferred by M1-27: the provider whose group claim fills a group.
ALTER TABLE groups ADD CONSTRAINT groups_provider_id_fkey
    FOREIGN KEY (provider_id) REFERENCES identity_providers (id) ON DELETE RESTRICT;

GRANT SELECT, INSERT, UPDATE, DELETE ON identity_providers TO bothy_app;
GRANT SELECT, INSERT, UPDATE ON user_identities TO bothy_app;

-- +goose Down
ALTER TABLE groups DROP CONSTRAINT groups_provider_id_fkey;
ALTER TABLE auth_sessions DROP CONSTRAINT auth_sessions_identity_id_fkey;
DROP TABLE user_identities;
DROP TABLE identity_providers;
