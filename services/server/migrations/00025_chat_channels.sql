-- SPDX-License-Identifier: Apache-2.0
-- Channels, their skill list, forum tags and threads (M1-53): channels, channel_skill_bindings,
-- forum_tags and threads as docs/spec/03-data-model.md section 4.5 states them. IDs come from
-- kernel/ids (R-DM-01); enumerations are text with CHECK (R-DM-02); sort keys are fractional
-- index keys compared bytewise (COLLATE "C", 03 principle 5). Retention classes: content
-- (channels, forum_tags, threads) and config (channel_skill_bindings).
--
-- Two deliberate deviations from 03 threads, both added by later tickets:
--   - threads.search_language, threads.title_search and its GIN index are not created here;
--     M3-13 adds them with the search (plan row M3-13).
--   - the foreign key threads.parent_message_id -> messages(id) is not created here, because
--     messages does not exist yet; M1-56 adds it DEFERRABLE INITIALLY DEFERRED.
-- Every other column, default, CHECK, unique and partial index of the four tables is created as
-- 03 states it, including channels.read_only, the behaviour columns and settings_version (their
-- operations come with M2-02 and M8-03), so no later migration rewrites the table.
-- history_backfill_limit has no default: the application sets channel.history_backfill_limit_default
-- and checks channel.history_backfill_limit_max (03, "by the application").
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update channels,
-- forum_tags and threads, never delete them (03 principle 7: archived_at and deleted_at, never a
-- hard delete); channel_skill_bindings also allows delete, because M2-02 replaces the ordered list.
--
-- grants.target_channel_id gets its foreign key here, because channels did not exist when M1-27
-- created grants (00005). It starts NOT VALID (upgrade.md rule M5) and 00026 validates it.

-- +goose Up
CREATE TABLE channels (
    id                         uuid        NOT NULL,
    space_id                   uuid,
    kind                       text        NOT NULL,
    name                       text        NOT NULL,
    topic                      text        NOT NULL DEFAULT '',
    sort_key                   text        COLLATE "C" NOT NULL,
    operator_inherit           boolean     NOT NULL DEFAULT true,
    operator_agent_id          uuid,
    require_mention            boolean     NOT NULL DEFAULT true,
    thread_require_mention     boolean     NOT NULL DEFAULT false,
    free_response              boolean     NOT NULL DEFAULT false,
    auto_thread                boolean     NOT NULL DEFAULT true,
    allow_mentioned_agents     boolean     NOT NULL DEFAULT false,
    history_backfill_limit     integer     NOT NULL,
    prompt                     text        NOT NULL DEFAULT '',
    forum_default_sort         text,
    default_notification_level text,
    read_only                  boolean     NOT NULL DEFAULT false,
    created_by                 uuid        NOT NULL,
    created_at                 timestamptz NOT NULL DEFAULT now(),
    archived_at                timestamptz,
    version                    bigint      NOT NULL DEFAULT 1,
    settings_version           bigint      NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    FOREIGN KEY (space_id) REFERENCES spaces (id) ON DELETE RESTRICT,
    FOREIGN KEY (operator_agent_id) REFERENCES agents (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT channels_kind_check CHECK (kind IN ('text', 'forum', 'dm')),
    CONSTRAINT channels_forum_default_sort_check CHECK (forum_default_sort IN ('last_activity', 'created')),
    CONSTRAINT channels_default_notification_level_check CHECK (default_notification_level IN ('all', 'mentions', 'mute')),
    -- A dm channel and only a dm channel has no space (R-DM-10).
    CONSTRAINT channels_dm_space_check CHECK ((kind = 'dm') = (space_id IS NULL)),
    -- An inheriting channel names no own operator.
    CONSTRAINT channels_operator_check CHECK (NOT operator_inherit OR operator_agent_id IS NULL),
    -- A dm channel has no operator and is never read-only.
    CONSTRAINT channels_dm_operator_check CHECK (kind <> 'dm' OR (NOT operator_inherit AND operator_agent_id IS NULL AND NOT read_only))
);
-- A name is unique case-insensitively per space among the channels that are not archived.
CREATE UNIQUE INDEX channels_space_name_active_key ON channels (space_id, lower(name)) WHERE archived_at IS NULL AND kind <> 'dm';
-- The sidebar reads the channels of a space that are not archived in their order.
CREATE INDEX channels_space_sort_key_idx ON channels (space_id, sort_key) WHERE archived_at IS NULL;
-- "Which channels does this agent operate" for the I1 check (03 section 6.4).
CREATE INDEX channels_operator_agent_id_idx ON channels (operator_agent_id) WHERE operator_agent_id IS NOT NULL;

CREATE TABLE channel_skill_bindings (
    channel_id uuid    NOT NULL,
    position   integer NOT NULL,
    skill_ref  text    NOT NULL,
    PRIMARY KEY (channel_id, position),
    FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE CASCADE,
    UNIQUE (channel_id, skill_ref)
);

CREATE TABLE forum_tags (
    id          uuid        NOT NULL,
    channel_id  uuid        NOT NULL,
    name        text        NOT NULL,
    color       text,
    sort_key    text        COLLATE "C" NOT NULL,
    restricted  boolean     NOT NULL DEFAULT false,
    deleted_at  timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE CASCADE
);
-- A tag name is unique case-insensitively per forum among the tags that are not deleted.
CREATE UNIQUE INDEX forum_tags_channel_name_active_key ON forum_tags (channel_id, lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE threads (
    id                uuid        NOT NULL,
    channel_id        uuid        NOT NULL,
    kind              text        NOT NULL,
    -- No foreign key to messages yet: M1-56 adds it DEFERRABLE INITIALLY DEFERRED (see above).
    parent_message_id uuid,
    title             text        NOT NULL DEFAULT '',
    title_source      text        NOT NULL DEFAULT 'auto',
    state             text        NOT NULL DEFAULT 'open',
    closed_at         timestamptz,
    closed_by         uuid,
    operator_agent_id uuid,
    last_message_seq  bigint      NOT NULL DEFAULT 0,
    unread_floor_seq  bigint      NOT NULL DEFAULT 0,
    last_activity_at  timestamptz NOT NULL DEFAULT now(),
    message_count     integer     NOT NULL DEFAULT 0,
    created_by        uuid        NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    moved_at          timestamptz,
    version           bigint      NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE RESTRICT,
    FOREIGN KEY (operator_agent_id) REFERENCES agents (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (closed_by) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT threads_kind_check CHECK (kind IN ('main', 'thread')),
    CONSTRAINT threads_title_source_check CHECK (title_source IN ('auto', 'manual')),
    CONSTRAINT threads_state_check CHECK (state IN ('open', 'closed')),
    CONSTRAINT threads_closed_check CHECK ((state = 'closed') = (closed_at IS NOT NULL)),
    -- The main thread has neither a parent message nor an own operator (R-DM-11, R-DM-21).
    CONSTRAINT threads_main_check CHECK (kind <> 'main' OR (parent_message_id IS NULL AND operator_agent_id IS NULL))
);
-- Exactly one main thread per text and dm channel (R-DM-11).
CREATE UNIQUE INDEX threads_channel_main_key ON threads (channel_id) WHERE kind = 'main';
-- A message starts at most one thread.
CREATE UNIQUE INDEX threads_parent_message_id_key ON threads (parent_message_id) WHERE parent_message_id IS NOT NULL;
-- The post list with its done section.
CREATE INDEX threads_channel_state_activity_idx ON threads (channel_id, state, last_activity_at DESC);
-- The I1 targets, without a condition on state (03 section 6.4).
CREATE INDEX threads_operator_agent_channel_idx ON threads (operator_agent_id, channel_id) WHERE operator_agent_id IS NOT NULL;

ALTER TABLE grants ADD CONSTRAINT grants_target_channel_id_fkey
    FOREIGN KEY (target_channel_id) REFERENCES channels (id) ON DELETE RESTRICT NOT VALID;

GRANT SELECT, INSERT, UPDATE ON channels TO bothy_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON channel_skill_bindings TO bothy_app;
GRANT SELECT, INSERT, UPDATE ON forum_tags TO bothy_app;
GRANT SELECT, INSERT, UPDATE ON threads TO bothy_app;

-- +goose Down
ALTER TABLE grants DROP CONSTRAINT grants_target_channel_id_fkey;
DROP TABLE threads;
DROP TABLE forum_tags;
DROP TABLE channel_skill_bindings;
DROP TABLE channels;
