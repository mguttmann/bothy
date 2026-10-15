// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// fakeAdmin is the port instance.Admin with fixed answers; it records the commands it got.
type fakeAdmin struct {
	discovery instance.Discovery
	info      instance.Info
	settings  []instance.Setting
	setting   instance.Setting
	branding  instance.Branding
	problem   instance.ProblemType
	err       error
	put       instance.PutSetting
	update    instance.UpdateBranding
	flags     []instance.FeatureFlag
	flag      instance.FeatureFlag
	putFlag   instance.PutFlagCmd
}

func (f *fakeAdmin) Discovery(context.Context) (instance.Discovery, error) { return f.discovery, f.err }
func (f *fakeAdmin) Info(context.Context, actor.Actor) (instance.Info, error) {
	return f.info, f.err
}
func (f *fakeAdmin) ListSettings(context.Context, actor.Actor) ([]instance.Setting, error) {
	return f.settings, f.err
}
func (f *fakeAdmin) PutSetting(_ context.Context, _ actor.Actor, cmd instance.PutSetting) (instance.Setting, error) {
	f.put = cmd
	return f.setting, f.err
}
func (f *fakeAdmin) UpdateBranding(_ context.Context, _ actor.Actor, cmd instance.UpdateBranding) (instance.Branding, error) {
	f.update = cmd
	return f.branding, f.err
}
func (f *fakeAdmin) ListFlags(context.Context, actor.Actor) ([]instance.FeatureFlag, error) {
	return f.flags, f.err
}
func (f *fakeAdmin) PutFlag(_ context.Context, _ actor.Actor, cmd instance.PutFlagCmd) (instance.FeatureFlag, error) {
	f.putFlag = cmd
	return f.flag, f.err
}
func (f *fakeAdmin) ProblemType(context.Context, string) (instance.ProblemType, error) {
	return f.problem, f.err
}

func TestAdmin_ConvertsDiscovery(t *testing.T) {
	t.Parallel()
	p := ids.New()
	f := &fakeAdmin{discovery: instance.Discovery{
		InstanceID: ids.New(), SetupRequired: true, APIVersions: []string{"v1"}, SpecVersion: "1.0.0", GatewayURL: "/g",
		ProductName: "Team", AccentColor: "loch", LogoURL: "/logo", Providers: []instance.ProviderInfo{{ID: p, Kind: "oidc", Name: "SSO"}},
		MinClientVersions: []instance.ClientVersion{{Platform: "ios", MinVersion: "1.0.0", RecommendedVersion: "1.1.0"}, {Platform: "macos", MinVersion: "2.0.0"}},
	}}
	r, err := httpapi.NewAdmin(f).GetDiscovery(context.Background(), rest.GetDiscoveryRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	d := r.(rest.GetDiscovery200JSONResponse).Body
	if d.Features == nil || len(*d.Features) != 0 || d.Gateway.ProtocolVersions == nil || *d.Branding.LogoUrl != "/logo" ||
		*d.Branding.AccentColor != "loch" || d.AuthProviders[0].Id != rest.Uuid(p) || *d.LocalPasswordEnabled {
		t.Errorf("discovery %+v", d)
	}
	vs := *d.MinClientVersions
	if len(vs) != 2 || *vs[0].RecommendedVersion != "1.1.0" || vs[1].RecommendedVersion != nil {
		t.Errorf("versions %+v", vs)
	}
}

func TestAdmin_ConvertsInstanceWithContentETag(t *testing.T) {
	t.Parallel()
	logo := ids.New()
	f := &fakeAdmin{info: instance.Info{
		InstanceID: ids.New(), Branding: instance.Branding{ProductName: "Team", AccentColor: "moss", LogoFileID: logo, Version: 3},
		LocaleDefault: "en", SpecVersion: "1.0.0", Features: []instance.EvaluatedFlag{{Key: "voice.enabled", Enabled: true}},
		Limits: instance.Limits{VoiceMaxDuration: 90 * time.Second},
	}}
	h := httpapi.NewAdmin(f)
	r, err := h.GetInstance(context.Background(), rest.GetInstanceRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	res := r.(rest.GetInstance200JSONResponse)
	if res.Body.Limits.VoiceMaxDurationMs != 90000 || *res.Body.Branding.LogoFileId != rest.Uuid(logo) || len(res.Body.Features) != 1 {
		t.Errorf("instance %+v", res.Body)
	}
	again, _ := h.GetInstance(context.Background(), rest.GetInstanceRequestObject{})
	if tag := *res.Headers.ETag; len(tag) != 24 || tag != *again.(rest.GetInstance200JSONResponse).Headers.ETag {
		t.Errorf("ETag %q", tag)
	}
	f.info.LocaleDefault = "de"
	changed, _ := h.GetInstance(context.Background(), rest.GetInstanceRequestObject{})
	if *changed.(rest.GetInstance200JSONResponse).Headers.ETag == *res.Headers.ETag {
		t.Error("the ETag did not change with the body")
	}
}

func TestAdmin_ConvertsSettings(t *testing.T) {
	t.Parallel()
	by := ids.New()
	at := time.Date(2026, 10, 13, 9, 0, 0, 123456789, time.UTC)
	f := &fakeAdmin{settings: []instance.Setting{
		{Key: "branding.accent_color", Value: json.RawMessage(`"spruce"`)},
		{Key: "locale.default", Value: json.RawMessage(`"en"`), Version: 2, UpdatedBy: by, UpdatedAt: at},
	}}
	r, err := httpapi.NewAdmin(f).ListInstanceSettings(context.Background(), rest.ListInstanceSettingsRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	items := r.(rest.ListInstanceSettings200JSONResponse).Body.Items
	if items[0].UpdatedAt != nil || items[0].UpdatedBy != nil || items[0].Value != "spruce" {
		t.Errorf("default %+v", items[0])
	}
	if *items[1].UpdatedBy != rest.Uuid(by) || !items[1].UpdatedAt.Equal(at.Truncate(time.Millisecond)) {
		t.Errorf("stored %+v", items[1])
	}
	f.settings = []instance.Setting{{Key: "x", Value: json.RawMessage(`{`)}}
	if _, err := httpapi.NewAdmin(f).ListInstanceSettings(context.Background(), rest.ListInstanceSettingsRequestObject{}); err == nil {
		t.Error("a broken stored value converted")
	}
}

func TestAdmin_PutSettingParsesIfMatchAndSetsETag(t *testing.T) {
	t.Parallel()
	f := &fakeAdmin{setting: instance.Setting{Key: "locale.default", Value: json.RawMessage(`"en"`), Version: 4}}
	h := httpapi.NewAdmin(f)
	tag := `"3"`
	r, err := h.PutInstanceSetting(context.Background(), rest.PutInstanceSettingRequestObject{
		Key: "locale.default", Params: rest.PutInstanceSettingParams{IfMatch: &tag}, Body: &rest.InstanceSettingPut{Value: "en"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := *r.(rest.PutInstanceSetting200JSONResponse).Headers.ETag; got != `"4"` {
		t.Errorf("ETag %q", got)
	}
	if want := (instance.IfMatch{Present: true, Valid: true, Version: 3}); f.put.IfMatch != want || string(f.put.Value) != `"en"` {
		t.Errorf("command %+v", f.put)
	}
	if _, err := h.PutInstanceSetting(context.Background(), rest.PutInstanceSettingRequestObject{Key: "locale.default"}); err != nil || f.put.IfMatch.Present {
		t.Errorf("without If-Match and body: %+v %v", f.put, err)
	}
	f.err = errs.VersionConflict(instance.Setting{Key: "locale.default", Value: json.RawMessage(`"de"`), Version: 2})
	_, err = h.PutInstanceSetting(context.Background(), rest.PutInstanceSettingRequestObject{Key: "locale.default"})
	if e, ok := errs.As(err); !ok || e.Current.(rest.InstanceSetting).Version != 2 {
		t.Errorf("current not in API form: %v", err)
	}
}

func TestAdmin_UpdateBrandingConvertsCurrent(t *testing.T) {
	t.Parallel()
	loch := "loch"
	f := &fakeAdmin{branding: instance.Branding{ProductName: "Team", Version: 5}}
	h := httpapi.NewAdmin(f)
	r, err := h.UpdateBranding(context.Background(), rest.UpdateBrandingRequestObject{
		Params: rest.UpdateBrandingParams{IfMatch: `W/"4"`}, Body: &rest.BrandingUpdate{ProductName: "Team", AccentColor: &loch},
	})
	if err != nil {
		t.Fatal(err)
	}
	res := r.(rest.UpdateBranding200JSONResponse)
	if *res.Headers.ETag != `"5"` || res.Body.AccentColor != nil || f.update.IfMatch.Valid || *f.update.AccentColor != "loch" {
		t.Errorf("response %+v command %+v", res, f.update)
	}
	f.err = errs.VersionConflict(instance.Branding{ProductName: "Old", AccentColor: "moss", Version: 7})
	_, err = h.UpdateBranding(context.Background(), rest.UpdateBrandingRequestObject{Params: rest.UpdateBrandingParams{IfMatch: `"1"`}})
	if e, ok := errs.As(err); !ok || e.Current.(rest.Branding).Version != 7 {
		t.Errorf("current not in API form: %v", err)
	}
	f.err = errs.VersionConflict(instance.Setting{Key: "x", Value: json.RawMessage(`{`)})
	if _, err := h.UpdateBranding(context.Background(), rest.UpdateBrandingRequestObject{}); err == nil || errs.KindOf(err) != errs.KindInternal {
		t.Errorf("broken current: %v", err)
	}
}

func TestAdmin_ProblemTypeAndErrors(t *testing.T) {
	t.Parallel()
	f := &fakeAdmin{problem: instance.ProblemType{Code: "gone", Status: 410, Title: "Gone", Description: "Expired.", ClientAction: "Start again."}}
	h := httpapi.NewAdmin(f)
	r, err := h.GetProblemType(context.Background(), rest.GetProblemTypeRequestObject{Code: "gone"})
	if err != nil || *r.(rest.GetProblemType200JSONResponse).Body.ClientAction != "Start again." {
		t.Fatalf("problem type %+v %v", r, err)
	}
	f.problem.ClientAction = ""
	r, _ = h.GetProblemType(context.Background(), rest.GetProblemTypeRequestObject{Code: "gone"})
	if r.(rest.GetProblemType200JSONResponse).Body.ClientAction != nil {
		t.Error("empty client action sent")
	}
	boom := errors.New("down")
	f.err = boom
	ctx := context.Background()
	if _, err := h.GetDiscovery(ctx, rest.GetDiscoveryRequestObject{}); !errors.Is(err, boom) {
		t.Error("discovery error lost")
	}
	if _, err := h.GetInstance(ctx, rest.GetInstanceRequestObject{}); !errors.Is(err, boom) {
		t.Error("instance error lost")
	}
	if _, err := h.ListInstanceSettings(ctx, rest.ListInstanceSettingsRequestObject{}); !errors.Is(err, boom) {
		t.Error("list error lost")
	}
	if _, err := h.PutInstanceSetting(ctx, rest.PutInstanceSettingRequestObject{}); !errors.Is(err, boom) {
		t.Error("put error lost")
	}
	if _, err := h.GetProblemType(ctx, rest.GetProblemTypeRequestObject{}); !errors.Is(err, boom) {
		t.Error("problem error lost")
	}
}
