// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Providers implements the provider operations of rest.StrictServerInterface (M1-36):
// listIdentityProviders, createIdentityProvider, getIdentityProvider, updateIdentityProvider and
// deleteIdentityProvider over identity.Providers. The use cases authorize; the handlers convert.
// Errors go to the problem writer of the chain unchanged.
type Providers struct {
	providers identity.Providers
}

// NewProviders returns the handlers over the provider port.
func NewProviders(p identity.Providers) *Providers { return &Providers{providers: p} }

// ListIdentityProviders implements rest.StrictServerInterface (GET /identity-providers).
func (h *Providers) ListIdentityProviders(ctx context.Context, _ rest.ListIdentityProvidersRequestObject) (rest.ListIdentityProvidersResponseObject, error) {
	list, err := h.providers.List(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	out := rest.IdentityProviderList{Items: make([]rest.IdentityProvider, 0, len(list))}
	for _, p := range list {
		item, err := toRESTProvider(p)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, item)
	}
	return rest.ListIdentityProviders200JSONResponse{Body: out}, nil
}

// CreateIdentityProvider implements rest.StrictServerInterface (POST /identity-providers); the
// Idempotency-Key stage of the chain replays a stored 201.
func (h *Providers) CreateIdentityProvider(ctx context.Context, req rest.CreateIdentityProviderRequestObject) (rest.CreateIdentityProviderResponseObject, error) {
	var in identity.CreateProvider
	if b := req.Body; b != nil {
		cfg, err := configOf(b.Config)
		if err != nil {
			return nil, err
		}
		in = identity.CreateProvider{Kind: identity.ProviderKind(b.Kind), Name: b.Name, Config: cfg, Enabled: b.Enabled, AllowProvisioning: b.AllowProvisioning}
	}
	p, err := h.providers.Create(ctx, principal(ctx), in)
	if err != nil {
		return nil, err
	}
	out, err := toRESTProvider(p)
	if err != nil {
		return nil, err
	}
	return rest.CreateIdentityProvider201JSONResponse{Body: out}, nil
}

// GetIdentityProvider implements rest.StrictServerInterface (GET /identity-providers/{provider_id}).
func (h *Providers) GetIdentityProvider(ctx context.Context, req rest.GetIdentityProviderRequestObject) (rest.GetIdentityProviderResponseObject, error) {
	p, err := h.providers.Get(ctx, principal(ctx), ids.ID(req.ProviderId))
	if err != nil {
		return nil, err
	}
	out, err := toRESTProvider(p)
	if err != nil {
		return nil, err
	}
	return rest.GetIdentityProvider200JSONResponse{Body: out}, nil
}

// UpdateIdentityProvider implements rest.StrictServerInterface (PATCH
// /identity-providers/{provider_id}); absent members stay unchanged.
func (h *Providers) UpdateIdentityProvider(ctx context.Context, req rest.UpdateIdentityProviderRequestObject) (rest.UpdateIdentityProviderResponseObject, error) {
	var in identity.UpdateProvider
	if b := req.Body; b != nil {
		in = identity.UpdateProvider{Name: b.Name, Enabled: b.Enabled, AllowProvisioning: b.AllowProvisioning}
		if b.Config != nil {
			cfg, err := configOf(*b.Config)
			if err != nil {
				return nil, err
			}
			in.Config = &cfg
		}
	}
	p, err := h.providers.Update(ctx, principal(ctx), ids.ID(req.ProviderId), in)
	if err != nil {
		return nil, err
	}
	out, err := toRESTProvider(p)
	if err != nil {
		return nil, err
	}
	return rest.UpdateIdentityProvider200JSONResponse{Body: out}, nil
}

// DeleteIdentityProvider implements rest.StrictServerInterface (DELETE
// /identity-providers/{provider_id}).
func (h *Providers) DeleteIdentityProvider(ctx context.Context, req rest.DeleteIdentityProviderRequestObject) (rest.DeleteIdentityProviderResponseObject, error) {
	if err := h.providers.Delete(ctx, principal(ctx), ids.ID(req.ProviderId)); err != nil {
		return nil, err
	}
	return rest.DeleteIdentityProvider204Response{}, nil
}

// configOf encodes the decoded config member again for the strict validation of the domain. The
// JSON guard of the chain has rejected duplicate members already; a JSON null inside survives the
// decoding as nil and is rejected by the schema.
func configOf(m rest.JsonObject) (json.RawMessage, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "identity: provider configuration")
	}
	return b, nil
}

// toRESTProvider converts a provider (schema IdentityProvider); config is the stored normalized
// configuration with secret references only.
func toRESTProvider(p identity.Provider) (rest.IdentityProvider, error) {
	cfg := rest.JsonObject{}
	if len(p.Config) > 0 {
		if err := json.Unmarshal(p.Config, &cfg); err != nil {
			return rest.IdentityProvider{}, errs.Wrap(errs.CodeInternalError, err, "identity: stored provider configuration")
		}
	}
	return rest.IdentityProvider{
		Id: rest.Uuid(p.ID), Kind: rest.IdentityProviderKind(p.Kind), Name: p.Name, Config: cfg, Enabled: p.Enabled,
		AllowProvisioning: p.AllowProvisioning, UpdatedAt: optionalTime(p.UpdatedAt),
	}, nil
}
