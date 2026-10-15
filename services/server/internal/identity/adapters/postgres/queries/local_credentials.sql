-- SPDX-License-Identifier: Apache-2.0
-- Queries on local_credentials (identity, docs/spec/03-data-model.md section 4.2, M1-39). The
-- password is only an Argon2id PHC string and the TOTP secret only its AEAD ciphertext; no query
-- reads or writes either in plain form.

-- name: LockLocalCredential :one
-- The row of a login or a change, locked until the end of the unit of work, so parallel
-- attempts on one login name count their failures one after the other.
SELECT * FROM local_credentials WHERE identity_id = @identity_id FOR UPDATE;

-- name: GetLocalCredential :one
-- The row without a lock: the password login verifies the hash outside its unit of work, so the
-- row lock is never held during an Argon2id computation.
SELECT * FROM local_credentials WHERE identity_id = @identity_id;

-- name: SaveLocalCredential :exec
INSERT INTO local_credentials (identity_id, password_hash, password_changed_at, totp_secret_ciphertext, totp_key_ref,
    totp_last_step, totp_enabled_at, failed_count, fail_window_started_at, locked_until)
VALUES (@identity_id, @password_hash, @password_changed_at, sqlc.narg(totp_secret_ciphertext), sqlc.narg(totp_key_ref),
    sqlc.narg(totp_last_step), sqlc.narg(totp_enabled_at), @failed_count, sqlc.narg(fail_window_started_at), sqlc.narg(locked_until))
ON CONFLICT (identity_id) DO UPDATE SET
    password_hash = EXCLUDED.password_hash, password_changed_at = EXCLUDED.password_changed_at,
    totp_secret_ciphertext = EXCLUDED.totp_secret_ciphertext, totp_key_ref = EXCLUDED.totp_key_ref,
    totp_last_step = EXCLUDED.totp_last_step, totp_enabled_at = EXCLUDED.totp_enabled_at,
    failed_count = EXCLUDED.failed_count, fail_window_started_at = EXCLUDED.fail_window_started_at,
    locked_until = EXCLUDED.locked_until;

-- name: LocalCredentialOfUser :one
-- The row of the active local identity of a person (credential overview).
SELECT c.* FROM local_credentials c
JOIN user_identities i ON i.id = c.identity_id
JOIN identity_providers p ON p.id = i.provider_id
WHERE i.user_id = @user_id AND i.unlinked_at IS NULL AND p.kind = 'local'
ORDER BY i.linked_at DESC, i.id
LIMIT 1;

-- name: HasEstablishedLocalCredential :one
-- The password path of a person is established once its TOTP enrollment is confirmed (07 section
-- 4.2 step 8, has_credentials of local_credential.set).
SELECT EXISTS (
    SELECT 1 FROM local_credentials c
    JOIN user_identities i ON i.id = c.identity_id
    WHERE i.user_id = @user_id AND i.unlinked_at IS NULL AND c.totp_enabled_at IS NOT NULL
)::boolean AS established;
