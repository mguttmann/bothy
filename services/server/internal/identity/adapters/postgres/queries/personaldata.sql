-- SPDX-License-Identifier: Apache-2.0
-- The participant of identity in the privacy requests (M7-06, docs/spec/20-privacy.md 4.4 and
-- 5.2): the reads of the export (own, non-secret columns only: never a hash, key, public key,
-- credential id, nonce, ciphertext or counter of the security state), the deletes of step 2 and
-- the pseudonymization of step 6. Every statement selects by the person, so a row of another
-- person never matches.

-- name: PersonalPrincipal :one
SELECT p.id, p.kind, p.handle, p.display_name, p.status, p.avatar_file_id, p.created_at,
       p.disabled_at, p.deleted_at, u.email, u.locale, u.time_zone, u.last_active_at
FROM principals p
JOIN users u ON u.principal_id = p.id
WHERE p.id = @user_id;

-- name: PersonalOwners :many
SELECT id, granted_at, revoked_at, reason FROM owners WHERE user_id = @user_id ORDER BY granted_at, id;

-- name: PersonalIdentities :many
SELECT id, provider_id, subject, issuer, email_at_link, linked_at, last_login_at, unlinked_at
FROM user_identities WHERE user_id = @user_id ORDER BY linked_at, id;

-- name: PersonalPasskeys :many
SELECT id, name, transports, created_at, last_used_at, revoked_at
FROM passkey_credentials WHERE user_id = @user_id ORDER BY created_at, id;

-- name: PersonalLocalCredentials :many
SELECT c.identity_id, c.password_changed_at, c.totp_enabled_at
FROM local_credentials c
JOIN user_identities i ON i.id = c.identity_id
WHERE i.user_id = @user_id
ORDER BY c.identity_id;

-- name: PersonalDevices :many
SELECT id, name, platform, state, app_version, created_at, approved_at, last_seen_at, revoked_at, revoke_reason
FROM devices WHERE user_id = @user_id ORDER BY created_at, id;

-- name: PersonalSessions :many
SELECT id, device_id, client_component, auth_method, created_at, expires_at, last_seen_at, last_ip,
       revoked_at, revoke_reason
FROM auth_sessions WHERE user_id = @user_id ORDER BY created_at, id;

-- name: PersonalAPITokens :many
SELECT id, name, created_at, last_used_at, expires_at, revoked_at
FROM api_tokens WHERE principal_id = @user_id ORDER BY created_at, id;

-- name: EraseRefreshTokensOfUser :execrows
DELETE FROM refresh_tokens t USING refresh_token_families f, auth_sessions s
WHERE t.family_id = f.id AND f.session_id = s.id AND s.user_id = @user_id;

-- name: EraseRefreshFamiliesOfUser :execrows
DELETE FROM refresh_token_families f USING auth_sessions s
WHERE f.session_id = s.id AND s.user_id = @user_id;

-- name: EraseChallengesOfUser :execrows
-- Step-up and passkey registration challenges of the person's sessions and passkey enrollment
-- challenges of the person's enrollment codes; a passkey login challenge names no person.
DELETE FROM auth_challenges c
WHERE c.session_id IN (SELECT s.id FROM auth_sessions s WHERE s.user_id = @user_id)
   OR c.enrollment_code_id IN (SELECT e.id FROM enrollment_codes e WHERE e.user_id = @user_id);

-- name: ErasePairingsOfUser :execrows
DELETE FROM device_pairings WHERE user_id = @user_id;

-- name: EraseLoginFlowsOfUser :execrows
DELETE FROM auth_login_flows WHERE user_id = @user_id;

-- name: EraseEnrollmentCodesOfUser :execrows
DELETE FROM enrollment_codes WHERE user_id = @user_id;

-- name: EraseAPITokensOfUser :execrows
-- The personal tokens: principal_id is the person. A token the person created for an agent or
-- an app belongs to that principal and stays (created_by keeps the pseudonymous ID).
DELETE FROM api_tokens WHERE principal_id = @user_id;

-- name: ErasePasskeysOfUser :execrows
DELETE FROM passkey_credentials WHERE user_id = @user_id;

-- name: EraseLocalCredentialsOfUser :execrows
DELETE FROM local_credentials c USING user_identities i WHERE c.identity_id = i.id AND i.user_id = @user_id;

-- name: EraseIdentitiesOfUser :execrows
-- auth_sessions.identity_id becomes NULL by its foreign key (ON DELETE SET NULL).
DELETE FROM user_identities WHERE user_id = @user_id;

-- name: EraseDeviceKeysOfUser :execrows
DELETE FROM device_keys k USING devices d WHERE k.device_id = d.id AND d.user_id = @user_id;

-- name: SessionIDsOfUser :many
-- Children (share_ext) before their parents, so a parent goes after its children were tried.
SELECT id FROM auth_sessions WHERE user_id = @user_id ORDER BY (parent_session_id IS NULL), id;

-- name: EraseSessionOfUser :execrows
DELETE FROM auth_sessions WHERE id = @id AND user_id = @user_id;

-- name: DeviceIDsOfUser :many
SELECT id FROM devices WHERE user_id = @user_id ORDER BY id;

-- name: EraseDeviceOfUser :execrows
DELETE FROM devices WHERE id = @id AND user_id = @user_id;

-- name: ScrubKeptSessionsOfUser :execrows
-- A session another module still holds (a terminal session of terminal, ON DELETE RESTRICT) stays
-- as the pseudonymous record of 20 section 2; its last IP goes now.
UPDATE auth_sessions SET last_ip = NULL WHERE user_id = @user_id AND last_ip IS NOT NULL;

-- name: ScrubKeptDevicesOfUser :execrows
-- A device another module still holds stays; its name (often with names of persons) and its app
-- version go now.
UPDATE devices SET name = @name, app_version = NULL
WHERE user_id = @user_id AND (name <> @name OR app_version IS NOT NULL);

-- name: PseudonymizePrincipal :execrows
-- Step 6 of 20 section 4.4 on principals; only a person not yet deleted changes, so a repeat
-- writes nothing.
UPDATE principals
SET handle = @handle, display_name = @display_name, avatar_file_id = NULL, status = 'deleted',
    deleted_at = @deleted_at
WHERE id = @user_id AND kind = 'user' AND status <> 'deleted';

-- name: PseudonymizeUser :execrows
-- Step 6 on users: no e-mail, no last activity, language and time zone back to the column
-- defaults, so nothing of the person's choice remains.
UPDATE users SET email = NULL, locale = DEFAULT, time_zone = DEFAULT, last_active_at = NULL
WHERE principal_id = @user_id;
