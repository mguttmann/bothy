-- SPDX-License-Identifier: Apache-2.0
-- Quiet hours (M3-18): quiet_hours as docs/spec/03-datenmodell.md section 4.10 states it, without
-- deviation. Only the module notify writes it; every query names the person (user_id) or the
-- explicit list of people. Retention class: personal_sync (the rules reveal working hours).

-- +goose Up
CREATE TABLE quiet_hours (
    id          uuid     NOT NULL,
    user_id     uuid     NOT NULL,
    -- Bit mask, bit 0 = Monday.
    weekdays    smallint NOT NULL CHECK (weekdays BETWEEN 1 AND 127),
    start_local time     NOT NULL,
    -- end_local <= start_local crosses midnight.
    end_local   time     NOT NULL,
    -- NULL means users.time_zone.
    time_zone   text,
    enabled     boolean  NOT NULL DEFAULT true,
    version     bigint   NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE CASCADE
);
CREATE INDEX quiet_hours_user_enabled_idx ON quiet_hours (user_id) WHERE enabled;

GRANT SELECT, INSERT, UPDATE, DELETE ON quiet_hours TO bothy_app;

-- +goose Down
DROP TABLE quiet_hours;
