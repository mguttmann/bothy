-- SPDX-License-Identifier: Apache-2.0
-- bothy_app reads the schema version at start (pg.CheckSchema) and has no other right on the
-- version table of goose (migrations/README.md, grant rule).

-- +goose Up
GRANT SELECT ON goose_db_version TO bothy_app;

-- +goose Down
REVOKE SELECT ON goose_db_version FROM bothy_app;
