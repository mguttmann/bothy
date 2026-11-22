-- SPDX-License-Identifier: Apache-2.0
-- Team channels of the runs module (FEAT-TEAM-CHANNEL-SERVER, R-DM-62, docs/spec/09-agents.md 3.9):
-- team_channels, team_members, team_inputs and team_mirrors exactly as docs/spec/03-data-model.md
-- section 4.8 states them, and runs.trigger_kind accepts 'team', whose input is the
-- triggering_message_id of the run. Retention classes: team_channels and team_members config,
-- team_inputs and team_mirrors content (03 section 3).
--
-- Expand only (upgrade.md rules M1, M3 and M5): new tables that the previous binary never reads;
-- the two wider checks on runs replace the old ones NOT VALID, so they hold for every new and
-- changed row at once, and 00107 validates them without blocking writes. Every stored row keeps
-- its trigger kind. The check of the triggering_* columns was created without a name (00051), so
-- the step finds it by its definition.
--
-- Grants (migrations/README.md, grant rule): bothy_app reads and writes the four tables; a team is
-- switched off and a member removed by DELETE, the inputs and mirrors go with their messages
-- (ON DELETE CASCADE).
--
-- Down: the old checks come back only while no run has trigger_kind 'team'; the step refuses
-- otherwise and names the count.

-- +goose Up
CREATE TABLE team_channels (
    channel_id     uuid        NOT NULL,
    sponsor_id     uuid        NOT NULL,
    member_turns   integer     NOT NULL DEFAULT 0,
    pingpong_from  uuid,
    pingpong_to    uuid,
    pingpong_count integer     NOT NULL DEFAULT 0,
    pause_reason   text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    version        bigint      NOT NULL DEFAULT 1,
    CONSTRAINT team_channels_pkey PRIMARY KEY (channel_id),
    CONSTRAINT team_channels_channel_id_fkey FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE CASCADE,
    CONSTRAINT team_channels_sponsor_id_fkey FOREIGN KEY (sponsor_id) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT team_channels_counts_check CHECK (member_turns >= 0 AND pingpong_count >= 0),
    CONSTRAINT team_channels_pause_reason_check CHECK (pause_reason IN ('turn_budget', 'pingpong', 'stopped'))
);

CREATE TABLE team_members (
    id              uuid        NOT NULL,
    channel_id      uuid        NOT NULL,
    agent_id        uuid        NOT NULL,
    home_channel_id uuid        NOT NULL,
    work_thread_id  uuid        NOT NULL,
    handle          text        NOT NULL,
    created_by      uuid        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT team_members_pkey PRIMARY KEY (id),
    CONSTRAINT team_members_channel_id_fkey FOREIGN KEY (channel_id) REFERENCES team_channels (channel_id) ON DELETE CASCADE,
    CONSTRAINT team_members_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES agents (principal_id) ON DELETE CASCADE,
    CONSTRAINT team_members_home_channel_id_fkey FOREIGN KEY (home_channel_id) REFERENCES channels (id) ON DELETE CASCADE,
    CONSTRAINT team_members_work_thread_id_fkey FOREIGN KEY (work_thread_id) REFERENCES threads (id) ON DELETE CASCADE,
    CONSTRAINT team_members_created_by_fkey FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT team_members_channel_id_handle_key UNIQUE (channel_id, handle),
    CONSTRAINT team_members_channel_id_agent_id_home_channel_id_key UNIQUE (channel_id, agent_id, home_channel_id),
    CONSTRAINT team_members_work_thread_id_key UNIQUE (work_thread_id),
    CONSTRAINT team_members_handle_check CHECK (handle ~ '^[a-z0-9][a-z0-9_-]*$')
);

CREATE TABLE team_inputs (
    message_id        uuid        NOT NULL,
    member_id         uuid        NOT NULL,
    source_message_id uuid        NOT NULL,
    source            text        NOT NULL,
    source_member_id  uuid,
    authority_id      uuid        NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT team_inputs_pkey PRIMARY KEY (message_id),
    CONSTRAINT team_inputs_message_id_fkey FOREIGN KEY (message_id) REFERENCES messages (id) ON DELETE CASCADE,
    CONSTRAINT team_inputs_member_id_fkey FOREIGN KEY (member_id) REFERENCES team_members (id) ON DELETE CASCADE,
    CONSTRAINT team_inputs_source_message_id_fkey FOREIGN KEY (source_message_id) REFERENCES messages (id) ON DELETE CASCADE,
    CONSTRAINT team_inputs_source_member_id_fkey FOREIGN KEY (source_member_id) REFERENCES team_members (id) ON DELETE SET NULL,
    CONSTRAINT team_inputs_authority_id_fkey FOREIGN KEY (authority_id) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT team_inputs_source_message_id_member_id_key UNIQUE (source_message_id, member_id),
    CONSTRAINT team_inputs_source_check CHECK (source IN ('owner', 'member'))
);

CREATE TABLE team_mirrors (
    source_message_id uuid        NOT NULL,
    mirror_message_id uuid        NOT NULL,
    member_id         uuid,
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT team_mirrors_pkey PRIMARY KEY (source_message_id),
    CONSTRAINT team_mirrors_source_message_id_fkey FOREIGN KEY (source_message_id) REFERENCES messages (id) ON DELETE CASCADE,
    CONSTRAINT team_mirrors_mirror_message_id_fkey FOREIGN KEY (mirror_message_id) REFERENCES messages (id) ON DELETE CASCADE,
    CONSTRAINT team_mirrors_member_id_fkey FOREIGN KEY (member_id) REFERENCES team_members (id) ON DELETE SET NULL,
    CONSTRAINT team_mirrors_mirror_message_id_key UNIQUE (mirror_message_id)
);

-- The foreign keys that point at team_members and messages need an index on the referencing side
-- for the cascades (migrations/README.md); the unique keys above cover channel_id and work_thread_id.
CREATE INDEX team_members_agent_id_idx ON team_members (agent_id);
CREATE INDEX team_members_home_channel_id_idx ON team_members (home_channel_id);
CREATE INDEX team_inputs_member_id_idx ON team_inputs (member_id);
CREATE INDEX team_inputs_source_member_id_idx ON team_inputs (source_member_id);
CREATE INDEX team_mirrors_member_id_idx ON team_mirrors (member_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON team_channels TO bothy_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON team_members TO bothy_app;
GRANT SELECT, INSERT ON team_inputs TO bothy_app;
GRANT SELECT, INSERT ON team_mirrors TO bothy_app;

-- +goose StatementBegin
DO $$
DECLARE
    con text;
BEGIN
    SELECT conname INTO con FROM pg_constraint
    WHERE conrelid = 'runs'::regclass AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%triggering_message_id IS NULL%';
    IF con IS NULL THEN
        RAISE EXCEPTION 'the check of the triggering columns of runs is missing';
    END IF;
    EXECUTE format('ALTER TABLE runs DROP CONSTRAINT %I', con);
END
$$;
-- +goose StatementEnd
ALTER TABLE runs ADD CONSTRAINT runs_triggering_check
    CHECK ((triggering_message_id IS NULL OR trigger_kind IN ('message', 'team'))
        AND (triggering_command_id IS NULL OR trigger_kind = 'command')
        AND (triggering_interaction_id IS NULL OR trigger_kind = 'interaction')) NOT VALID;
ALTER TABLE runs DROP CONSTRAINT runs_trigger_kind_check;
ALTER TABLE runs ADD CONSTRAINT runs_trigger_kind_check
    CHECK (trigger_kind IN ('message', 'command', 'interaction', 'wake', 'team')) NOT VALID;

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    found bigint;
BEGIN
    SELECT count(*) INTO found FROM runs WHERE trigger_kind = 'team';
    IF found > 0 THEN
        RAISE EXCEPTION 'runs with trigger_kind team exist: %', found
            USING HINT = 'stop and end the runs of team channels before the rollback';
    END IF;
END
$$;
-- +goose StatementEnd
ALTER TABLE runs DROP CONSTRAINT runs_trigger_kind_check;
ALTER TABLE runs ADD CONSTRAINT runs_trigger_kind_check CHECK (trigger_kind IN ('message', 'command', 'interaction', 'wake'));
ALTER TABLE runs DROP CONSTRAINT runs_triggering_check;
ALTER TABLE runs ADD CONSTRAINT runs_check1
    CHECK ((triggering_message_id IS NULL OR trigger_kind = 'message')
        AND (triggering_command_id IS NULL OR trigger_kind = 'command')
        AND (triggering_interaction_id IS NULL OR trigger_kind = 'interaction'));
DROP TABLE team_mirrors;
DROP TABLE team_inputs;
DROP TABLE team_members;
DROP TABLE team_channels;
