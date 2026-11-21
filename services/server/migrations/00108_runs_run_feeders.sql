-- SPDX-License-Identifier: Apache-2.0
-- The principals who fed a run (FEAT-TEAM-CHANNEL-SERVER, red team round 4, R-DM-63,
-- docs/spec/09-agents.md 3.9.4): run_feeders exactly as docs/spec/03-data-model.md section 4.8
-- states it. One row per run, principal and source; principal_id NULL is an input that speaks for
-- no person (an agent's or an app's follow-up). The rows are the only record the check of a
-- member's reply reads, so they never follow the rows they were derived from: deleting a message,
-- a card or an interaction and revoking a grant leave them; they go only with their run.
--
-- The persisted inputs record themselves here, so no code path can store an input without its
-- feeder: the trigger of a run (runs.triggered_by, at insert), every follow-up (at insert and when
-- it moves to the follow-on run; the authority of a team input, the author of a person's
-- message, NULL otherwise) and every interaction with a component bound to a run. The inputs that
-- are no rows (the deliveries of runs.Delivery and the messages of the run context) are recorded
-- by the runs module in the same unit of work.
--
-- Expand only (upgrade.md rule M1): a new table and three AFTER triggers that the previous binary
-- never reads; the table starts empty and is not backfilled. A run older than this migration has
-- no row of source trigger, and the check refuses a run without one (fail closed); team channels
-- come with 00106 of the same release, so no run of a team member predates the record.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read and insert, never update or
-- delete: the record is append-only for the server; rows go only with their run (ON DELETE
-- CASCADE).

-- +goose Up
CREATE TABLE run_feeders (
    run_id       uuid        NOT NULL,
    principal_id uuid,
    source       text        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT run_feeders_run_id_fkey FOREIGN KEY (run_id) REFERENCES runs (id) ON DELETE CASCADE,
    CONSTRAINT run_feeders_principal_id_fkey FOREIGN KEY (principal_id) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT run_feeders_run_id_principal_id_source_key UNIQUE NULLS NOT DISTINCT (run_id, principal_id, source),
    CONSTRAINT run_feeders_source_check
        CHECK (source IN ('trigger', 'followup', 'interaction', 'command', 'wake_input', 'context'))
);

-- The foreign key to principals needs an index on the referencing side (migrations/README.md);
-- the unique key covers run_id.
CREATE INDEX run_feeders_principal_id_idx ON run_feeders (principal_id);

GRANT SELECT, INSERT ON run_feeders TO bothy_app;

-- +goose StatementBegin
CREATE FUNCTION runs_record_feeder() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
    INSERT INTO public.run_feeders (run_id, principal_id, source)
    VALUES (NEW.id, NEW.triggered_by, 'trigger')
    ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION run_followups_record_feeder() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
    INSERT INTO public.run_feeders (run_id, principal_id, source)
    VALUES (NEW.run_id,
            (SELECT COALESCE(ti.authority_id, CASE WHEN m.origin = 'human' THEN m.author_principal_id END)
             FROM public.messages m LEFT JOIN public.team_inputs ti ON ti.message_id = m.id
             WHERE m.id = NEW.message_id),
            'followup')
    ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION interactions_record_feeder() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
    INSERT INTO public.run_feeders (run_id, principal_id, source)
    SELECT c.run_id, NEW.principal_id, 'interaction'
    FROM public.message_components c
    WHERE c.id = NEW.component_id AND c.run_id IS NOT NULL
    ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER runs_record_feeder
    AFTER INSERT ON runs
    FOR EACH ROW EXECUTE FUNCTION runs_record_feeder();
CREATE TRIGGER run_followups_record_feeder
    AFTER INSERT OR UPDATE OF run_id ON run_followups
    FOR EACH ROW EXECUTE FUNCTION run_followups_record_feeder();
CREATE TRIGGER interactions_record_feeder
    AFTER INSERT ON interactions
    FOR EACH ROW EXECUTE FUNCTION interactions_record_feeder();

-- Trigger functions fire without EXECUTE; nobody needs to call them directly.
REVOKE ALL ON FUNCTION runs_record_feeder() FROM PUBLIC;
REVOKE ALL ON FUNCTION run_followups_record_feeder() FROM PUBLIC;
REVOKE ALL ON FUNCTION interactions_record_feeder() FROM PUBLIC;

-- +goose Down
DROP TRIGGER interactions_record_feeder ON interactions;
DROP TRIGGER run_followups_record_feeder ON run_followups;
DROP TRIGGER runs_record_feeder ON runs;
DROP FUNCTION interactions_record_feeder();
DROP FUNCTION run_followups_record_feeder();
DROP FUNCTION runs_record_feeder();
DROP TABLE run_feeders;
