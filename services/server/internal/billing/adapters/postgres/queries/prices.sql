-- SPDX-License-Identifier: Apache-2.0
-- The price table of the billing module (M2-22, 03 section 4.9). price_table is no content table
-- (class cost) and is readable only through price.read and price.manage of an owner, which the
-- use cases authorize before these queries run.

-- name: LockPriceSet :many
-- The price set S(provider, model, at): every row of exactly this provider and model that covers
-- at ([valid_from, valid_to), an absent valid_to is open), locked FOR SHARE in ascending id (03
-- section 6.3). Under READ COMMITTED a booking that waited for a closePrice re-checks the row
-- with its new end and leaves it out (M2-22 design note 12 d).
SELECT * FROM price_table
WHERE provider = sqlc.arg(provider) AND model = sqlc.arg(model)
  AND valid_from <= sqlc.arg(at)::timestamptz
  AND (valid_to IS NULL OR valid_to > sqlc.arg(at)::timestamptz)
ORDER BY id
FOR SHARE;

-- name: ListPrices :many
-- Every row, closed ones included, for listPrices (a small unpaginated list, 06 section 6 point 6).
SELECT * FROM price_table
ORDER BY provider, model, unit, valid_from, id;

-- name: CurrencyMismatch :one
-- A row of provider and model, any unit, whose validity overlaps [valid_from, valid_to) and whose
-- currency differs (409 price_currency_mismatch of createPrice).
SELECT EXISTS (
    SELECT 1 FROM price_table
    WHERE provider = sqlc.arg(provider) AND model = sqlc.arg(model) AND currency <> sqlc.arg(currency)
      AND tstzrange(valid_from, valid_to) && tstzrange(sqlc.arg(valid_from)::timestamptz, sqlc.narg(valid_to)::timestamptz)
)::boolean AS mismatch;

-- name: InsertPrice :exec
INSERT INTO price_table (id, provider, model, unit, price_micros, currency, valid_from, valid_to, source_url, retrieved_at, created_by)
VALUES (sqlc.arg(id), sqlc.arg(provider), sqlc.arg(model), sqlc.arg(unit), sqlc.arg(price_micros), sqlc.arg(currency),
    sqlc.arg(valid_from), sqlc.narg(valid_to), sqlc.arg(source_url), sqlc.arg(retrieved_at), sqlc.arg(created_by));

-- name: LockPrice :one
-- The row of closePrice with its row lock for the rest of the unit of work.
SELECT * FROM price_table WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: SetPriceValidTo :exec
-- The only column bothy_app may update (grant of 00061, R-DM-23).
UPDATE price_table SET valid_to = sqlc.arg(valid_to) WHERE id = sqlc.arg(id);
