-- SPDX-License-Identifier: Apache-2.0
-- Queries of the terminal across several api nodes (M2-47): the signals on the channel ephemeral
-- and the reconciliation of a node after its LISTEN connection was lost.

-- name: NotifyEphemeral :exec
-- One terminal signal on the channel ephemeral (03 section 6.6): kind and IDs only. Inside a unit
-- of work PostgreSQL delivers it only after the commit and never after a rollback.
SELECT pg_notify('ephemeral', sqlc.arg(payload)::text);

-- name: HostStatesByIDs :many
-- State and disabled_reason of the hosts of the connections of this node (Design note 9 of
-- M2-47): the relay reads the state of its own connections.
SELECT id, state, disabled_reason FROM hosts WHERE id = ANY(@host_ids::uuid[]) ORDER BY id;
