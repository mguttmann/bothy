-- SPDX-License-Identifier: Apache-2.0
-- Push registrations, the writes and reads of registerPush, unregisterPush, getPushPreview, the
-- deactivation on a session end or device revocation and the member push_registrations of the
-- schema Device (M3-22; 03 push_registrations, 13 sections 6.4, 7.3 and 13). Only the module notify
-- reads and writes the table (03 principle 12). These statements have no visibility predicate by
-- design: the use cases authorize push.register and push.unregister for the caller, the preview
-- checks token, session, device and message.read itself, and the subscribers run inside the
-- revoking transaction (07 7.4 rule 1). No statement returns the token or the preview hash.

-- name: LockRegistrationByProviderToken :one
-- The registration of (provider, token), active or disabled and of any person, locked until the
-- end of the transaction, so a takeover deletes exactly the row it saw (design note 5 c).
SELECT id
FROM push_registrations
WHERE provider = @provider AND token = @token
FOR UPDATE;

-- name: DeleteRegistration :execrows
-- Removes one registration (takeover and unregisterPush); pending notify.push_send jobs of it end
-- with no_registration.
DELETE FROM push_registrations
WHERE id = @id;

-- name: DisableOtherRegistrationsOfApp :execrows
-- One current token per app installation (design note 5 d): the other active registrations of the
-- device with the same provider and app_id end with the given reason, locked in ascending id
-- (03 6.3 lock order).
UPDATE push_registrations
SET disabled_at = @disabled_at, disabled_reason = @disabled_reason
WHERE id IN (
    SELECT r.id
    FROM push_registrations r
    WHERE r.device_id = @device_id AND r.provider = @provider AND r.app_id = @app_id AND r.disabled_at IS NULL
    ORDER BY r.id
    FOR UPDATE
);

-- name: InsertRegistration :one
-- A new active registration with the HMAC of its preview token (design note 5 e). A row with the
-- same (provider, token) that a concurrent registration committed first gives no row instead of a
-- unique violation: the handler runs inside the transaction of the idempotency stage, which a
-- failed statement would abort, so the use case can take the row over in the same transaction
-- (design note 5 g).
INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, environment, app_id, preview_token_hash,
                                created_at, failure_count)
VALUES (@id, @user_id, @device_id, @auth_session_id, @provider, @token, @environment, @app_id, @preview_token_hash, @created_at, 0)
ON CONFLICT (provider, token) DO NOTHING
RETURNING id, user_id, device_id, provider, environment, app_id, created_at;

-- name: GetRegistrationByPreviewHash :one
-- The active registration of a preview token, on the unique partial index of preview_token_hash
-- (13 6.4). A deactivated row answers like an unknown token.
SELECT id, user_id, device_id, auth_session_id, provider
FROM push_registrations
WHERE preview_token_hash = @preview_token_hash AND disabled_at IS NULL;

-- name: GetRegistrationForOwner :one
-- The person of one registration for push.unregister, locked until the end of the transaction.
SELECT user_id
FROM push_registrations
WHERE id = @id
FOR UPDATE;

-- name: DisableRegistrationsOfSessions :execrows
-- Deactivates the active registrations of ended sessions in the revoking transaction (design note
-- 7 a), on the index (auth_session_id), locked in ascending id. A disabled row keeps its first
-- reason.
UPDATE push_registrations
SET disabled_at = @disabled_at, disabled_reason = @disabled_reason
WHERE id IN (
    SELECT r.id
    FROM push_registrations r
    WHERE r.auth_session_id = ANY (@session_ids::uuid[]) AND r.disabled_at IS NULL
    ORDER BY r.id
    FOR UPDATE
);

-- name: DisableRegistrationsOfDevice :execrows
-- Deactivates every active registration of a revoked device, whatever its session (design note
-- 7 b), locked in ascending id.
UPDATE push_registrations
SET disabled_at = @disabled_at, disabled_reason = @disabled_reason
WHERE id IN (
    SELECT r.id
    FROM push_registrations r
    WHERE r.device_id = @device_id AND r.disabled_at IS NULL
    ORDER BY r.id
    FOR UPDATE
);

-- name: ListRegistrationsOfDevices :many
-- Every registration of the given devices, active and disabled, for the member push_registrations
-- of the schema Device (design note 11). Never the token or the preview hash.
SELECT id, device_id, provider, environment, app_id, created_at, disabled_at, disabled_reason
FROM push_registrations
WHERE device_id = ANY (@device_ids::uuid[])
ORDER BY device_id, id;
