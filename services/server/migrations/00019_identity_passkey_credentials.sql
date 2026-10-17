-- SPDX-License-Identifier: Apache-2.0
-- WebAuthn passkeys of the identity module (M1-40): passkey_credentials exactly as
-- docs/spec/03-data-model.md section 4.2 states it, for the login of local accounts and the
-- step-up (docs/spec/08-identity.md sections 2.5 and 6.2). Retention class: security (03
-- section 3). The table holds only public keys and counters, no secret.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update passkeys but
-- not delete them; a passkey ends by revoked_at, and the rows are security history, deleted only
-- by the retention job, which grants its delete (M7).

-- +goose Up
CREATE TABLE passkey_credentials (
    id              uuid        NOT NULL,
    user_id         uuid        NOT NULL,
    credential_id   bytea       NOT NULL,
    public_key_cose bytea       NOT NULL,
    sign_count      bigint      NOT NULL DEFAULT 0,
    aaguid          uuid,
    transports      text[]      NOT NULL DEFAULT '{}',
    backup_eligible boolean     NOT NULL DEFAULT false,
    backup_state    boolean     NOT NULL DEFAULT false,
    name            text        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    last_used_at    timestamptz,
    revoked_at      timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE RESTRICT,
    CONSTRAINT passkey_credentials_credential_id_key UNIQUE (credential_id)
);
-- The active passkeys of a person for the options of a ceremony.
CREATE INDEX passkey_credentials_user_id_active_idx ON passkey_credentials (user_id) WHERE revoked_at IS NULL;

GRANT SELECT, INSERT, UPDATE ON passkey_credentials TO bothy_app;

-- +goose Down
DROP TABLE passkey_credentials;
