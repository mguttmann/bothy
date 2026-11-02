-- SPDX-License-Identifier: Apache-2.0
-- Terminal in the owner's own account (FEAT-terminal-admin-user, ADR 0073): an assignment of a
-- session user records the explicit opt-in for the owner account of the host. A constant default
-- adds the column without rewriting the table (upgrade.md, Expand); every existing row is an
-- assignment of a separate session user.

-- +goose Up
ALTER TABLE host_session_users ADD COLUMN owner_account boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE host_session_users DROP COLUMN owner_account;
