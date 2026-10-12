-- SPDX-License-Identifier: Apache-2.0
-- Module sync (M1-16): the event log with the global, commit-ordered sequence
-- (docs/spec/03-datenmodell.md 4.10 event_log, B8, ADR 0009, R-DM-15). Only the module sync reads
-- and writes it (03 principle 12).
--
-- event_log is partitioned by range of seq with sync.event_log_partition_size numbers per
-- partition and has no default partition and no foreign key. This migration creates no
-- partition: bothy migrate up and the job sync.partition_maintenance create them through
-- sync_event_log_ensure_partitions, never a migration in operation (upgrade.md rule M10).
--
-- bothy_app keeps DML only (16 "Rollen", R-OP-17): SELECT and INSERT on the table (the envelope is
-- append-only, R-DM-18; redaction adds its column grant with M1-58) and EXECUTE on two SECURITY
-- DEFINER functions of bothy_owner, which take only numbers and times and build every name with
-- format('%I'). Lock levels (PostgreSQL 18, sql-altertable.html and ddl-partitioning.html):
-- ATTACH PARTITION takes SHARE UPDATE EXCLUSIVE on the parent, so inserts go on; a CHECK constraint
-- that matches the bounds lets it skip the validation scan of the new table. Plain DETACH PARTITION
-- takes ACCESS EXCLUSIVE on the parent, bounded by the lock timeout passed in (DETACH ...
-- CONCURRENTLY cannot run inside a function, it needs its own transactions).

-- +goose Up
CREATE TABLE event_log (
    seq bigint NOT NULL,
    id uuid NOT NULL,
    type text NOT NULL,
    schema_version smallint NOT NULL DEFAULT 1,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    actor_kind text,
    actor_principal_id uuid,
    space_id uuid,
    channel_id uuid,
    thread_id uuid,
    resource_kind text NOT NULL,
    resource_id uuid,
    resource_version bigint,
    audience_principal_id uuid,
    receive_action text NOT NULL,
    payload jsonb NOT NULL,
    truncated boolean NOT NULL DEFAULT false,
    redacted_at timestamptz,
    correlation_id uuid,
    trace_context text,
    client_msg_id uuid,
    permission_version bigint NOT NULL,
    CONSTRAINT event_log_pkey PRIMARY KEY (seq),
    CONSTRAINT event_log_id_seq_key UNIQUE (id, seq),
    CONSTRAINT event_log_actor_kind_check CHECK (actor_kind IN ('user', 'agent', 'app', 'system')),
    CONSTRAINT event_log_actor_principal_check
        CHECK (COALESCE(actor_kind IN ('user', 'agent', 'app'), false) = (actor_principal_id IS NOT NULL)),
    CONSTRAINT event_log_self_audience_check CHECK (receive_action <> 'self' OR audience_principal_id IS NOT NULL)
) PARTITION BY RANGE (seq);

CREATE INDEX event_log_channel_seq_idx ON event_log (channel_id, seq);
CREATE INDEX event_log_space_seq_idx ON event_log (space_id, seq);
CREATE INDEX event_log_audience_seq_idx ON event_log (audience_principal_id, seq) WHERE audience_principal_id IS NOT NULL;
CREATE INDEX event_log_resource_idx ON event_log (resource_kind, resource_id);
CREATE INDEX event_log_occurred_at_brin ON event_log USING brin (occurred_at);

GRANT SELECT, INSERT ON event_log TO bothy_app;

-- sync_event_log_ensure_partitions creates partitions [upper, upper + p_size) while the highest
-- upper bound is at most p_head + p_ahead * p_size; the first partition starts at 1. Each new
-- partition is a standalone table LIKE event_log with a CHECK of its bounds, attached and then
-- relieved of the CHECK. The LOCK first serialises concurrent calls (the job and bothy migrate
-- up), so the second reads the bounds the first created and the function stays idempotent.
-- The bounds on size and ahead keep one call from creating more partitions than the
-- configuration allows (sync.event_log_partition_size, sync.event_log_partitions_ahead) and keep
-- p_head + p_ahead * p_size inside bigint. It returns the number of partitions created.
-- +goose StatementBegin
CREATE FUNCTION sync_event_log_ensure_partitions(p_size bigint, p_ahead int, p_head bigint) RETURNS int
    LANGUAGE plpgsql
    SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
DECLARE
    v_upper bigint;
    v_name text;
    v_created int := 0;
BEGIN
    IF p_size IS NULL OR p_size < 1 OR p_size > 1000000000000 OR p_ahead IS NULL OR p_ahead < 0
       OR p_ahead > 1000 OR p_head IS NULL OR p_head < 0 THEN
        RAISE EXCEPTION 'sync_event_log_ensure_partitions: size must be 1 to 10^12, ahead 0 to 1000, head at least 0'
            USING ERRCODE = 'invalid_parameter_value';
    END IF;
    LOCK TABLE event_log IN SHARE UPDATE EXCLUSIVE MODE;
    SELECT max((regexp_match(pg_get_expr(c.relpartbound, c.oid), 'TO \(''?(-?[0-9]+)''?\)'))[1]::bigint)
      INTO v_upper
      FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
     WHERE i.inhparent = 'event_log'::regclass;
    v_upper := COALESCE(v_upper, 1);
    WHILE v_upper <= p_head + p_ahead::bigint * p_size LOOP
        v_name := 'event_log_from_' || lpad(v_upper::text, 19, '0');
        EXECUTE format('CREATE TABLE %I (LIKE event_log INCLUDING DEFAULTS INCLUDING CONSTRAINTS)', v_name);
        EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I CHECK (seq IS NOT NULL AND seq >= %s AND seq < %s)',
                       v_name, v_name || '_bounds', v_upper, v_upper + p_size);
        EXECUTE format('ALTER TABLE event_log ATTACH PARTITION %I FOR VALUES FROM (%s) TO (%s)',
                       v_name, v_upper, v_upper + p_size);
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', v_name, v_name || '_bounds');
        v_upper := v_upper + p_size;
        v_created := v_created + 1;
    END LOOP;
    RETURN v_created;
END;
$$;
-- +goose StatementEnd

-- sync_event_log_drop_expired detaches and drops, in the order of their bounds, the partitions
-- whose upper bound is at most p_head (the partition holding the head is never touched, 03
-- event_log "Aufbewahrung") and whose largest occurred_at is older than p_cutoff or that are
-- empty. It stops at the first partition it keeps, so the retained sequence never has a hole
-- (occurred_at is the start of a transaction and need not grow with seq). lock_timeout is set
-- to p_lock_timeout for the rest of the transaction. The function trusts neither argument beyond
-- the table: the head is at most max(seq) and the cutoff at most now(), so a caller cannot drop
-- the head partition or events younger than the call. It returns the number of partitions
-- dropped.
-- +goose StatementBegin
CREATE FUNCTION sync_event_log_drop_expired(p_cutoff timestamptz, p_head bigint, p_lock_timeout interval) RETURNS int
    LANGUAGE plpgsql
    SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
DECLARE
    r record;
    v_newest timestamptz;
    v_head bigint;
    v_cutoff timestamptz;
    v_dropped int := 0;
BEGIN
    IF p_cutoff IS NULL OR p_head IS NULL OR p_head < 0 OR p_lock_timeout IS NULL OR p_lock_timeout <= interval '0' THEN
        RAISE EXCEPTION 'sync_event_log_drop_expired: cutoff and lock timeout must be set, head at least 0'
            USING ERRCODE = 'invalid_parameter_value';
    END IF;
    PERFORM set_config('lock_timeout', ceil(extract(epoch FROM p_lock_timeout) * 1000)::bigint::text || 'ms', true);
    SELECT least(p_head, COALESCE(max(seq), 0)) INTO v_head FROM event_log;
    v_cutoff := least(p_cutoff, now());
    FOR r IN
        SELECT c.relname AS name,
               (regexp_match(pg_get_expr(c.relpartbound, c.oid), 'TO \(''?(-?[0-9]+)''?\)'))[1]::bigint AS upper
          FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
         WHERE i.inhparent = 'event_log'::regclass
         ORDER BY 2
    LOOP
        EXIT WHEN r.upper > v_head;
        EXECUTE format('SELECT max(occurred_at) FROM %I', r.name) INTO v_newest;
        EXIT WHEN v_newest IS NOT NULL AND v_newest >= v_cutoff;
        EXECUTE format('ALTER TABLE event_log DETACH PARTITION %I', r.name);
        EXECUTE format('DROP TABLE %I', r.name);
        v_dropped := v_dropped + 1;
    END LOOP;
    RETURN v_dropped;
END;
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION sync_event_log_ensure_partitions(bigint, int, bigint) FROM PUBLIC;
REVOKE ALL ON FUNCTION sync_event_log_drop_expired(timestamptz, bigint, interval) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION sync_event_log_ensure_partitions(bigint, int, bigint) TO bothy_app;
GRANT EXECUTE ON FUNCTION sync_event_log_drop_expired(timestamptz, bigint, interval) TO bothy_app;

-- +goose Down
DROP FUNCTION sync_event_log_drop_expired(timestamptz, bigint, interval);
DROP FUNCTION sync_event_log_ensure_partitions(bigint, int, bigint);
DROP TABLE event_log;
