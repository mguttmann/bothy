// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the module apps (M2-24): the nine app operations of
// rest.StrictServerInterface over the port apps.Apps. The use cases authorize; the handlers
// convert. Errors go to the problem writer of the chain unchanged.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// Apps implements listApps, installApp, validateManifest, getApp, upgradeAppManifest,
// setAppScopes, suspendApp, resumeApp and uninstallApp.
type Apps struct {
	apps apps.Apps
}

// NewApps returns the handlers over the use cases.
func NewApps(a apps.Apps) *Apps { return &Apps{apps: a} }

// Cursors implements the cursor port of the use cases with the codec of the HTTP chain.
type Cursors struct{}

// Encode returns the cursor of key (httpserver.EncodeCursor).
func (Cursors) Encode(list string, binding []byte, key any) (string, error) {
	return httpserver.EncodeCursor(list, binding, key)
}

// Decode checks s and unmarshals its key (httpserver.DecodeCursor, 400 cursor_invalid).
func (Cursors) Decode(list string, binding []byte, s string, key any) error {
	return httpserver.DecodeCursor(list, binding, s, key)
}

// principal returns the actor of the request; anonymous outside the chain.
func principal(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// ListApps implements rest.StrictServerInterface (GET /apps).
func (h *Apps) ListApps(ctx context.Context, req rest.ListAppsRequestObject) (rest.ListAppsResponseObject, error) {
	var cursor string
	var limit int
	if req.Params.Cursor != nil {
		cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	p, err := h.apps.List(ctx, principal(ctx), cursor, limit)
	if err != nil {
		return nil, err
	}
	out := rest.AppPage{Items: make([]rest.App, 0, len(p.Items))}
	for _, a := range p.Items {
		v, err := appREST(a)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, v)
	}
	if p.NextCursor != "" {
		out.NextCursor = &p.NextCursor
	}
	return rest.ListApps200JSONResponse{Body: out}, nil
}

// InstallApp implements rest.StrictServerInterface (POST /apps).
func (h *Apps) InstallApp(ctx context.Context, req rest.InstallAppRequestObject) (rest.InstallAppResponseObject, error) {
	cmd, err := installCmd(ctx, req.Body)
	if err != nil {
		return nil, err
	}
	a, err := h.apps.Install(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	v, err := appREST(a)
	if err != nil {
		return nil, err
	}
	return rest.InstallApp201JSONResponse{Body: v}, nil
}

// ValidateManifest implements rest.StrictServerInterface (POST /app-manifests/validate): the
// body as received is the manifest.
func (h *Apps) ValidateManifest(ctx context.Context, req rest.ValidateManifestRequestObject) (rest.ValidateManifestResponseObject, error) {
	raw, ok := httpserver.RawJSONBody(ctx)
	if !ok {
		var err error
		if raw, err = json.Marshal(req.Body); err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, err, "apps: manifest")
		}
	}
	res, err := h.apps.Validate(ctx, principal(ctx), raw)
	if err != nil {
		return nil, err
	}
	out := rest.ManifestValidation{Valid: res.Valid, Errors: make([]rest.FieldError, 0, len(res.Errors))}
	for _, f := range res.Errors {
		detail := f.Detail
		out.Errors = append(out.Errors, rest.FieldError{Pointer: f.Pointer, Code: f.Code, Detail: &detail})
	}
	return rest.ValidateManifest200JSONResponse{Body: out}, nil
}

// GetApp implements rest.StrictServerInterface (GET /apps/{app_id}).
func (h *Apps) GetApp(ctx context.Context, req rest.GetAppRequestObject) (rest.GetAppResponseObject, error) {
	a, err := h.apps.Get(ctx, principal(ctx), ids.ID(req.AppId))
	if err != nil {
		return nil, err
	}
	v, err := appREST(a)
	if err != nil {
		return nil, err
	}
	return rest.GetApp200JSONResponse{Body: v}, nil
}

// UpgradeAppManifest implements rest.StrictServerInterface (PUT /apps/{app_id}/manifest).
func (h *Apps) UpgradeAppManifest(ctx context.Context, req rest.UpgradeAppManifestRequestObject) (rest.UpgradeAppManifestResponseObject, error) {
	cmd, err := installCmd(ctx, req.Body)
	if err != nil {
		return nil, err
	}
	a, err := h.apps.Upgrade(ctx, principal(ctx), ids.ID(req.AppId), cmd)
	if err != nil {
		return nil, err
	}
	v, err := appREST(a)
	if err != nil {
		return nil, err
	}
	return rest.UpgradeAppManifest200JSONResponse{Body: v}, nil
}

// SetAppScopes implements rest.StrictServerInterface (PUT /apps/{app_id}/scopes).
func (h *Apps) SetAppScopes(ctx context.Context, req rest.SetAppScopesRequestObject) (rest.SetAppScopesResponseObject, error) {
	var cmd apps.SetScopesCmd
	if b := req.Body; b != nil {
		cmd.GrantedScopes, cmd.Reason = scopes(b.GrantedScopes), b.Reason
		if b.EventSubscriptions != nil {
			cmd.EventSubscriptions, cmd.EventSubscriptionsSet = append([]string{}, *b.EventSubscriptions...), true
		}
	}
	a, err := h.apps.SetScopes(ctx, principal(ctx), ids.ID(req.AppId), cmd)
	if err != nil {
		return nil, err
	}
	v, err := appREST(a)
	if err != nil {
		return nil, err
	}
	return rest.SetAppScopes200JSONResponse{Body: v}, nil
}

// SuspendApp implements rest.StrictServerInterface (POST /apps/{app_id}/suspend).
func (h *Apps) SuspendApp(ctx context.Context, req rest.SuspendAppRequestObject) (rest.SuspendAppResponseObject, error) {
	if err := h.apps.Suspend(ctx, principal(ctx), ids.ID(req.AppId)); err != nil {
		return nil, err
	}
	return rest.SuspendApp204Response{}, nil
}

// ResumeApp implements rest.StrictServerInterface (POST /apps/{app_id}/resume).
func (h *Apps) ResumeApp(ctx context.Context, req rest.ResumeAppRequestObject) (rest.ResumeAppResponseObject, error) {
	if err := h.apps.Resume(ctx, principal(ctx), ids.ID(req.AppId)); err != nil {
		return nil, err
	}
	return rest.ResumeApp204Response{}, nil
}

// UninstallApp implements rest.StrictServerInterface (POST /apps/{app_id}/uninstall).
func (h *Apps) UninstallApp(ctx context.Context, req rest.UninstallAppRequestObject) (rest.UninstallAppResponseObject, error) {
	var reason string
	if req.Body != nil {
		reason = req.Body.Reason
	}
	if err := h.apps.Uninstall(ctx, principal(ctx), ids.ID(req.AppId), reason); err != nil {
		return nil, err
	}
	return rest.UninstallApp204Response{}, nil
}

// installCmd converts the body of installApp and upgradeAppManifest. The manifest is the member
// manifest of the body exactly as received (its hash is apps.manifest_sha256, M2-24 design note
// 14), taken from the raw body the chain checked.
func installCmd(ctx context.Context, b *rest.AppInstall) (apps.InstallCmd, error) {
	var cmd apps.InstallCmd
	if b == nil {
		return cmd, errs.New(errs.CodeValidationFailed, "The body is required.")
	}
	cmd.GrantedScopes, cmd.Reason = scopes(b.GrantedScopes), b.Reason
	if b.EventSubscriptions != nil {
		cmd.EventSubscriptions, cmd.EventSubscriptionsSet = append([]string{}, *b.EventSubscriptions...), true
	}
	if raw, ok := httpserver.RawJSONBody(ctx); ok {
		var body struct {
			Manifest json.RawMessage `json:"manifest"`
		}
		if err := json.Unmarshal(raw, &body); err == nil && len(body.Manifest) > 0 {
			cmd.Manifest = bytes.Clone(body.Manifest)
			return cmd, nil
		}
	}
	m, err := json.Marshal(b.Manifest)
	if err != nil {
		return cmd, errs.Wrap(errs.CodeInternalError, err, "apps: manifest")
	}
	cmd.Manifest = m
	return cmd, nil
}

// scopes converts a list of AppScope.
func scopes(list []rest.AppScope) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		out = append(out, string(s))
	}
	return out
}

// appREST converts an app (schema App); empty values are absent members, never null (ADR 0053).
func appREST(a apps.App) (rest.App, error) {
	var m rest.AppManifest
	if err := json.Unmarshal(a.Manifest, &m); err != nil {
		return rest.App{}, errs.Wrap(errs.CodeInternalError, err, "apps: stored manifest")
	}
	granted := make([]rest.AppScope, 0, len(a.Installation.GrantedScopes))
	for _, s := range a.Installation.GrantedScopes {
		granted = append(granted, rest.AppScope(s))
	}
	inst := rest.AppInstallation{
		Id: rest.Uuid(a.Installation.ID), GrantedScopes: granted, EventSubscriptions: append([]string{}, a.Installation.EventSubscriptions...),
		State: rest.AppInstallationState(a.Installation.State), InstalledAt: a.Installation.InstalledAt.UTC().Truncate(time.Millisecond),
	}
	if !a.Installation.InstalledBy.IsZero() {
		by := rest.Uuid(a.Installation.InstalledBy)
		inst.InstalledBy = &by
	}
	return rest.App{
		Id: rest.Uuid(a.ID), Slug: a.Slug, DisplayName: a.DisplayName, Manifest: m, ManifestVersion: a.ManifestVersion, Installation: inst,
	}, nil
}
