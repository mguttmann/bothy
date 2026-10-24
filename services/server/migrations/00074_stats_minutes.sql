-- SPDX-License-Identifier: Apache-2.0
-- Own measurement of the stats module (M6-13): stats_minutes exactly as
-- docs/spec/03-data-model.md section 4.15 states it, every column with its type, default and
-- nullability, the checks, the unique key over all dimensions with NULLS NOT DISTINCT (the target
-- of the additive upsert, docs/spec/25-visualization.md 3.4) and the two indexes. No primary key
-- and no foreign key: a row is an aggregate without an identity, and an archived channel keeps
-- its numbers (03). No column for a person and none for content (25 section 4 rule 3).
--
-- The table is partitioned by range of bucket_start, one partition per UTC day named
-- stats_minutes_YYYYMMDD, with no default partition. This migration creates no partition
-- (upgrade.md rule M10): the job stats.retention creates today's and tomorrow's in advance, and a
-- flush that meets a day without a partition creates it through stats_minutes_ensure_partition
-- and writes again (docs/DECISIONS.md D-M6-13-6).
--
-- Grants (migrations/README.md, grant rule): bothy_app reads, inserts and updates (the additive
-- upsert); it never deletes, because retention detaches and drops whole partitions (retention
-- class stats, 20). The partitions are created and dropped only through the two SECURITY DEFINER
-- functions of bothy_owner below, which take only a date or a time and build every name with
-- format('%I'). Lock levels (PostgreSQL 18, sql-altertable.html): ATTACH PARTITION takes SHARE
-- UPDATE EXCLUSIVE on the parent, so flushes go on; a CHECK that matches the bounds lets it skip
-- the validation scan. Plain DETACH PARTITION takes ACCESS EXCLUSIVE on the parent, bounded by the
-- lock timeout passed in (DETACH ... CONCURRENTLY cannot run inside a function).

-- +goose Up
CREATE TABLE stats_minutes (
    bucket_start timestamptz NOT NULL,
    metric       text        NOT NULL,
    channel_id   uuid,
    agent_id     uuid,
    dimension    text        NOT NULL DEFAULT '',
    currency     text,
    count        bigint      NOT NULL DEFAULT 0,
    sum          bigint      NOT NULL DEFAULT 0,
    good         bigint      NOT NULL DEFAULT 0,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT stats_minutes_bucket_start_check CHECK (bucket_start = date_trunc('minute', bucket_start)),
    CONSTRAINT stats_minutes_metric_check CHECK (metric IN ('messages_created','threads_created','runs_started','runs_finished','tool_calls','approval_cards','cost','invocations_denied','billing_mode_switches','catchups','resyncs','slow_consumer_closes','sli.api_availability','sli.delivery_latency','sli.reconnect','sli.push_acceptance','sli.run_start','sli.turn_duration')),
    CONSTRAINT stats_minutes_counts_check CHECK (count >= 0 AND sum >= 0 AND good >= 0),
    CONSTRAINT stats_minutes_good_check CHECK (good <= count),
    CONSTRAINT stats_minutes_currency_check CHECK ((currency IS NOT NULL) = (metric = 'cost')),
    CONSTRAINT stats_minutes_sli_good_check CHECK (metric LIKE 'sli.%' OR good = 0),
    CONSTRAINT stats_minutes_key UNIQUE NULLS NOT DISTINCT (bucket_start, metric, channel_id, agent_id, dimension, currency)
) PARTITION BY RANGE (bucket_start);

-- The timeline and the traffic light read by metric and time.
CREATE INDEX stats_minutes_metric_bucket_idx ON stats_minutes (metric, bucket_start);
-- The permission filter reads by channel.
CREATE INDEX stats_minutes_channel_bucket_idx ON stats_minutes (channel_id, bucket_start) WHERE channel_id IS NOT NULL;

GRANT SELECT, INSERT, UPDATE ON stats_minutes TO bothy_app;

-- stats_minutes_ensure_partition creates the partition of the UTC day p_day,
-- [p_day 00:00 UTC, p_day + 1 00:00 UTC), unless it exists, and returns 1 when it created it and 0
-- otherwise. The partition is a standalone table LIKE stats_minutes with a CHECK of its bounds,
-- attached and then relieved of the CHECK. The LOCK first serialises concurrent calls (the job and
-- the flushes of several processes), so the second sees the partition the first attached.
-- +goose StatementBegin
CREATE FUNCTION stats_minutes_ensure_partition(p_day date) RETURNS int
    LANGUAGE plpgsql
    SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
DECLARE
    v_name text;
    v_from timestamptz;
    v_to timestamptz;
BEGIN
    IF p_day IS NULL OR p_day < date '2000-01-01' OR p_day > date '9999-12-30' THEN
        RAISE EXCEPTION 'stats_minutes_ensure_partition: the day must be set and between 2000-01-01 and 9999-12-30'
            USING ERRCODE = 'invalid_parameter_value';
    END IF;
    v_name := 'stats_minutes_' || to_char(p_day, 'YYYYMMDD');
    LOCK TABLE stats_minutes IN SHARE UPDATE EXCLUSIVE MODE;
    IF EXISTS (SELECT 1 FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
                WHERE i.inhparent = 'stats_minutes'::regclass AND c.relname = v_name) THEN
        RETURN 0;
    END IF;
    v_from := p_day::timestamp AT TIME ZONE 'UTC';
    v_to := (p_day + 1)::timestamp AT TIME ZONE 'UTC';
    EXECUTE format('CREATE TABLE %I (LIKE stats_minutes INCLUDING DEFAULTS INCLUDING CONSTRAINTS)', v_name);
    EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I CHECK (bucket_start IS NOT NULL AND bucket_start >= %L AND bucket_start < %L)',
                   v_name, v_name || '_bounds', v_from, v_to);
    EXECUTE format('ALTER TABLE stats_minutes ATTACH PARTITION %I FOR VALUES FROM (%L) TO (%L)', v_name, v_from, v_to);
    EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', v_name, v_name || '_bounds');
    RETURN 1;
END;
$$;
-- +goose StatementEnd

-- stats_minutes_drop_expired detaches and drops every partition whose day ends at or before
-- p_cutoff (retention class stats: the job passes now minus retention.stats_minutes). The cutoff
-- is never later than now(), so a caller cannot drop the partition of the current day.
-- lock_timeout is set to p_lock_timeout for the rest of the transaction. It returns the number of
-- partitions dropped.
-- +goose StatementBegin
CREATE FUNCTION stats_minutes_drop_expired(p_cutoff timestamptz, p_lock_timeout interval) RETURNS int
    LANGUAGE plpgsql
    SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
DECLARE
    r record;
    v_cutoff timestamptz;
    v_dropped int := 0;
BEGIN
    IF p_cutoff IS NULL OR p_lock_timeout IS NULL OR p_lock_timeout <= interval '0' THEN
        RAISE EXCEPTION 'stats_minutes_drop_expired: cutoff and a positive lock timeout must be set'
            USING ERRCODE = 'invalid_parameter_value';
    END IF;
    PERFORM set_config('lock_timeout', ceil(extract(epoch FROM p_lock_timeout) * 1000)::bigint::text || 'ms', true);
    v_cutoff := least(p_cutoff, now());
    FOR r IN
        SELECT c.relname AS name
          FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
         WHERE i.inhparent = 'stats_minutes'::regclass
           AND c.relname ~ '^stats_minutes_[0-9]{8}$'
           AND (to_date(substr(c.relname, 15), 'YYYYMMDD') + 1)::timestamp AT TIME ZONE 'UTC' <= v_cutoff
         ORDER BY c.relname
    LOOP
        EXECUTE format('ALTER TABLE stats_minutes DETACH PARTITION %I', r.name);
        EXECUTE format('DROP TABLE %I', r.name);
        v_dropped := v_dropped + 1;
    END LOOP;
    RETURN v_dropped;
END;
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION stats_minutes_ensure_partition(date) FROM PUBLIC;
REVOKE ALL ON FUNCTION stats_minutes_drop_expired(timestamptz, interval) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION stats_minutes_ensure_partition(date) TO bothy_app;
GRANT EXECUTE ON FUNCTION stats_minutes_drop_expired(timestamptz, interval) TO bothy_app;

-- +goose Down
DROP FUNCTION stats_minutes_drop_expired(timestamptz, interval);
DROP FUNCTION stats_minutes_ensure_partition(date);
DROP TABLE stats_minutes;
