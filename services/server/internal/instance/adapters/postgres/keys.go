// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Repository is the key port of app.
var _ app.KeyRepository = (*Repository)(nil)

// publicJWK is the stored form of server_signing_keys.public_jwk: the published JWK of an Ed25519
// key (RFC 7517, RFC 8037) without a private member.
type publicJWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

// LockPurpose takes the advisory lock of the purpose until the commit (app.KeyRepository). It
// needs a unit of work, because a lock outside one would end with its autocommit.
func (r *Repository) LockPurpose(ctx context.Context, p instance.Purpose) error {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	return sqlcgen.New(tx).LockSigningKeyPurpose(ctx, string(p))
}

// ListKeys returns the unretired keys of p (app.KeyRepository).
func (r *Repository) ListKeys(ctx context.Context, p instance.Purpose) ([]app.KeyRow, error) {
	rows, err := r.queries(ctx).ListSigningKeys(ctx, string(p))
	if err != nil {
		return nil, err
	}
	return keyRows(rows)
}

// ActiveKey returns the active key of p (app.KeyRepository).
func (r *Repository) ActiveKey(ctx context.Context, p instance.Purpose) (app.KeyRow, bool, error) {
	return oneKey(r.queries(ctx).ActiveSigningKey(ctx, string(p)))
}

// PublishedKeys returns the next, active and retiring keys of every purpose (app.KeyRepository).
func (r *Repository) PublishedKeys(ctx context.Context) ([]app.KeyRow, error) {
	rows, err := r.queries(ctx).PublishedSigningKeys(ctx)
	if err != nil {
		return nil, err
	}
	return keyRows(rows)
}

// KeyByKID returns one key in any state (app.KeyRepository).
func (r *Repository) KeyByKID(ctx context.Context, kid string) (app.KeyRow, bool, error) {
	return oneKey(r.queries(ctx).SigningKeyByKID(ctx, kid))
}

// InsertKey inserts a key with its public JWK (app.KeyRepository).
func (r *Repository) InsertKey(ctx context.Context, row app.KeyRow) error {
	jwk, err := json.Marshal(publicJWK{
		Kty: "OKP", Crv: "Ed25519", X: base64.RawURLEncoding.EncodeToString(row.Public),
		Kid: row.KID, Alg: row.Alg, Use: "sig",
	})
	if err != nil {
		return err
	}
	return r.queries(ctx).InsertSigningKey(ctx, sqlcgen.InsertSigningKeyParams{
		Kid: row.KID, Purpose: string(row.Purpose), Alg: row.Alg, PublicJwk: jwk, PrivateKeyRef: row.Ref,
		State: string(row.State), CreatedAt: timestamptz(row.CreatedAt),
	})
}

// TransitionKey moves a key between states (app.KeyRepository).
func (r *Repository) TransitionKey(ctx context.Context, kid string, from, to instance.KeyState, notAfter time.Time) (bool, error) {
	n, err := r.queries(ctx).TransitionSigningKey(ctx, sqlcgen.TransitionSigningKeyParams{
		ToState: string(to), NotAfter: timestamptz(notAfter), Kid: kid, FromState: string(from),
	})
	return n == 1, err
}

// PurposesWithoutActive returns the purposes of ps without an active key (app.KeyRepository).
func (r *Repository) PurposesWithoutActive(ctx context.Context, ps []instance.Purpose) ([]instance.Purpose, error) {
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = string(p)
	}
	missing, err := r.queries(ctx).PurposesWithoutActiveKey(ctx, names)
	if err != nil {
		return nil, err
	}
	out := make([]instance.Purpose, len(missing))
	for i, m := range missing {
		out[i] = instance.Purpose(m)
	}
	return out, nil
}

// oneKey converts the result of a single-row query; no row is found false.
func oneKey(row sqlcgen.ServerSigningKey, err error) (app.KeyRow, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.KeyRow{}, false, nil
	}
	if err != nil {
		return app.KeyRow{}, false, err
	}
	k, err := keyRow(row)
	return k, err == nil, err
}

func keyRows(rows []sqlcgen.ServerSigningKey) ([]app.KeyRow, error) {
	out := make([]app.KeyRow, 0, len(rows))
	for _, row := range rows {
		k, err := keyRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, nil
}

// keyRow converts a row; the public key comes from the member x of public_jwk.
func keyRow(row sqlcgen.ServerSigningKey) (app.KeyRow, error) {
	var jwk publicJWK
	if err := json.Unmarshal(row.PublicJwk, &jwk); err != nil {
		return app.KeyRow{}, fmt.Errorf("instance: public_jwk of key %s: %w", row.Kid, err)
	}
	x, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil || len(x) != ed25519.PublicKeySize {
		return app.KeyRow{}, fmt.Errorf("instance: public_jwk of key %s has no Ed25519 x", row.Kid)
	}
	k := app.KeyRow{
		KID: row.Kid, Purpose: instance.Purpose(row.Purpose), Alg: row.Alg, Public: ed25519.PublicKey(x),
		Ref: row.PrivateKeyRef, State: instance.KeyState(row.State), CreatedAt: row.CreatedAt.Time.UTC(),
	}
	if row.NotAfter.Valid {
		k.NotAfter = row.NotAfter.Time.UTC()
	}
	return k, nil
}

// timestamptz converts a time; the zero time is NULL.
func timestamptz(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}
