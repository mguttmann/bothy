-- SPDX-License-Identifier: Apache-2.0
-- Queries of the pairing of a host (M2-41, 24 sections 3.2 and 3.3). The caller holds the lock of
-- the row (LockHost) inside a rights change for every write.

-- name: HostByPairingCodeHash :one
-- The host of a pairing code, without a lock (the unique partial index hosts_pairing_code_hash_key).
SELECT * FROM hosts WHERE pairing_code_hash = @pairing_code_hash;

-- name: SetHostPairingCode :execrows
-- A new pairing code (issueHostPairingCode, the fallback after too many failures): state pairing,
-- the hash and expiry of the code, the failure counter back at 0, the keys dropped when drop_keys
-- (key_fingerprint stays: a digest of the dropped keys), the disabled state cleared, the version.
UPDATE hosts
SET state = 'pairing', pairing_code_hash = @pairing_code_hash, pairing_expires_at = @pairing_expires_at,
    fingerprint_failures = 0,
    noise_public_key = CASE WHEN @drop_keys::boolean THEN NULL ELSE noise_public_key END,
    auth_public_jwk = CASE WHEN @drop_keys::boolean THEN NULL ELSE auth_public_jwk END,
    auth_jkt = CASE WHEN @drop_keys::boolean THEN NULL ELSE auth_jkt END,
    disabled_at = NULL, disabled_reason = NULL, version = @version
WHERE id = @id;

-- name: StoreHostPairing :execrows
-- The redemption of a pairing code (host.pair): both public keys, the fingerprint, the next key
-- generation, the report of the host, the code used up, state confirming and the version.
UPDATE hosts
SET noise_public_key = @noise_public_key, auth_public_jwk = @auth_public_jwk::jsonb, auth_jkt = @auth_jkt,
    key_fingerprint = @key_fingerprint, key_generation = key_generation + 1, fingerprint_failures = 0,
    paired_at = @paired_at, platform = @platform, reported_os_version = sqlc.narg(reported_os_version),
    reported_host_version = @reported_host_version, reported_profiles = @reported_profiles::jsonb,
    reported_session_users = @reported_session_users::jsonb, pairing_code_hash = NULL, pairing_expires_at = NULL,
    state = 'confirming', version = @version
WHERE id = @id;

-- name: SetHostFingerprintFailures :execrows
-- A failed confirmation; the counter is not part of the representation, so the version stays.
UPDATE hosts SET fingerprint_failures = @fingerprint_failures WHERE id = @id;

-- name: ConfirmHostPairing :execrows
-- The confirmation (confirmHost): the declaration of the server host, who confirmed and when, the
-- failure counter back at 0, state active and the version.
UPDATE hosts
SET declared_server_host = @declared_server_host, confirmed_by = @confirmed_by, confirmed_at = @confirmed_at,
    fingerprint_failures = 0, state = 'active', version = @version
WHERE id = @id;
