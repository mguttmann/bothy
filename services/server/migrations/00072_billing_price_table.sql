-- SPDX-License-Identifier: Apache-2.0
-- Price table of the billing module (M2-22): price_table exactly as docs/spec/03-data-model.md
-- section 4.9 states it, every column with its type, default and nullability. A row is the price
-- of one unit of one provider and model for its validity [valid_from, valid_to); an absent
-- valid_to is open. The exclusion constraint over tstzrange keeps the validities of one provider,
-- model and unit apart (extension btree_gist, which PostgreSQL marks trusted, so the database
-- owner that runs the migrations creates it without a superuser). Retention class: cost.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read and insert rows and update only
-- valid_to. A row used by a booking never changes again except its end (R-DM-23), and the column
-- grant makes that hold in the database (ruling D-M2-22-1). No delete: retention is M7-05's.

-- +goose Up
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE price_table (
    id           uuid        NOT NULL,
    provider     text        NOT NULL,
    model        text        NOT NULL,
    unit         text        NOT NULL,
    price_micros bigint      NOT NULL,
    currency     text        NOT NULL DEFAULT 'USD',
    valid_from   timestamptz NOT NULL,
    valid_to     timestamptz,
    source_url   text        NOT NULL,
    retrieved_at timestamptz NOT NULL,
    created_by   uuid        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT price_table_pkey PRIMARY KEY (id),
    CONSTRAINT price_table_created_by_fkey FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT price_table_unit_check CHECK (unit IN ('input_mtok', 'output_mtok', 'cache_read_mtok', 'cache_write_mtok',
        'cache_write_1h_mtok', 'request')),
    CONSTRAINT price_table_provider_model_unit_valid_from_key UNIQUE (provider, model, unit, valid_from),
    CONSTRAINT price_table_no_overlap EXCLUDE USING gist (provider WITH =, model WITH =, unit WITH =,
        tstzrange(valid_from, valid_to) WITH &&)
);

-- The price at time t: the rows of one provider, model and unit, newest validity first.
CREATE INDEX price_table_lookup_idx ON price_table (provider, model, unit, valid_from DESC);

GRANT SELECT, INSERT ON price_table TO bothy_app;
GRANT UPDATE (valid_to) ON price_table TO bothy_app;

-- +goose Down
DROP TABLE price_table;
