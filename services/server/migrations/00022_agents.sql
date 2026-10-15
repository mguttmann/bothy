-- SPDX-License-Identifier: Apache-2.0
-- Operating record of agent principals (M1-51): agents exactly as docs/spec/03-data-model.md
-- section 4.4 states it, plus the trigger that requires principals.kind = 'agent'. IDs come from
-- kernel/ids (R-DM-01); enumerations are text with CHECK (R-DM-02). Retention class: account
-- (03 section 3). operator_config holds agent_operator_config.v1 and reported_details
-- agent_heartbeat.v1, never secrets (B10, I5); the agents module validates both.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update, not delete.
-- An agent principal is never hard deleted (03 principle 7), and its record lives with it.

-- +goose Up
CREATE TABLE agents (
    principal_id             uuid        NOT NULL,
    adapter_kind             text        NOT NULL,
    description              text        NOT NULL DEFAULT '',
    operator_config          jsonb       NOT NULL DEFAULT '{}',
    billing_mode             text        NOT NULL DEFAULT 'organization'
                                         CHECK (billing_mode IN ('personal', 'organization')),
    credential_holder_id     uuid,
    billing_mode_changed_at  timestamptz,
    billing_mode_changed_by  uuid,
    tool_class               text        NOT NULL DEFAULT 'chat'
                                         CHECK (tool_class IN ('chat', 'scoped', 'host')),
    reported_tool_class      text        CHECK (reported_tool_class IN ('chat', 'scoped', 'host')),
    reported_runtime_state   text        NOT NULL DEFAULT 'unknown'
                                         CHECK (reported_runtime_state IN ('unknown', 'online', 'degraded', 'offline')),
    reported_billing_mode    text        CHECK (reported_billing_mode IN ('personal', 'organization')),
    reported_runtime_version text,
    reported_details         jsonb       NOT NULL DEFAULT '{}',
    last_heartbeat_at        timestamptz,
    version                  bigint      NOT NULL DEFAULT 1,
    PRIMARY KEY (principal_id),
    FOREIGN KEY (principal_id) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (credential_holder_id) REFERENCES users (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (billing_mode_changed_by) REFERENCES principals (id) ON DELETE RESTRICT,
    -- A holder exactly for personal (03 agents, I1).
    CONSTRAINT agents_credential_holder_check CHECK ((billing_mode = 'personal') = (credential_holder_id IS NOT NULL))
);
-- The I1 check after a change of a person reads the personal agents of a holder.
CREATE INDEX agents_credential_holder_personal_idx ON agents (credential_holder_id) WHERE billing_mode = 'personal';
-- The presence evaluation reads by the last heartbeat (M2-20).
CREATE INDEX agents_last_heartbeat_at_idx ON agents (last_heartbeat_at);

-- An agents row belongs to a principal of kind agent (03 agents: checked by a trigger). The kind
-- of a principal never changes (identity_principals_kind_immutable), so a check at insert and at
-- a change of principal_id suffices. SQLSTATE 23514 (check_violation) lets the adapter map it
-- like a check.
-- +goose StatementBegin
CREATE FUNCTION agents_kind_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM public.principals WHERE id = NEW.principal_id AND kind = 'agent') THEN
        RAISE EXCEPTION 'agents row for a principal that is not of kind agent'
            USING ERRCODE = 'check_violation', TABLE = 'agents', COLUMN = 'principal_id';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER agents_kind_check
    BEFORE INSERT OR UPDATE OF principal_id ON agents
    FOR EACH ROW EXECUTE FUNCTION agents_kind_check();

-- Trigger functions fire without EXECUTE; nobody needs to call them directly.
REVOKE ALL ON FUNCTION agents_kind_check() FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE ON agents TO bothy_app;

-- +goose Down
DROP TABLE agents;
DROP FUNCTION agents_kind_check();
