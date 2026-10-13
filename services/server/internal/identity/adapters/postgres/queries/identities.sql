-- SPDX-License-Identifier: Apache-2.0
-- Queries on user_identities and the counts of the remaining sign-in methods (identity,
-- docs/spec/03-datenmodell.md section 4.2, docs/spec/08-identitaet.md sections 3.1 and 3.3, M1-36).

-- name: InsertIdentity :one
INSERT INTO user_identities (id, user_id, provider_id, subject, issuer, email_at_link, linked_at)
VALUES (@id, @user_id, @provider_id, @subject, sqlc.narg(issuer), sqlc.narg(email_at_link), @linked_at)
RETURNING *;

-- name: GetActiveIdentityBySubject :one
SELECT * FROM user_identities
WHERE provider_id = @provider_id AND subject = @subject AND unlinked_at IS NULL;

-- name: ShareActiveIdentityBySubject :one
-- Inside a login unit: a concurrent unlink waits until the login has committed its session.
SELECT * FROM user_identities
WHERE provider_id = @provider_id AND subject = @subject AND unlinked_at IS NULL
FOR SHARE;

-- name: GetIdentityOfUser :one
SELECT sqlc.embed(i), p.kind AS provider_kind, p.enabled AS provider_enabled
FROM user_identities i JOIN identity_providers p ON p.id = i.provider_id
WHERE i.user_id = @user_id AND i.id = @id;

-- name: ListActiveIdentitiesOfUser :many
SELECT sqlc.embed(i), p.kind AS provider_kind, p.enabled AS provider_enabled
FROM user_identities i JOIN identity_providers p ON p.id = i.provider_id
WHERE i.user_id = @user_id AND i.unlinked_at IS NULL
ORDER BY i.linked_at, i.id;

-- name: LockActiveIdentitiesOfUser :many
-- In ascending id (03 section 6.3), so two unlinks of one person serialize (08 section 3.3).
SELECT id FROM user_identities
WHERE user_id = @user_id AND unlinked_at IS NULL
ORDER BY id
FOR UPDATE;

-- name: CountOtherIdentitiesAtEnabledProviders :one
SELECT count(*) FROM user_identities i JOIN identity_providers p ON p.id = i.provider_id
WHERE i.user_id = @user_id AND i.unlinked_at IS NULL AND p.enabled AND i.id <> @except_id;

-- name: CountPairingDevicesOfUser :one
-- Active devices that can approve a pairing with step-up (08 section 5.3); a cli device has no
-- step-up (ID-T29).
SELECT count(*) FROM devices
WHERE user_id = @user_id AND state = 'active' AND platform <> 'cli';

-- name: UnlinkIdentity :execrows
UPDATE user_identities SET unlinked_at = @at WHERE id = @id AND unlinked_at IS NULL;

-- name: MarkIdentityLogin :execrows
UPDATE user_identities SET last_login_at = @at WHERE id = @id;

-- name: LiveSessionsOfIdentity :many
SELECT id FROM auth_sessions
WHERE identity_id = @identity_id AND revoked_at IS NULL AND expires_at > @now
ORDER BY id;
