-- SPDX-License-Identifier: Apache-2.0
-- Module instance (M1-15): the instance row and the sequence counters
-- (docs/spec/03-data-model.md 4.1, retention class config). Only the module instance reads and
-- writes these tables (03 principle 12).
--
-- instance holds exactly one row (CHECK id = 1). bothy migrate up creates it as bothy_owner
-- through the port of instance, so no row is inserted here; bothy_app only reads it at start.
--
-- sequence_counters holds the three counter rows, created here with value 0. Their row lock
-- orders event_seq, audit_seq and permission_version by commit (04 "Pattern with a counter row").
-- bothy_app may read them and raise value, never insert, delete or rename a row; the trigger
-- refuses a lower value and a new name for every role, because a lower value would hand out a
-- number twice.

-- +goose Up
CREATE TABLE instance (
    id smallint NOT NULL DEFAULT 1,
    instance_id uuid NOT NULL,
    setup_completed_at timestamptz,
    schema_version bigint NOT NULL,
    min_server_version text,
    sync_epoch uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT instance_pkey PRIMARY KEY (id),
    CONSTRAINT instance_id_check CHECK (id = 1),
    CONSTRAINT instance_instance_id_key UNIQUE (instance_id)
);

CREATE TABLE sequence_counters (
    name text NOT NULL,
    value bigint NOT NULL DEFAULT 0,
    CONSTRAINT sequence_counters_pkey PRIMARY KEY (name),
    CONSTRAINT sequence_counters_name_check CHECK (name IN ('event_seq', 'audit_seq', 'permission_version'))
);

INSERT INTO sequence_counters (name, value) VALUES ('event_seq', 0), ('audit_seq', 0), ('permission_version', 0);

-- +goose StatementBegin
CREATE FUNCTION instance_counter_monotonic() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog
AS $$
BEGIN
    IF NEW.name IS DISTINCT FROM OLD.name THEN
        RAISE EXCEPTION 'sequence_counters: the name of a counter row never changes';
    END IF;
    IF NEW.value < OLD.value THEN
        RAISE EXCEPTION 'sequence_counters: the value of % may never decrease', OLD.name;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION instance_counter_monotonic() FROM PUBLIC;

CREATE TRIGGER sequence_counters_monotonic
    BEFORE UPDATE ON sequence_counters
    FOR EACH ROW EXECUTE FUNCTION instance_counter_monotonic();

GRANT SELECT ON instance TO bothy_app;
GRANT SELECT, UPDATE (value) ON sequence_counters TO bothy_app;

-- +goose Down
DROP TABLE sequence_counters;
DROP FUNCTION instance_counter_monotonic();
DROP TABLE instance;
