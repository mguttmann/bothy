-- SPDX-License-Identifier: Apache-2.0
-- Queries on devices and device_keys (identity, docs/spec/03-data-model.md section 4.2). A device
-- row comes with the thumbprints of its active dpop and step_up keys (schema Device).

-- name: InsertDevice :one
-- created_at is the time of the login request when given (the pending window counts from it,
-- M1-38), else the database time; an active device is approved at its creation.
INSERT INTO devices (id, user_id, name, platform, state, app_version, approved_by, approved_at, created_at)
VALUES (@id, @user_id, @name, @platform, @state, sqlc.narg(app_version), sqlc.narg(approved_by),
        CASE WHEN sqlc.narg(approved_by)::uuid IS NULL THEN NULL ELSE coalesce(sqlc.narg(created_at)::timestamptz, now()) END,
        coalesce(sqlc.narg(created_at)::timestamptz, now()))
RETURNING created_at, approved_at;

-- name: InsertDeviceKey :exec
INSERT INTO device_keys (id, device_id, purpose, jkt, public_jwk, alg, attestation)
VALUES (@id, @device_id, @purpose, @jkt, @public_jwk, @alg, sqlc.narg(attestation));

-- name: GetDevice :one
SELECT sqlc.embed(d),
       kd.jkt AS key_thumbprint,
       ks.jkt AS step_up_key_thumbprint
FROM devices d
LEFT JOIN device_keys kd ON kd.device_id = d.id AND kd.purpose = 'dpop' AND kd.revoked_at IS NULL
LEFT JOIN device_keys ks ON ks.device_id = d.id AND ks.purpose = 'step_up' AND ks.revoked_at IS NULL
WHERE d.id = @id;

-- name: ListDevicesOfUser :many
-- Keyset on id DESC (UUIDv7, newest first); before is the id of the last row of the previous page.
SELECT sqlc.embed(d),
       kd.jkt AS key_thumbprint,
       ks.jkt AS step_up_key_thumbprint
FROM devices d
LEFT JOIN device_keys kd ON kd.device_id = d.id AND kd.purpose = 'dpop' AND kd.revoked_at IS NULL
LEFT JOIN device_keys ks ON ks.device_id = d.id AND ks.purpose = 'step_up' AND ks.revoked_at IS NULL
WHERE d.user_id = @user_id
  AND (sqlc.narg(before)::uuid IS NULL OR d.id < sqlc.narg(before)::uuid)
ORDER BY d.id DESC
LIMIT @row_limit;

-- name: GetActiveDeviceKey :one
SELECT * FROM device_keys
WHERE device_id = @device_id AND purpose = @purpose AND revoked_at IS NULL;

-- name: GetDeviceKeyByJKT :one
SELECT * FROM device_keys WHERE jkt = @jkt;

-- name: LockDevice :one
-- The device row of an approval, expiry (M1-38) or revocation (M1-45), locked until the end of the
-- unit of work.
SELECT sqlc.embed(d),
       kd.jkt AS key_thumbprint,
       ks.jkt AS step_up_key_thumbprint
FROM devices d
LEFT JOIN device_keys kd ON kd.device_id = d.id AND kd.purpose = 'dpop' AND kd.revoked_at IS NULL
LEFT JOIN device_keys ks ON ks.device_id = d.id AND ks.purpose = 'step_up' AND ks.revoked_at IS NULL
WHERE d.id = @id
FOR UPDATE OF d;

-- name: ActivateDevice :execrows
UPDATE devices SET state = 'active', approved_by = @approved_by, approved_at = @approved_at
WHERE id = @id AND state = 'pending';

-- name: RevokeDevice :one
UPDATE devices SET state = 'revoked', revoked_at = @revoked_at, revoked_by = sqlc.narg(revoked_by), revoke_reason = @revoke_reason
WHERE id = @id AND state <> 'revoked'
RETURNING user_id;

-- name: RevokeDeviceKeys :exec
UPDATE device_keys SET revoked_at = @revoked_at WHERE device_id = @device_id AND revoked_at IS NULL;

-- name: ListPendingDevicesCreatedBefore :many
-- The expiry of the job identity.auth_cleanup: pending devices past auth.device_pending_ttl.
SELECT sqlc.embed(d),
       kd.jkt AS key_thumbprint,
       ks.jkt AS step_up_key_thumbprint
FROM devices d
LEFT JOIN device_keys kd ON kd.device_id = d.id AND kd.purpose = 'dpop' AND kd.revoked_at IS NULL
LEFT JOIN device_keys ks ON ks.device_id = d.id AND ks.purpose = 'step_up' AND ks.revoked_at IS NULL
WHERE d.state = 'pending' AND d.created_at <= @created_before
ORDER BY d.created_at, d.id
LIMIT @row_limit;
