-- SPDX-License-Identifier: Apache-2.0
-- Queries on webhook_subscriptions and webhook_deliveries (apps module, M2-26,
-- docs/spec/03-data-model.md section 4.11). Lock order (03 section 6.3): the installation, then
-- the subscription row, then its delivery rows.

-- name: InsertSubscription :one
INSERT INTO webhook_subscriptions (id, app_installation_id, url, event_types, signing_key_kid, hmac_enabled,
    hmac_generation, state, created_by)
VALUES (@id, @app_installation_id, @url, @event_types, @signing_key_kid, @hmac_enabled, 0, 'active', @created_by)
RETURNING *;

-- name: GetSubscription :one
SELECT sqlc.embed(s), i.app_id FROM webhook_subscriptions s
JOIN app_installations i ON i.id = s.app_installation_id
WHERE s.id = @id;

-- name: LockSubscription :one
SELECT sqlc.embed(s), i.app_id FROM webhook_subscriptions s
JOIN app_installations i ON i.id = s.app_installation_id
WHERE s.id = @id
FOR UPDATE OF s;

-- name: UpdateSubscription :execrows
UPDATE webhook_subscriptions SET url = @url, event_types = @event_types, signing_key_kid = @signing_key_kid,
    hmac_generation = @hmac_generation, hmac_rotated_at = sqlc.narg(hmac_rotated_at), state = @state,
    consecutive_failures = @consecutive_failures, last_delivered_seq = @last_delivered_seq,
    disabled_at = sqlc.narg(disabled_at)
WHERE id = @id;

-- name: DeleteSubscription :execrows
DELETE FROM webhook_subscriptions WHERE id = @id;

-- name: ListAppSubscriptions :many
SELECT sqlc.embed(s), i.app_id FROM webhook_subscriptions s
JOIN app_installations i ON i.id = s.app_installation_id
WHERE i.app_id = @app_id
ORDER BY s.created_at, s.id;

-- name: LockInstallationSubscriptions :many
-- uninstallApp and resumeApp change every subscription of an installation in their unit.
SELECT sqlc.embed(s), i.app_id FROM webhook_subscriptions s
JOIN app_installations i ON i.id = s.app_installation_id
WHERE s.app_installation_id = @installation_id
ORDER BY s.id
FOR UPDATE OF s;

-- name: LockLiveSubscriptions :many
-- The lost range of the consumer round: every app subscription that could have matched a lost
-- event (M2-26 design note 3 f).
SELECT sqlc.embed(s), i.app_id FROM webhook_subscriptions s
JOIN app_installations i ON i.id = s.app_installation_id
WHERE s.state IN ('active', 'paused') AND i.state IN ('active', 'suspended')
ORDER BY s.id
FOR UPDATE OF s;

-- name: ShareLiveInstallation :one
-- createWebhookSubscription: the current installation of the app, active or suspended.
SELECT sqlc.embed(i) FROM app_installations i
WHERE i.app_id = @app_id AND i.state IN ('active', 'suspended')
FOR SHARE;

-- name: GetInstallation :one
SELECT sqlc.embed(i) FROM app_installations i WHERE i.id = @id;

-- name: Candidates :many
-- The candidates of a consumer round (M2-26 design note 3 b): app subscriptions in state active or
-- paused whose installation is active or suspended.
SELECT s.id, s.event_types, s.state, i.id AS installation_id, i.app_id, i.state AS installation_state,
       i.event_subscriptions
FROM webhook_subscriptions s
JOIN app_installations i ON i.id = s.app_installation_id
WHERE s.state IN ('active', 'paused') AND i.state IN ('active', 'suspended')
ORDER BY s.id;

-- name: InsertDelivery :execrows
INSERT INTO webhook_deliveries (id, subscription_id, event_seq, event_id, status, attempt, created_at)
VALUES (@id, @subscription_id, @event_seq, @event_id, 'pending', 0, @created_at)
ON CONFLICT (subscription_id, event_seq) DO NOTHING;

-- name: CountPending :one
-- The open deliveries of a subscription, counted up to a bound (the delivery quota).
SELECT count(*)::bigint FROM (
    SELECT 1 FROM webhook_deliveries
    WHERE subscription_id = @subscription_id AND status = 'pending'
    LIMIT @bound
) p;

-- name: HeadDelivery :one
-- The head of a subscription: the pending row with the lowest event_seq.
SELECT * FROM webhook_deliveries
WHERE subscription_id = @subscription_id AND status = 'pending'
ORDER BY event_seq
LIMIT 1;

-- name: GetDelivery :one
SELECT * FROM webhook_deliveries WHERE id = @id;

-- name: UpdateDelivery :execrows
UPDATE webhook_deliveries SET attempt = @attempt, status = @status, response_code = sqlc.narg(response_code),
    last_error = sqlc.narg(last_error), next_attempt_at = sqlc.narg(next_attempt_at), created_at = @created_at,
    delivered_at = sqlc.narg(delivered_at)
WHERE id = @id;

-- name: ExpirePending :many
-- The gap rule: the pending rows below a seq become dead with event_expired; returns their seqs.
UPDATE webhook_deliveries SET status = 'dead', last_error = 'event_expired', next_attempt_at = NULL
WHERE subscription_id = @subscription_id AND status = 'pending' AND event_seq < @below_seq
RETURNING event_seq;

-- name: ListDeliveries :many
-- One page newest first; the keyset is (event_seq, id) descending; an empty status lists all.
SELECT * FROM webhook_deliveries
WHERE subscription_id = @subscription_id
  AND (@status::text = '' OR status = @status::text)
  AND (NOT @has_after::boolean OR (event_seq, id) < (@after_seq::bigint, @after_id::uuid))
ORDER BY event_seq DESC, id DESC
LIMIT @lim;
