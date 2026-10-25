-- SPDX-License-Identifier: Apache-2.0
-- The participant of class security (M7-05, docs/spec/20-privacy.md 4.1): per table the rows whose
-- end time lies before the cutoff (retention.security_after_end before now), oldest first, and the
-- delete of one of them. A row without an end time (an active device, a valid key) never matches.

-- name: ExpiredRefreshTokens :many
SELECT id FROM refresh_tokens WHERE expires_at < @cutoff ORDER BY expires_at, id LIMIT @row_limit;

-- name: DeleteRefreshToken :execrows
DELETE FROM refresh_tokens WHERE id = @id AND expires_at < @cutoff;

-- name: ExpiredRefreshFamilies :many
SELECT id FROM refresh_token_families WHERE revoked_at < @cutoff ORDER BY revoked_at, id LIMIT @row_limit;

-- name: DeleteRefreshFamily :execrows
DELETE FROM refresh_token_families WHERE id = @id AND revoked_at < @cutoff;

-- name: ExpiredAuthSessions :many
-- The end of a session is its revocation or its expiry, whichever came first.
SELECT id FROM auth_sessions WHERE LEAST(expires_at, revoked_at) < @cutoff
ORDER BY LEAST(expires_at, revoked_at), id LIMIT @row_limit;

-- name: DeleteAuthSession :execrows
DELETE FROM auth_sessions WHERE id = @id AND LEAST(expires_at, revoked_at) < @cutoff;

-- name: ExpiredDeviceKeys :many
SELECT id FROM device_keys WHERE revoked_at < @cutoff ORDER BY revoked_at, id LIMIT @row_limit;

-- name: DeleteDeviceKey :execrows
DELETE FROM device_keys WHERE id = @id AND revoked_at < @cutoff;

-- name: ExpiredDevices :many
SELECT id FROM devices WHERE revoked_at < @cutoff ORDER BY revoked_at, id LIMIT @row_limit;

-- name: DeleteDevice :execrows
DELETE FROM devices WHERE id = @id AND revoked_at < @cutoff;

-- name: ExpiredPasskeys :many
SELECT id FROM passkey_credentials WHERE revoked_at < @cutoff ORDER BY revoked_at, id LIMIT @row_limit;

-- name: DeletePasskey :execrows
DELETE FROM passkey_credentials WHERE id = @id AND revoked_at < @cutoff;

-- name: ExpiredAPITokens :many
-- The end of a token is its revocation or its expiry, whichever came first.
SELECT id FROM api_tokens WHERE LEAST(expires_at, revoked_at) < @cutoff
ORDER BY LEAST(expires_at, revoked_at), id LIMIT @row_limit;

-- name: DeleteAPIToken :execrows
DELETE FROM api_tokens WHERE id = @id AND LEAST(expires_at, revoked_at) < @cutoff;

-- name: ExpiredEnrollmentCodes :many
-- The end of a code is its use or its expiry, whichever came first.
SELECT id FROM enrollment_codes WHERE LEAST(expires_at, used_at) < @cutoff
ORDER BY LEAST(expires_at, used_at), id LIMIT @row_limit;

-- name: DeleteEnrollmentCode :execrows
DELETE FROM enrollment_codes WHERE id = @id AND LEAST(expires_at, used_at) < @cutoff;
