-- SPDX-License-Identifier: Apache-2.0
-- Queries on devices and device_keys (identity, docs/spec/03-datenmodell.md section 4.2). A device
-- row comes with the thumbprints of its active dpop and step_up keys (schema Device).

-- name: InsertDevice :one
INSERT INTO devices (id, user_id, name, platform, state, app_version, approved_by, approved_at)
VALUES (@id, @user_id, @name, @platform, @state, sqlc.narg(app_version), sqlc.narg(approved_by),
        CASE WHEN sqlc.narg(approved_by)::uuid IS NULL THEN NULL ELSE now() END)
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
