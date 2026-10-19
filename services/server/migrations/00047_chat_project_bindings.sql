-- SPDX-License-Identifier: Apache-2.0
-- Project bindings of forums (M2-03): project_bindings exactly as docs/spec/03-datenmodell.md
-- section 4.5 states it. One row per bound forum (PK channel_id); workspace_ref is an opaque
-- reference the runner resolves and is unique case-insensitively; repository_url is display only;
-- runner_config follows the schema project_runner_config.v1 (09 section 18.1) and holds no
-- secrets; ready is written only from the heartbeat of the serving runner and by the resets of
-- M2-03. Retention class: config (03 section 3).
--
-- The rule "only kind = 'forum'" spans two tables and cannot be a CHECK; the use cases of
-- channel.project.bind enforce it (internal/chat/app/project_binding.go). The row ends with its
-- channel through ON DELETE CASCADE; created_by keeps its principal (ON DELETE RESTRICT).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert, update and delete,
-- because deleteProjectBinding removes the row (a binding has no tombstone, 06 section 7).

-- +goose Up
CREATE TABLE project_bindings (
    channel_id     uuid        NOT NULL,
    workspace_ref  text        NOT NULL,
    repository_url text,
    default_branch text,
    runner_config  jsonb       NOT NULL DEFAULT '{}',
    ready          boolean     NOT NULL DEFAULT false,
    created_by     uuid        NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    version        bigint      NOT NULL DEFAULT 1,
    PRIMARY KEY (channel_id),
    FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX project_bindings_workspace_ref_key ON project_bindings (lower(workspace_ref));

GRANT SELECT, INSERT, UPDATE, DELETE ON project_bindings TO bothy_app;

-- +goose Down
DROP TABLE project_bindings;
