-- SPDX-License-Identifier: Apache-2.0
-- Queries of the revocation use cases (identity, M1-45; docs/spec/08-identitaet.md sections 10 and
-- 11). Rows of one table are locked in ascending id (docs/spec/03-datenmodell.md section 6.3):
-- devices first, then sessions, so a session that a concurrent login inserts on a device is
-- either committed before the device lock is granted, and then read, or waits for it.

-- name: DisablePrincipal :exec
UPDATE principals SET status = 'disabled', disabled_at = @disabled_at WHERE id = @id;

-- name: LockLiveDevicesOfUser :many
-- Every device of the person that is not revoked (pending included), locked in ascending id.
SELECT sqlc.embed(d),
       kd.jkt AS key_thumbprint,
       ks.jkt AS step_up_key_thumbprint
FROM devices d
LEFT JOIN device_keys kd ON kd.device_id = d.id AND kd.purpose = 'dpop' AND kd.revoked_at IS NULL
LEFT JOIN device_keys ks ON ks.device_id = d.id AND ks.purpose = 'step_up' AND ks.revoked_at IS NULL
WHERE d.user_id = @user_id AND d.state <> 'revoked'
ORDER BY d.id
FOR UPDATE OF d;

-- name: LockLiveSessionsOfUser :many
-- Every session of the person with revoked_at IS NULL (children included), in ascending id.
SELECT * FROM auth_sessions
WHERE user_id = @user_id AND revoked_at IS NULL
ORDER BY id
FOR UPDATE;

-- name: LockLiveSessionsOfDevice :many
-- Every session of the device with revoked_at IS NULL (children share the device), ascending id.
SELECT * FROM auth_sessions
WHERE device_id = @device_id AND revoked_at IS NULL
ORDER BY id
FOR UPDATE;

-- name: RevokeDevices :many
-- Revokes the devices among ids that are not revoked yet and returns their ids; revoked_by is
-- NULL for the system action device.expire.
UPDATE devices SET state = 'revoked', revoked_at = @revoked_at, revoked_by = sqlc.narg(revoked_by),
                   revoke_reason = @revoke_reason
WHERE id = ANY(@ids::uuid[]) AND state <> 'revoked'
RETURNING id;

-- name: RevokeKeysOfDevices :execrows
UPDATE device_keys SET revoked_at = @revoked_at
WHERE device_id = ANY(@device_ids::uuid[]) AND revoked_at IS NULL;

-- name: ListInactiveDevices :many
-- Active devices without contact since before: last_seen_at, or approved_at for a device never
-- seen (08 section 5.4), in ascending id after after_id.
SELECT id, user_id FROM devices
WHERE state = 'active' AND coalesce(last_seen_at, approved_at, created_at) < @before
  AND (sqlc.narg(after_id)::uuid IS NULL OR id > sqlc.narg(after_id)::uuid)
ORDER BY id
LIMIT @row_limit;
