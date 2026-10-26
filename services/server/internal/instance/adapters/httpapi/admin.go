// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// Admin implements getDiscovery, getInstance, listInstanceSettings, putInstanceSetting,
// updateBranding and getProblemType of rest.StrictServerInterface over instance.Admin (M1-46).
// The use cases authorize; the handlers convert, set the ETags and turn the current state of a
// version_conflict into its API form. Other errors go to the problem writer of the chain
// unchanged.
type Admin struct {
	admin instance.Admin
}

// NewAdmin returns the handlers over the port.
func NewAdmin(a instance.Admin) *Admin { return &Admin{admin: a} }

// GetDiscovery implements rest.StrictServerInterface (GET /instance/discovery, public).
func (h *Admin) GetDiscovery(ctx context.Context, _ rest.GetDiscoveryRequestObject) (rest.GetDiscoveryResponseObject, error) {
	d, err := h.admin.Discovery(ctx)
	if err != nil {
		return nil, err
	}
	providers := make([]rest.AuthProviderInfo, 0, len(d.Providers))
	for _, p := range d.Providers {
		providers = append(providers, rest.AuthProviderInfo{Id: rest.Uuid(p.ID), Kind: rest.IdentityProviderKind(p.Kind), Name: p.Name})
	}
	versions := make([]rest.ClientVersionRequirement, 0, len(d.MinClientVersions))
	for _, v := range d.MinClientVersions {
		cv := rest.ClientVersionRequirement{Platform: rest.DevicePlatform(v.Platform), MinVersion: v.MinVersion}
		if v.RecommendedVersion != "" {
			cv.RecommendedVersion = ptr(v.RecommendedVersion)
		}
		versions = append(versions, cv)
	}
	branding := rest.PublicBranding{ProductName: d.ProductName}
	if d.AccentColor != "" {
		branding.AccentColor = ptr(d.AccentColor)
	}
	if d.LogoURL != "" {
		branding.LogoUrl = ptr(d.LogoURL)
	}
	features := append([]string{}, d.Features...)
	return rest.GetDiscovery200JSONResponse{Body: rest.Discovery{
		InstanceId: rest.Uuid(d.InstanceID), SetupRequired: d.SetupRequired, ApiVersions: append([]string{}, d.APIVersions...),
		SpecVersion: d.SpecVersion, Features: &features,
		Gateway:  rest.GatewayInfo{Url: d.GatewayURL, ProtocolVersions: append([]int{}, d.GatewayVersions...)},
		Branding: branding, AuthProviders: providers, LocalPasswordEnabled: ptr(d.LocalPasswordEnabled),
		JwksUri: d.JWKSURI, WebhookKeysUri: d.WebhookKeysURI, MinClientVersions: &versions,
	}}, nil
}

// GetInstance implements rest.StrictServerInterface (GET /instance). The ETag is a hash of the
// body (domain.ContentETag), because the body differs per caller once flags exist (M1-47).
func (h *Admin) GetInstance(ctx context.Context, _ rest.GetInstanceRequestObject) (rest.GetInstanceResponseObject, error) {
	info, err := h.admin.Info(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	features := make([]rest.EvaluatedFlag, 0, len(info.Features))
	for _, f := range info.Features {
		features = append(features, rest.EvaluatedFlag{Key: f.Key, Enabled: f.Enabled})
	}
	body := rest.InstanceInfo{
		InstanceId: rest.Uuid(info.InstanceID), Branding: toRESTBranding(info.Branding), LocaleDefault: info.LocaleDefault,
		SpecVersion: info.SpecVersion, Features: features,
		Limits: rest.InstanceLimits{
			MessageMaxContentBytes: info.Limits.MessageMaxContentBytes, MessageMaxAttachments: info.Limits.MessageMaxAttachments,
			FileMaxUploadBytes: info.Limits.FileMaxUploadBytes, ThreadTitleMaxChars: info.Limits.ThreadTitleMaxChars,
			VoiceMaxDurationMs: info.Limits.VoiceMaxDuration.Milliseconds(),
		},
		AppLock: rest.AppLockPolicy{Required: info.AppLock.Required, AfterMs: info.AppLock.After.Milliseconds()},
	}
	if info.StatsTimeZone != "" {
		body.StatsTimeZone = ptr(info.StatsTimeZone)
	}
	if info.TelemetryDashboardURL != "" {
		body.TelemetryDashboardUrl = ptr(info.TelemetryDashboardURL)
	}
	if info.GrafanaEmbedURL != "" {
		body.GrafanaEmbedUrl = ptr(info.GrafanaEmbedURL)
	}
	// encoding/json writes a struct in field order and a map in key order, so the same view
	// always gives the same bytes and the same tag.
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "instance: the instance view")
	}
	return rest.GetInstance200JSONResponse{Body: body, Headers: rest.GetInstance200ResponseHeaders{ETag: ptr(domain.ContentETag(raw))}}, nil
}

// ListInstanceSettings implements rest.StrictServerInterface (GET /instance/settings, owner).
func (h *Admin) ListInstanceSettings(ctx context.Context, _ rest.ListInstanceSettingsRequestObject) (rest.ListInstanceSettingsResponseObject, error) {
	list, err := h.admin.ListSettings(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	out := rest.InstanceSettingList{Items: make([]rest.InstanceSetting, 0, len(list))}
	for _, s := range list {
		item, err := toRESTSetting(s)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, item)
	}
	return rest.ListInstanceSettings200JSONResponse{Body: out}, nil
}

// PutInstanceSetting implements rest.StrictServerInterface (PUT /instance/settings/{key}, owner
// with step-up, optional If-Match).
func (h *Admin) PutInstanceSetting(ctx context.Context, req rest.PutInstanceSettingRequestObject) (rest.PutInstanceSettingResponseObject, error) {
	cmd := instance.PutSetting{Key: req.Key, IfMatch: ifMatch(req.Params.IfMatch)}
	if req.Body != nil {
		raw, err := json.Marshal(req.Body.Value)
		if err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, err, "instance: the setting value")
		}
		cmd.Value = raw
	}
	s, err := h.admin.PutSetting(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, currentForAPI(err)
	}
	out, err := toRESTSetting(s)
	if err != nil {
		return nil, err
	}
	return rest.PutInstanceSetting200JSONResponse{Body: out, Headers: rest.PutInstanceSetting200ResponseHeaders{ETag: ptr(httpserver.FormatETag(s.Version))}}, nil
}

// UpdateBranding implements rest.StrictServerInterface (PUT /instance/branding, owner with
// step-up, If-Match required; the chain answers 428 without it).
func (h *Admin) UpdateBranding(ctx context.Context, req rest.UpdateBrandingRequestObject) (rest.UpdateBrandingResponseObject, error) {
	im := req.Params.IfMatch
	cmd := instance.UpdateBranding{IfMatch: ifMatch(&im)}
	if req.Body != nil {
		cmd.ProductName, cmd.AccentColor = req.Body.ProductName, req.Body.AccentColor
	}
	b, err := h.admin.UpdateBranding(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, currentForAPI(err)
	}
	return rest.UpdateBranding200JSONResponse{Body: toRESTBranding(b), Headers: rest.UpdateBranding200ResponseHeaders{ETag: ptr(httpserver.FormatETag(b.Version))}}, nil
}

// GetProblemType implements rest.StrictServerInterface (GET /problems/{code}, public).
func (h *Admin) GetProblemType(ctx context.Context, req rest.GetProblemTypeRequestObject) (rest.GetProblemTypeResponseObject, error) {
	p, err := h.admin.ProblemType(ctx, req.Code)
	if err != nil {
		return nil, err
	}
	out := rest.ProblemType{Code: p.Code, Status: p.Status, Title: p.Title, Description: p.Description}
	if p.ClientAction != "" {
		out.ClientAction = ptr(p.ClientAction)
	}
	return rest.GetProblemType200JSONResponse{Body: out}, nil
}

// ifMatch parses the If-Match header value v (nil when absent) with the parser of the chain.
func ifMatch(v *string) instance.IfMatch {
	if v == nil {
		return instance.IfMatch{}
	}
	h := http.Header{}
	h.Set("If-Match", *v)
	im := httpserver.ParseIfMatch(h)
	return instance.IfMatch{Present: im.Present, Valid: im.Valid, Version: im.Version}
}

// currentForAPI replaces the current state of a version_conflict with its API form (InstanceSetting
// or Branding), so the problem body carries the schema of the resource.
func currentForAPI(err error) error {
	e, ok := errs.As(err)
	if !ok || e.Current == nil {
		return err
	}
	switch cur := e.Current.(type) {
	case instance.Setting:
		s, cerr := toRESTSetting(cur)
		if cerr != nil {
			return cerr
		}
		e.Current = s
	case instance.Branding:
		e.Current = toRESTBranding(cur)
	}
	return err
}

// toRESTBranding converts the branding (schema Branding).
func toRESTBranding(b instance.Branding) rest.Branding {
	out := rest.Branding{ProductName: b.ProductName, Version: b.Version}
	if b.AccentColor != "" {
		out.AccentColor = ptr(b.AccentColor)
	}
	if !b.LogoFileID.IsZero() {
		id := rest.Uuid(b.LogoFileID)
		out.LogoFileId = &id
	}
	return out
}

// toRESTSetting converts a setting (schema InstanceSetting); a default has neither updated_by nor
// updated_at.
func toRESTSetting(s instance.Setting) (rest.InstanceSetting, error) {
	var v any
	if err := json.Unmarshal(s.Value, &v); err != nil {
		return rest.InstanceSetting{}, errs.Wrap(errs.CodeInternalError, err, "instance: the stored value of a setting")
	}
	out := rest.InstanceSetting{Key: s.Key, Value: v, Version: s.Version}
	if !s.UpdatedBy.IsZero() {
		id := rest.Uuid(s.UpdatedBy)
		out.UpdatedBy = &id
	}
	if !s.UpdatedAt.IsZero() {
		t := s.UpdatedAt.UTC().Truncate(time.Millisecond)
		out.UpdatedAt = &t
	}
	return out, nil
}

// ptr returns a pointer to v.
func ptr[T any](v T) *T { return &v }
