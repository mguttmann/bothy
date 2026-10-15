-- SPDX-License-Identifier: Apache-2.0
-- Message templates (M1-60): message_templates as docs/spec/03-data-model.md section 4.6 states
-- it, without deviation. A template is personal (owner_user_id) or belongs to a channel
-- (channel_id), never both and never neither. Name and content rules and the bound
-- template.max_per_scope are checked in the application (messaging). Retention class: content.

-- +goose Up
CREATE TABLE message_templates (
    id            uuid        NOT NULL,
    owner_user_id uuid,
    channel_id    uuid,
    name          text        NOT NULL,
    content_md    text        NOT NULL,
    created_by    uuid        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz,
    version       bigint      NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    FOREIGN KEY (owner_user_id) REFERENCES users (principal_id) ON DELETE CASCADE,
    FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT,
    -- Exactly one scope: personal or channel.
    CONSTRAINT message_templates_scope_check CHECK ((owner_user_id IS NULL) <> (channel_id IS NULL))
);
-- The personal templates of a person.
CREATE INDEX message_templates_owner_user_id_idx ON message_templates (owner_user_id) WHERE deleted_at IS NULL;
-- The templates of a channel.
CREATE INDEX message_templates_channel_id_idx ON message_templates (channel_id) WHERE deleted_at IS NULL;

GRANT SELECT, INSERT, UPDATE ON message_templates TO bothy_app;

-- +goose Down
DROP TABLE message_templates;
