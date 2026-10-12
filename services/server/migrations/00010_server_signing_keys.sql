-- SPDX-License-Identifier: Apache-2.0
-- Server signing keys of the module instance (M1-32): server_signing_keys exactly as
-- docs/spec/03-datenmodell.md section 4.1 states it, retention class security. The table holds
-- the public JWK, the state and the reference of the private key; the private key itself lives
-- only in the key store secrets.signing_key_dir (ADR 0066, 03 principle 9). Enumerations are text
-- with CHECK (R-DM-02); kid is the RFC 7638 thumbprint of the public key.
--
-- Grants (migrations/README.md, grant rule): bothy_app reads, inserts and updates (state and
-- not_after) but never deletes: a retired row stays as the record of a key that once signed, and
-- audit_checkpoints and webhook_subscriptions will reference it (03, ON DELETE RESTRICT). The
-- retention job of class security grants its delete in its own migration.

-- +goose Up
CREATE TABLE server_signing_keys (
    kid             text        NOT NULL,
    purpose         text        NOT NULL
                                CHECK (purpose IN ('access_token', 'invocation', 'terminal', 'webhook_out', 'audit_checkpoint')),
    alg             text        NOT NULL CHECK (alg IN ('EdDSA', 'ES256')),
    public_jwk      jsonb       NOT NULL,
    private_key_ref text        NOT NULL,
    state           text        NOT NULL DEFAULT 'active'
                                CHECK (state IN ('next', 'active', 'retiring', 'retired')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    not_after       timestamptz,
    PRIMARY KEY (kid),
    -- Invocation and terminal tokens allow exactly one algorithm, and Standard Webhooks v1a knows
    -- only Ed25519 (03, E-EX-04).
    CONSTRAINT server_signing_keys_eddsa_check CHECK (purpose NOT IN ('invocation', 'terminal', 'webhook_out') OR alg = 'EdDSA')
);
-- Exactly one active key per purpose.
CREATE UNIQUE INDEX server_signing_keys_active_purpose_key ON server_signing_keys (purpose) WHERE state = 'active';
-- The JWKS output and the transitions read the keys of a purpose by state.
CREATE INDEX server_signing_keys_purpose_state_idx ON server_signing_keys (purpose, state);

GRANT SELECT, INSERT, UPDATE ON server_signing_keys TO bothy_app;

-- +goose Down
DROP TABLE server_signing_keys;
