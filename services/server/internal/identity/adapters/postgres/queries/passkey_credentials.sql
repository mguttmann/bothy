-- SPDX-License-Identifier: Apache-2.0
-- Queries on passkey_credentials (identity, docs/spec/03-datenmodell.md section 4.2,
-- docs/spec/08-identitaet.md section 2.5, M1-40). A passkey ends by revoked_at; no row is
-- deleted here.

-- name: InsertPasskey :one
INSERT INTO passkey_credentials (id, user_id, credential_id, public_key_cose, sign_count, aaguid, transports,
                                 backup_eligible, backup_state, name, created_at)
VALUES (@id, @user_id, @credential_id, @public_key_cose, @sign_count, sqlc.narg(aaguid), @transports,
        @backup_eligible, @backup_state, @name, @created_at)
RETURNING *;

-- name: ActivePasskeysOfUser :many
-- The active passkeys of a person, oldest first, over passkey_credentials_user_id_active_idx.
SELECT * FROM passkey_credentials WHERE user_id = @user_id AND revoked_at IS NULL ORDER BY created_at, id;

-- name: PasskeyByCredentialID :one
SELECT * FROM passkey_credentials WHERE credential_id = @credential_id;

-- name: LockPasskey :one
SELECT * FROM passkey_credentials WHERE id = @id FOR UPDATE;

-- name: UpdatePasskeyUse :exec
-- A successful assertion: the kept sign counter, the backup state and the time of use.
UPDATE passkey_credentials SET sign_count = @sign_count, backup_state = @backup_state, last_used_at = @last_used_at
WHERE id = @id;

-- name: RevokePasskey :execrows
UPDATE passkey_credentials SET revoked_at = @revoked_at WHERE id = @id AND revoked_at IS NULL;

-- name: CountActivePasskeys :one
SELECT count(*) FROM passkey_credentials WHERE user_id = @user_id AND revoked_at IS NULL;
