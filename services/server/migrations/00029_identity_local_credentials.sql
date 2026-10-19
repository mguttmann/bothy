-- SPDX-License-Identifier: Apache-2.0
-- Local credentials of the identity module (M1-39): local_credentials exactly as
-- docs/spec/03-data-model.md section 4.2 states it, with the failure window column
-- fail_window_started_at and its check. Retention class: security (03 section 3).
-- The password is stored only as Argon2id in PHC format; the TOTP secret only AEAD-encrypted, its
-- key lives in the secret provider and the row keeps the reference (03 principle 9, 19).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update local
-- credentials but not delete them; a row goes with its identity (ON DELETE CASCADE) or with the
-- deletion of the account.

-- +goose Up
CREATE TABLE local_credentials (
    identity_id            uuid        NOT NULL,
    password_hash          text        NOT NULL,
    password_changed_at    timestamptz NOT NULL DEFAULT now(),
    totp_secret_ciphertext bytea,
    totp_key_ref           text,
    totp_last_step         bigint,
    totp_enabled_at        timestamptz,
    failed_count           integer     NOT NULL DEFAULT 0,
    fail_window_started_at timestamptz,
    locked_until           timestamptz,
    PRIMARY KEY (identity_id),
    FOREIGN KEY (identity_id) REFERENCES user_identities (id) ON DELETE CASCADE,
    CONSTRAINT local_credentials_totp_key_check CHECK ((totp_secret_ciphertext IS NULL) = (totp_key_ref IS NULL)),
    CONSTRAINT local_credentials_fail_window_check CHECK ((failed_count = 0) = (fail_window_started_at IS NULL))
);

GRANT SELECT, INSERT, UPDATE ON local_credentials TO bothy_app;

-- +goose Down
DROP TABLE local_credentials;
