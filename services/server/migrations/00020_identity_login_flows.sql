-- SPDX-License-Identifier: Apache-2.0
-- OIDC login flows of the identity module (M1-41): auth_login_flows exactly as
-- docs/spec/03-datenmodell.md section 4.2 states it. Retention class: transient (03 section 3);
-- the job identity.auth_cleanup deletes the rows past expires_at.
-- state, nonce and login_code are stored only as HMAC-SHA-256 with the token pepper (03 Grundsatz
-- 9); the server PKCE verifier is derived from id and the pepper and not stored. The callback
-- stores only the verified external identity (R-DM-30); mapping, linking and provisioning happen
-- at complete.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert, update and delete flows;
-- the delete is the cleanup job's.

-- +goose Up
CREATE TABLE auth_login_flows (
    id                    uuid        NOT NULL,
    provider_id           uuid        NOT NULL,
    mode                  text        NOT NULL CHECK (mode IN ('login', 'link')),
    user_id               uuid,
    device_id             uuid,
    oidc_state_hash       bytea       NOT NULL,
    oidc_nonce_hash       bytea       NOT NULL,
    client_challenge      text        NOT NULL,
    app_redirect_uri      text        NOT NULL,
    login_code_hash       bytea,
    login_code_expires_at timestamptz,
    enrollment_code_id    uuid,
    external_identity     jsonb,
    status                text        NOT NULL DEFAULT 'started'
                                      CHECK (status IN ('started', 'code_issued', 'completed', 'failed')),
    error                 text,
    created_at            timestamptz NOT NULL DEFAULT now(),
    expires_at            timestamptz NOT NULL,
    PRIMARY KEY (id),
    FOREIGN KEY (provider_id) REFERENCES identity_providers (id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE CASCADE,
    FOREIGN KEY (enrollment_code_id) REFERENCES enrollment_codes (id) ON DELETE SET NULL,
    CONSTRAINT auth_login_flows_oidc_state_hash_key UNIQUE (oidc_state_hash),
    -- The verified identity lives in the flow exactly while the login_code waits (R-DM-30).
    CHECK ((status = 'code_issued') = (external_identity IS NOT NULL)),
    -- A link flow links to its own person, never through an enrollment code.
    CHECK (mode <> 'link' OR enrollment_code_id IS NULL)
);
-- complete finds a flow by the HMAC of its login_code.
CREATE UNIQUE INDEX auth_login_flows_login_code_hash_key ON auth_login_flows (login_code_hash) WHERE login_code_hash IS NOT NULL;
-- The cleanup job finds expired flows.
CREATE INDEX auth_login_flows_expires_at_idx ON auth_login_flows (expires_at);

GRANT SELECT, INSERT, UPDATE, DELETE ON auth_login_flows TO bothy_app;

-- +goose Down
DROP TABLE auth_login_flows;
