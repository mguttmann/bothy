-- SPDX-License-Identifier: Apache-2.0
-- Redaction of event payloads on deletion (M1-58, docs/spec/03-data-model.md section 6.5,
-- R-DM-18): bothy_app may change payload and redacted_at of event_log and no other column, so
-- the envelope stays append-only. The column right is granted on the partitioned table; queries
-- through it reach every partition.

-- +goose Up
GRANT UPDATE (payload, redacted_at) ON event_log TO bothy_app;

-- +goose Down
REVOKE UPDATE (payload, redacted_at) ON event_log FROM bothy_app;
