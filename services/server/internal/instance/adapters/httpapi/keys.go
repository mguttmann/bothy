// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the instance module (adapters/httpapi, 02 "5.
// Schichten im Modul"): getJwks, getWebhookKeys and rotateSigningKey of the generated strict server
// over the port instance.Keys (M1-32, M2-26).
//
// Example:
//
//	h := httpapi.NewKeys(keys, cfg.JWKSMaxAge) // embedded into wire.Handlers
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
)

// jwkSetContentType is the media type of a JWK set (RFC 7517 section 8.5).
const jwkSetContentType = "application/jwk-set+json"

// Keys implements getJwks, getWebhookKeys and rotateSigningKey of rest.StrictServerInterface over
// instance.Keys.
// The use cases authorize; the handlers only convert. Errors go to the problem writer of the chain
// unchanged.
type Keys struct {
	keys   instance.Keys
	maxAge time.Duration
}

// NewKeys returns the handlers over the key port; maxAge is agent.jwks_max_age, the max-age of the
// JWKS response.
func NewKeys(keys instance.Keys, maxAge time.Duration) *Keys {
	return &Keys{keys: keys, maxAge: maxAge}
}

// GetJwks implements rest.StrictServerInterface (GET /instance/jwks): the published key set with
// Cache-Control: public, max-age=<agent.jwks_max_age in whole seconds>. The route is public
// (security: []); the CORS stage answers its action instance.discovery.read with
// Access-Control-Allow-Origin: * and no credentials (SEC-CORS-2).
func (h *Keys) GetJwks(ctx context.Context, _ rest.GetJwksRequestObject) (rest.GetJwksResponseObject, error) {
	set, err := h.keys.JWKS(ctx)
	if err != nil {
		return nil, err
	}
	return jwksResponse{set: set, maxAge: int64(h.maxAge / time.Second)}, nil
}

// jwksResponse writes the JWK set with its caching header; the generated response type has no
// field for Cache-Control.
type jwksResponse struct {
	set    instance.JWKSet
	maxAge int64
}

// VisitGetJwksResponse writes 200 with the JWK set.
func (r jwksResponse) VisitGetJwksResponse(w http.ResponseWriter) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r.set); err != nil {
		return err
	}
	w.Header().Set("Content-Type", jwkSetContentType)
	w.Header().Set("Cache-Control", "public, max-age="+strconv.FormatInt(r.maxAge, 10))
	w.WriteHeader(http.StatusOK)
	_, err := buf.WriteTo(w)
	return err
}

// GetWebhookKeys implements rest.StrictServerInterface (GET /instance/webhook-keys, M2-26): the
// published webhook_out keys (states next, active and retiring) in the form whpk_ (D-M2-26-11),
// ordered by state and kid, not_after only when set, with the Cache-Control of getJwks. The route
// is public (security: []); the CORS stage answers its action instance.discovery.read with
// Access-Control-Allow-Origin: * (SEC-CORS-2).
func (h *Keys) GetWebhookKeys(ctx context.Context, _ rest.GetWebhookKeysRequestObject) (rest.GetWebhookKeysResponseObject, error) {
	keys, err := h.keys.PublicKeys(ctx, instance.PurposeWebhookOut)
	if err != nil {
		return nil, err
	}
	body := rest.WebhookKeys{Keys: make([]rest.WebhookKey, 0, len(keys))}
	for _, k := range keys {
		item := rest.WebhookKey{Kid: k.KID, PublicKey: instance.FormatWebhookPublicKey(k.Key), State: rest.WebhookKeyState(k.State)}
		if !k.NotAfter.IsZero() {
			at := k.NotAfter.UTC().Truncate(time.Millisecond)
			item.NotAfter = &at
		}
		body.Keys = append(body.Keys, item)
	}
	return webhookKeysResponse{body: body, maxAge: int64(h.maxAge / time.Second)}, nil
}

// webhookKeysResponse writes the webhook keys with the caching header of the JWKS.
type webhookKeysResponse struct {
	body   rest.WebhookKeys
	maxAge int64
}

// VisitGetWebhookKeysResponse writes 200 with the keys.
func (r webhookKeysResponse) VisitGetWebhookKeysResponse(w http.ResponseWriter) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r.body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age="+strconv.FormatInt(r.maxAge, 10))
	w.WriteHeader(http.StatusOK)
	_, err := buf.WriteTo(w)
	return err
}

// RotateSigningKey implements rest.StrictServerInterface (POST /signing-keys/rotate).
func (h *Keys) RotateSigningKey(ctx context.Context, req rest.RotateSigningKeyRequestObject) (rest.RotateSigningKeyResponseObject, error) {
	var cmd instance.RotateCommand
	if req.Body != nil {
		cmd = instance.RotateCommand{
			Purpose: managedPurpose(string(req.Body.Purpose)), Mode: instance.RotateMode(req.Body.Mode), Reason: req.Body.Reason,
		}
	}
	r, err := h.keys.Rotate(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.RotateSigningKey200JSONResponse{Body: rest.SigningKeyRotation{
		Purpose: rest.SigningKeyPurpose(r.Purpose), NewKid: r.NewKID, PreviousKid: r.PreviousKID,
		PreviousNotAfter: r.PreviousNotAfter.UTC().Truncate(time.Millisecond),
	}}, nil
}

// principal returns the actor of the request; outside the chain it is the anonymous principal,
// which authorize refuses.
func principal(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// managedPurpose returns the managed purpose constant equal to s, or the empty purpose, which
// checkRotate refuses as not_supported. The request value itself never reaches the key store paths
// or the log lines of the use case, only a constant of instance.Purposes.
func managedPurpose(s string) instance.Purpose {
	for _, p := range instance.Purposes() {
		if string(p) == s {
			return p
		}
	}
	return ""
}
