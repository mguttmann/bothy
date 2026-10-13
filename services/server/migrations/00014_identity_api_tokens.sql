-- SPDX-License-Identifier: Apache-2.0
-- Long-lived API tokens of the identity module (M1-44): api_tokens exactly as
-- docs/spec/03-datenmodell.md section 4.2 states it. Retention class: security (03 section 3).
-- A token is stored only as HMAC-SHA-256 with the server pepper (token_hash) plus token_prefix,
-- never in the clear (03 Grundsatz 9, ADR 0061); expires_at is required (08 section 12).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update, not delete.
-- A token ends by revoked_at or expires_at and the rows are security history; a retention job
-- grants its delete in its own migration.

-- +goose Up
CREATE TABLE api_tokens (
    id           uuid        NOT NULL,
    principal_id uuid        NOT NULL,
    name         text        NOT NULL,
    token_prefix text        NOT NULL,
    token_hash   bytea       NOT NULL,
    scopes       text[]      NOT NULL DEFAULT '{}',
    dpop_jkt     text,
    expires_at   timestamptz NOT NULL,
    created_by   uuid        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    revoked_at   timestamptz,
    revoked_by   uuid,
    PRIMARY KEY (id),
    FOREIGN KEY (principal_id) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (revoked_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT api_tokens_token_hash_key UNIQUE (token_hash)
);
-- The token list of a principal (listTokens) and RevokeAllForPrincipal read the active tokens.
CREATE INDEX api_tokens_principal_active_idx ON api_tokens (principal_id) WHERE revoked_at IS NULL;

GRANT SELECT, INSERT, UPDATE ON api_tokens TO bothy_app;

-- +goose Down
DROP TABLE api_tokens;
