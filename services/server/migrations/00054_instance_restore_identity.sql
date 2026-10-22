-- SPDX-License-Identifier: Apache-2.0
-- Module instance (M7-04): the identity of the database cluster on the instance row, exactly as
-- docs/spec/03-data-model.md 4.1 states it (R-DM-50). pg_system_identifier is system_identifier
-- of pg_control_system() as decimal text (an unsigned 64-bit number does not fit bigint), and
-- pg_timeline_id is timeline_id of pg_control_checkpoint(). Both stay empty here: bothy migrate up
-- fills them while they are empty, and only bothy restore finalize replaces them (03 6.7 item 4).
--
-- No grant: bothy_app already reads the whole row (SELECT on instance, 00002) and never writes
-- these columns. PostgreSQL 18 grants EXECUTE on both control functions to PUBLIC by default
-- (proacl NULL), so the readiness check of the server needs no grant either.

-- +goose Up
ALTER TABLE instance
    ADD COLUMN pg_system_identifier text,
    ADD COLUMN pg_timeline_id bigint;

-- +goose Down
ALTER TABLE instance
    DROP COLUMN pg_timeline_id,
    DROP COLUMN pg_system_identifier;
