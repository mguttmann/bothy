-- SPDX-License-Identifier: Apache-2.0
-- Installations of the apps module (M2-24): app_installations exactly as
-- docs/spec/03-data-model.md section 4.4 states it. An installation holds the granted scopes and
-- the event subscriptions the owner consented to; app.suspend sets suspended, app.resume active
-- again, app.uninstall uninstalled (docs/spec/11-extensions.md section 4.3). Retention class:
-- config (03 section 3).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update, not delete.
-- An uninstalled installation stays as history; a reinstall adds a new row.

-- +goose Up
CREATE TABLE app_installations (
    id                  uuid        NOT NULL,
    app_id              uuid        NOT NULL,
    granted_scopes      text[]      NOT NULL DEFAULT '{}',
    event_subscriptions text[]      NOT NULL DEFAULT '{}',
    state               text        NOT NULL DEFAULT 'active'
                                    CHECK (state IN ('active', 'suspended', 'uninstalled')),
    installed_by        uuid        NOT NULL,
    installed_at        timestamptz NOT NULL DEFAULT now(),
    uninstalled_at      timestamptz,
    uninstalled_by      uuid,
    PRIMARY KEY (id),
    FOREIGN KEY (app_id) REFERENCES apps (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (installed_by) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (uninstalled_by) REFERENCES principals (id) ON DELETE RESTRICT
);
-- At most one installation per app that is not uninstalled (03 app_installations); the index
-- also serves the lookup of the live installation of an app.
CREATE UNIQUE INDEX app_installations_app_live_key ON app_installations (app_id) WHERE state <> 'uninstalled';

GRANT SELECT, INSERT, UPDATE ON app_installations TO bothy_app;

-- +goose Down
DROP TABLE app_installations;
