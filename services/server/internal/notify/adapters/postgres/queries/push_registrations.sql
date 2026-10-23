-- SPDX-License-Identifier: Apache-2.0
-- Push registrations (M3-19; 03 push_registrations, 13 sections 7.3 and 12 point 5). Only the
-- module notify reads and writes the table (03 principle 12). These reads have no visibility
-- predicate by design: the consumer push and its jobs select the people or the registration
-- themselves, and every hint is checked with push.deliver (07 7.4 rule 1). This ticket writes only
-- the delivery columns; creation and removal are M3-22.

-- name: ActiveRegistrations :many
-- The active registrations of the given people for the read port notify.PushRegistrations, on
-- the index (user_id) WHERE disabled_at IS NULL. Never the token or the preview hash.
SELECT id, user_id, device_id, provider, environment, app_id, created_at, last_success_at, failure_count
FROM push_registrations
WHERE user_id = ANY (@user_ids::uuid[]) AND disabled_at IS NULL
ORDER BY user_id, id;

-- name: LoadRegistration :one
-- One registration with its token and delivery state for the job notify.push_send.
SELECT id, user_id, device_id, provider, token, environment, app_id, created_at, last_success_at, failure_count,
       disabled_at, disabled_reason
FROM push_registrations
WHERE id = @id;

-- name: LockRegistration :one
-- The delivery state of one registration, locked until the end of the transaction, so two
-- results of the same registration apply one after the other (13 7.3).
SELECT created_at, last_success_at, failure_count, disabled_at, disabled_reason
FROM push_registrations
WHERE id = @id
FOR UPDATE;

-- name: UpdateRegistrationDelivery :execrows
-- Writes only the delivery columns of one registration (design note 11 d).
UPDATE push_registrations
SET last_success_at = sqlc.narg(last_success_at), failure_count = @failure_count, disabled_at = sqlc.narg(disabled_at),
    disabled_reason = sqlc.narg(disabled_reason)
WHERE id = @id;

-- name: ActiveRegistrationCounts :many
-- The data points of the gauge bothy.push.registrations.active: active rows per provider.
SELECT provider, count(*)::bigint AS active
FROM push_registrations
WHERE disabled_at IS NULL
GROUP BY provider
ORDER BY provider;
