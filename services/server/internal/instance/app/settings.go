// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The event and audit names of the settings use cases (05 section 6, brief M1-46).
const (
	eventInstanceUpdated = "instance.updated"
	receiveInstanceRead  = "instance.read"
	attrSettingKey       = "key"
	attrChangedKeys      = "changed_keys"
	attrNoChange         = "no_change"
)

// SettingRow is one row of instance_settings as the use cases read and write it.
type SettingRow struct {
	// Key is the setting key.
	Key string
	// Value is the JSON value in its canonical form (domain.Registry.Validate).
	Value json.RawMessage
	// Version is the row version, 1 after the first write.
	Version int64
	// UpdatedBy is the principal of the last write; zero for NULL (a value written at setup).
	UpdatedBy ids.ID
	// UpdatedAt is the time of the last write.
	UpdatedAt time.Time
}

// SettingsRepository is the outgoing port to instance_settings; adapters/postgres implements it.
// Statements run in the ambient transaction of ctx when there is one.
type SettingsRepository interface {
	// ListSettings returns every stored row in key order.
	ListSettings(ctx context.Context) ([]SettingRow, error)
	// LockSettings first takes the transaction-scoped advisory lock that serializes every writer
	// of instance_settings, also of keys without a row yet, and then returns the stored rows of
	// keys in key order under their row lock until the commit (SELECT ... FOR UPDATE); a key
	// without a row is missing from the result. It requires a unit of work.
	LockSettings(ctx context.Context, keys []string) ([]SettingRow, error)
	// InsertSetting inserts the first row of row.Key with version 1 and returns it. inserted is
	// false when a concurrent writer inserted the key first (ON CONFLICT DO NOTHING); nothing was
	// written then.
	InsertSetting(ctx context.Context, row SettingRow) (out SettingRow, inserted bool, err error)
	// UpdateSetting writes value, updated_by and updated_at of a locked row with version + 1 and
	// returns the new row.
	UpdateSetting(ctx context.Context, row SettingRow) (SettingRow, error)
}

// Contract holds the values of the API contract the use cases report: they come from the
// generated layer api/routes, which the app layer may not import, so internal/wire passes them.
type Contract struct {
	// SpecVersion is routes.SpecVersion, info.version of docs/api/openapi.yaml.
	SpecVersion string
	// GatewayVersions is routes.GatewayProtocolVersions(), info.x-protocol-versions of
	// docs/api/asyncapi.yaml.
	GatewayVersions []int
	// Problems is routes.ProblemTypes(), the one problem catalogue x-problem-codes (06 section 5).
	Problems []instance.ProblemType
}

// AdminDeps are the collaborators of Admin.
type AdminDeps struct {
	// Instance reads the instance row (instance_id, setup_completed_at).
	Instance Repository
	// Settings reads and writes instance_settings.
	Settings SettingsRepository
	// Logins tells discovery the sign-in ways; identity implements it (adapters/loginoptions).
	Logins instance.LoginOptionsSource
	// UoW runs every write.
	UoW txn.UnitOfWork
	// Authorizer decides the five actions; it writes the deny audit itself.
	Authorizer authorize.Authorizer
	// Audit writes the allow entries of instance.settings.update and instance.branding.update.
	Audit auditing.Recorder
	// Events appends instance.updated.
	Events eventlog.Appender
	// Flags reads and writes feature_flags (M1-47).
	Flags FlagRepository
	// FlagGroups are the valid groups of a person and the existing groups; authz implements it.
	FlagGroups instance.FlagGroupSource
	// FlagUsers tells which rollout IDs are people; identity implements it.
	FlagUsers instance.FlagUserSource
	// Contract are the values of the API contract.
	Contract Contract
	// Clock is the time of updated_at; nil means clock.System().
	Clock clock.Clock
}

// Admin implements instance.Admin (the six operations of M1-46), instance.Settings (the read
// port of the other modules), and the feature flags of M1-47: instance.Flags (the evaluation) and
// instance.FlagAdmin (listFeatureFlags, putFeatureFlag), and the logo operations of M3-10:
// instance.Logos (logo.go). It is safe for concurrent use.
type Admin struct {
	cfg Config
	reg *domain.Registry
	d   AdminDeps
	// logos is the source port of the module files, bound after construction by BindLogoFiles
	// because internal/wire builds files after instance (logo.go).
	logos atomic.Pointer[instance.LogoFiles]
}

// Compile-time checks: the incoming port and the read port.
var (
	_ instance.Admin    = (*Admin)(nil)
	_ instance.Settings = (*Admin)(nil)
)

// errConcurrentInsert reports that a concurrent writer inserted a setting key first; the caller
// reads again (putInstanceSetting) or answers version_conflict (updateBranding).
var errConcurrentInsert = errors.New("instance: a concurrent writer inserted the setting first")

// NewAdmin returns the settings, branding and discovery use cases over deps with the
// configuration values of cfg. Errors: a missing collaborator, a limit below its range or a
// client version that is not SemVer.
func NewAdmin(cfg Config, deps AdminDeps) (*Admin, error) {
	if deps.Instance == nil || deps.Settings == nil || deps.Logins == nil || deps.UoW == nil || deps.Authorizer == nil ||
		deps.Audit == nil || deps.Events == nil || deps.Flags == nil || deps.FlagGroups == nil || deps.FlagUsers == nil {
		return nil, errors.New("instance: NewAdmin needs every collaborator")
	}
	if deps.Contract.SpecVersion == "" {
		return nil, errors.New("instance: NewAdmin needs the spec version of the contract")
	}
	if cfg.MessageMaxContentBytes < 1 || cfg.MessageMaxAttachments < 0 || cfg.FileMaxUploadBytes < 1 ||
		cfg.ThreadTitleMaxChars < 1 || cfg.VoiceMaxDuration <= 0 {
		return nil, errors.New("instance: the client limits must be positive")
	}
	for _, v := range cfg.clientVersions() {
		if v != "" && domain.ValidateSemVer(v) != nil {
			return nil, errors.New("instance: a client version is not SemVer 2.0.0")
		}
	}
	reg, err := domain.NewRegistry(cfg.ProductNameMaxChars, instance.SupportedLocales())
	if err != nil {
		return nil, err
	}
	if deps.Clock == nil {
		deps.Clock = clock.System()
	}
	deps.Contract.GatewayVersions = slices.Clone(deps.Contract.GatewayVersions)
	deps.Contract.Problems = slices.Clone(deps.Contract.Problems)
	return &Admin{cfg: cfg, reg: reg, d: deps}, nil
}

// clientVersions returns the eight client version values.
func (c Config) clientVersions() []string {
	return []string{
		c.MinVersionIOS, c.MinVersionAndroid, c.MinVersionMacOS, c.MinVersionWindows,
		c.RecommendedVersionIOS, c.RecommendedVersionAndroid, c.RecommendedVersionMacOS, c.RecommendedVersionWindows,
	}
}

// versionConfig returns the client versions per platform for domain.MinClientVersions.
func (c Config) versionConfig() domain.ClientVersionConfig {
	return domain.ClientVersionConfig{
		Min: map[string]string{"ios": c.MinVersionIOS, "android": c.MinVersionAndroid, "macos": c.MinVersionMacOS, "windows": c.MinVersionWindows},
		Recommended: map[string]string{
			"ios": c.RecommendedVersionIOS, "android": c.RecommendedVersionAndroid, "macos": c.RecommendedVersionMacOS,
			"windows": c.RecommendedVersionWindows,
		},
	}
}

// instanceResource is the resource of the five actions (05 section 4.1 and 4.2).
var instanceResource = authorize.Resource{Kind: authorize.ResourceInstance}

// check authorizes act on the instance for a and returns the decision as an error (nil for allow).
func (s *Admin) check(ctx context.Context, a actor.Actor, act authorize.Action) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, instanceResource)
	if err != nil {
		return err
	}
	return d.Err()
}

// caller returns the actor of the request in ctx, the anonymous actor outside one.
func caller(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// withCaller puts a into ctx; a different actor already in ctx is a programming error.
func withCaller(ctx context.Context, a actor.Actor) (context.Context, error) {
	if have, ok := actor.FromContext(ctx); ok && (have.ID != a.ID || have.Kind != a.Kind) {
		return nil, errs.New(errs.CodeInternalError, "instance: the caller is not the principal of the request")
	}
	return actor.WithActor(ctx, a), nil
}

// view is the effective state of every registered key: the stored row or the default.
type view struct {
	reg  *domain.Registry
	rows map[string]SettingRow
}

// newView indexes rows; rows of keys the registry does not know are dropped (they cannot exist,
// because the writer only accepts registered keys).
func (s *Admin) newView(rows []SettingRow) view {
	v := view{reg: s.reg, rows: map[string]SettingRow{}}
	for _, r := range rows {
		if _, ok := s.reg.Lookup(r.Key); ok {
			v.rows[r.Key] = r
		}
	}
	return v
}

// setting returns the effective setting of key: the stored row, else the default with version 0.
// ok is false for a key without a row and without a default (branding.logo_file_id before
// M3-10): it has no value.
func (v view) setting(key string) (instance.Setting, bool) {
	if r, ok := v.rows[key]; ok {
		return instance.Setting{Key: key, Value: slices.Clone(r.Value), Version: r.Version, UpdatedBy: r.UpdatedBy, UpdatedAt: r.UpdatedAt}, true
	}
	def, _ := v.reg.Lookup(key)
	if def.Default == nil {
		return instance.Setting{Key: key}, false
	}
	return instance.Setting{Key: key, Value: def.Default}, true
}

// str returns the string value of key ("" without a value). Errors: a stored value that is no
// JSON string (internal).
func (v view) str(key string) (string, error) {
	st, ok := v.setting(key)
	if !ok {
		return "", nil
	}
	var out string
	if err := json.Unmarshal(st.Value, &out); err != nil {
		return "", errs.Wrap(errs.CodeInternalError, err, "instance: the stored value of a setting is not a JSON string")
	}
	return out, nil
}

// version returns the row version of key, 0 without a row.
func (v view) version(key string) int64 { return v.rows[key].Version }

// branding returns the branding of the view.
func (v view) branding() (instance.Branding, error) {
	name, err := v.str(domain.KeyProductName)
	if err != nil {
		return instance.Branding{}, err
	}
	accent, err := v.str(domain.KeyAccentColor)
	if err != nil {
		return instance.Branding{}, err
	}
	logo, err := v.logo()
	if err != nil {
		return instance.Branding{}, err
	}
	return instance.Branding{
		ProductName: name, AccentColor: accent, LogoFileID: logo,
		Version: domain.BrandingVersion(v.version(domain.KeyProductName), v.version(domain.KeyAccentColor), v.version(domain.KeyLogoFileID)),
	}, nil
}

// load reads every stored setting.
func (s *Admin) load(ctx context.Context) (view, error) {
	rows, err := s.d.Settings.ListSettings(ctx)
	if err != nil {
		return view{}, err
	}
	return s.newView(rows), nil
}

// record loads the instance row; a missing row is internal (bothy migrate up creates it).
func (s *Admin) record(ctx context.Context) (instance.Record, error) {
	rec, found, err := s.d.Instance.LoadInstance(ctx)
	if err != nil {
		return instance.Record{}, err
	}
	if !found {
		return instance.Record{}, errs.New(errs.CodeInternalError, `instance: the instance row is missing; run "bothy migrate up"`)
	}
	return rec, nil
}

// Branding implements instance.Settings: the current branding, without authorization.
func (s *Admin) Branding(ctx context.Context) (instance.Branding, error) {
	v, err := s.load(ctx)
	if err != nil {
		return instance.Branding{}, err
	}
	return v.branding()
}

// LocaleDefault implements instance.Settings: locale.default, without authorization.
func (s *Admin) LocaleDefault(ctx context.Context) (string, error) {
	v, err := s.load(ctx)
	if err != nil {
		return "", err
	}
	return v.str(domain.KeyLocaleDefault)
}

// Discovery implements instance.Admin (getDiscovery): public, also before setup. setup_required
// is read from the row on every call, because the setup completes while the process runs.
func (s *Admin) Discovery(ctx context.Context) (instance.Discovery, error) {
	if err := s.check(ctx, caller(ctx), authorize.ActionInstanceDiscoveryRead); err != nil {
		return instance.Discovery{}, err
	}
	rec, err := s.record(ctx)
	if err != nil {
		return instance.Discovery{}, err
	}
	v, err := s.load(ctx)
	if err != nil {
		return instance.Discovery{}, err
	}
	b, err := v.branding()
	if err != nil {
		return instance.Discovery{}, err
	}
	opts, err := s.d.Logins.LoginOptions(ctx)
	if err != nil {
		return instance.Discovery{}, err
	}
	providers := slices.Clone(opts.Providers)
	if providers == nil {
		providers = []instance.ProviderInfo{}
	}
	// The source orders by name; sorting again keeps the contract when a source does not.
	slices.SortStableFunc(providers, func(x, y instance.ProviderInfo) int {
		return strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
	})
	kinds := make([]string, 0, len(providers))
	for _, p := range providers {
		kinds = append(kinds, p.Kind)
	}
	out := instance.Discovery{
		InstanceID: rec.InstanceID, SetupRequired: !rec.SetupCompleted(), APIVersions: domain.APIVersions(),
		SpecVersion: s.d.Contract.SpecVersion, Features: domain.DiscoveryFeatures(), GatewayURL: domain.GatewayURL,
		GatewayVersions: slices.Clone(s.d.Contract.GatewayVersions), ProductName: b.ProductName, AccentColor: b.AccentColor,
		Providers: providers, LocalPasswordEnabled: domain.LocalPasswordEnabled(opts.LocalPasswordEnabled, kinds),
		JWKSURI: domain.JWKSURI, WebhookKeysURI: domain.WebhookKeysURI, MinClientVersions: []instance.ClientVersion{},
		LogoURL: domain.DiscoveryLogoURL(b.LogoFileID),
	}
	if out.GatewayVersions == nil {
		out.GatewayVersions = []int{}
	}
	for _, cv := range domain.MinClientVersions(s.cfg.versionConfig()) {
		out.MinClientVersions = append(out.MinClientVersions, instance.ClientVersion{
			Platform: cv.Platform, MinVersion: cv.MinVersion, RecommendedVersion: cv.RecommendedVersion,
		})
	}
	return out, nil
}

// Info implements instance.Admin (getInstance): any active principal; features are the flags
// evaluated for a (agents and apps get the global values).
func (s *Admin) Info(ctx context.Context, a actor.Actor) (instance.Info, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return instance.Info{}, err
	}
	if err := s.check(ctx, a, authorize.ActionInstanceRead); err != nil {
		return instance.Info{}, err
	}
	rec, err := s.record(ctx)
	if err != nil {
		return instance.Info{}, err
	}
	v, err := s.load(ctx)
	if err != nil {
		return instance.Info{}, err
	}
	b, err := v.branding()
	if err != nil {
		return instance.Info{}, err
	}
	locale, err := v.str(domain.KeyLocaleDefault)
	if err != nil {
		return instance.Info{}, err
	}
	features, err := s.EvaluateAll(ctx, instance.SubjectOf(a))
	if err != nil {
		return instance.Info{}, err
	}
	dashboard, err := s.dashboardURL(ctx, a)
	if err != nil {
		return instance.Info{}, err
	}
	return instance.Info{
		InstanceID: rec.InstanceID, Branding: b, LocaleDefault: locale, SpecVersion: s.d.Contract.SpecVersion,
		Features: features, TelemetryDashboardURL: dashboard, StatsTimeZone: s.cfg.StatsTimeZone,
		Limits: instance.Limits{
			MessageMaxContentBytes: s.cfg.MessageMaxContentBytes, MessageMaxAttachments: s.cfg.MessageMaxAttachments,
			FileMaxUploadBytes: s.cfg.FileMaxUploadBytes, ThreadTitleMaxChars: s.cfg.ThreadTitleMaxChars,
			VoiceMaxDuration: s.cfg.VoiceMaxDuration,
		},
	}, nil
}

// dashboardURL returns telemetry.dashboard_url when a may read the statistics of the whole
// instance (stats.read on the instance, which only owners have), else "". The check runs in
// filter mode: a person who is no owner is no failure of getInstance, so it writes no deny
// audit and the member is simply absent (brief M6-06 Design note 4; the URL names internal
// infrastructure).
func (s *Admin) dashboardURL(ctx context.Context, a actor.Actor) (string, error) {
	if s.cfg.TelemetryDashboardURL == "" {
		return "", nil
	}
	d, err := s.d.Authorizer.Authorize(authorize.Filter(ctx), a, authorize.ActionStatsRead, instanceResource)
	if err != nil {
		return "", err
	}
	if !d.Allowed() {
		return "", nil
	}
	return s.cfg.TelemetryDashboardURL, nil
}

// ListSettings implements instance.Admin (listInstanceSettings, owner): every registered key with
// a value in key order; a default has version 0 and no updated_by and updated_at. A key without
// a row and without a default has no value and is not listed (docs/DECISIONS.md M1-46/7).
func (s *Admin) ListSettings(ctx context.Context, a actor.Actor) ([]instance.Setting, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return nil, err
	}
	if err := s.check(ctx, a, authorize.ActionInstanceSettingsRead); err != nil {
		return nil, err
	}
	v, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	out := []instance.Setting{}
	for _, key := range s.reg.Keys() {
		if st, ok := v.setting(key); ok {
			out = append(out, st)
		}
	}
	return out, nil
}

// matches reports whether a present If-Match names version; an invalid header never matches.
func matches(im instance.IfMatch, version int64) bool {
	return im.Valid && im.Version == version
}

// PutSetting implements instance.Admin (putInstanceSetting, owner with step-up). See the
// interface for the errors; the brief M1-46 "putInstanceSetting" for the order of the checks.
func (s *Admin) PutSetting(ctx context.Context, a actor.Actor, cmd instance.PutSetting) (instance.Setting, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return instance.Setting{}, err
	}
	var out instance.Setting
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, a, authorize.ActionInstanceSettingsUpdate); err != nil {
			return err
		}
		def, ok := s.reg.Lookup(cmd.Key)
		if !ok {
			return errs.New(errs.CodeNotFound, "")
		}
		if def.Route == domain.RouteBranding {
			return domain.UseBrandingEndpoint()
		}
		value, err := s.reg.Validate(cmd.Key, cmd.Value)
		if err != nil {
			return err
		}
		// A concurrent first write of the key makes the insert return nothing; the second round
		// sees its row under the lock.
		for range 2 {
			out, err = s.putLocked(ctx, a, cmd, value)
			if !errors.Is(err, errConcurrentInsert) {
				return err
			}
		}
		return errs.New(errs.CodeInternalError, "instance: the setting row is neither insertable nor lockable")
	})
	return out, err
}

// putLocked is one round of PutSetting under the row lock of the key.
func (s *Admin) putLocked(ctx context.Context, a actor.Actor, cmd instance.PutSetting, value json.RawMessage) (instance.Setting, error) {
	rows, err := s.d.Settings.LockSettings(ctx, []string{cmd.Key})
	if err != nil {
		return instance.Setting{}, err
	}
	v := s.newView(rows)
	cur, _ := v.setting(cmd.Key)
	if cmd.IfMatch.Present && !matches(cmd.IfMatch, cur.Version) {
		return instance.Setting{}, errs.VersionConflict(cur)
	}
	if sameJSON(cur.Value, value) {
		return cur, s.recordSetting(ctx, cmd.Key, true)
	}
	row := SettingRow{Key: cmd.Key, Value: value, UpdatedBy: a.ID, UpdatedAt: s.d.Clock.Now()}
	written, err := s.write(ctx, v, row)
	if err != nil {
		return instance.Setting{}, err
	}
	if err := s.emit(ctx, []string{cmd.Key}); err != nil {
		return instance.Setting{}, err
	}
	if err := s.recordSetting(ctx, cmd.Key, false); err != nil {
		return instance.Setting{}, err
	}
	return instance.Setting{Key: written.Key, Value: written.Value, Version: written.Version, UpdatedBy: written.UpdatedBy, UpdatedAt: written.UpdatedAt}, nil
}

// sameJSON reports whether a and b are the same JSON value; the byte forms differ, because jsonb
// returns its own canonical text (for example "&" where encoding/json writes "\u0026").
func sameJSON(a, b json.RawMessage) bool {
	if bytes.Equal(a, b) {
		return true
	}
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

// write inserts row when v holds no row of its key and updates the locked row otherwise.
// Errors: errConcurrentInsert when a concurrent writer inserted the key first.
func (s *Admin) write(ctx context.Context, v view, row SettingRow) (SettingRow, error) {
	if _, stored := v.rows[row.Key]; stored {
		return s.d.Settings.UpdateSetting(ctx, row)
	}
	out, inserted, err := s.d.Settings.InsertSetting(ctx, row)
	if err != nil {
		return SettingRow{}, err
	}
	if !inserted {
		return SettingRow{}, errConcurrentInsert
	}
	return out, nil
}

// recordSetting writes the allow entry of instance.settings.update with the key and never the
// value, so one rule covers every key, also keys whose values are personal.
func (s *Admin) recordSetting(ctx context.Context, key string, noChange bool) error {
	attrs := auditing.Attrs{attrSettingKey: auditing.Code(key)}
	if noChange {
		attrs[attrNoChange] = auditing.Bool(true)
	}
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionInstanceSettingsUpdate), Decision: auditing.DecisionAllow,
		ResourceKind: string(authorize.ResourceInstance), Attrs: attrs,
	})
}

// emit appends instance.updated with the sorted changed keys (05 section 6): resource instance
// without version, empty scope, the actor of the request.
func (s *Admin) emit(ctx context.Context, keys []string) error {
	rec, err := s.record(ctx)
	if err != nil {
		return err
	}
	sorted := slices.Clone(keys)
	slices.Sort(sorted)
	payload, err := json.Marshal(struct {
		ChangedKeys []string `json:"changed_keys"`
	}{sorted})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "instance: instance.updated payload")
	}
	kind, id := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: eventInstanceUpdated, SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: id,
		ResourceKind: string(authorize.ResourceInstance), ResourceID: rec.InstanceID, ReceiveAction: receiveInstanceRead,
		Payload: payload,
	})
}

// eventActor returns the actor of the request in ctx; none outside a request.
func eventActor(ctx context.Context) (eventlog.ActorKind, ids.ID) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.ID.IsZero() {
		return "", ids.ID{}
	}
	switch a.Kind {
	case actor.KindUser:
		return eventlog.ActorUser, a.ID
	case actor.KindAgent:
		return eventlog.ActorAgent, a.ID
	case actor.KindApp:
		return eventlog.ActorApp, a.ID
	case actor.KindHost, actor.KindAnonymous:
	}
	return "", ids.ID{}
}

// brandingKeys are the keys whose versions make the branding version, in key order.
var brandingKeys = []string{domain.KeyAccentColor, domain.KeyLogoFileID, domain.KeyProductName}

// UpdateBranding implements instance.Admin (updateBranding, owner with step-up). PUT replaces both
// members: an absent accent colour means the default, written as a value. Only changed keys are
// written and named in instance.updated; no change writes nothing and no event.
func (s *Admin) UpdateBranding(ctx context.Context, a actor.Actor, cmd instance.UpdateBranding) (instance.Branding, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return instance.Branding{}, err
	}
	var out instance.Branding
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, a, authorize.ActionInstanceBrandingUpdate); err != nil {
			return err
		}
		next, err := s.brandingValues(cmd)
		if err != nil {
			return err
		}
		out, err = s.brandingLocked(ctx, a, cmd.IfMatch, next)
		return err
	})
	if errors.Is(err, errConcurrentInsert) {
		// A concurrent first write of a branding key committed a newer branding, so the version the
		// caller named is outdated.
		cur, rerr := s.Branding(txn.Detach(ctx))
		if rerr != nil {
			return instance.Branding{}, rerr
		}
		return instance.Branding{}, errs.VersionConflict(cur)
	}
	return out, err
}

// brandingValues validates the body of updateBranding; both field errors are reported together.
func (s *Admin) brandingValues(cmd instance.UpdateBranding) (domain.BrandingValues, error) {
	next := domain.BrandingValues{AccentColor: domain.DefaultAccentColor}
	var fields []errs.FieldError
	name, err := domain.ValidateProductName(cmd.ProductName, s.reg.ProductNameMax())
	fields = appendFields(fields, err)
	next.ProductName = name
	if cmd.AccentColor != nil {
		next.AccentColor = *cmd.AccentColor
		fields = appendFields(fields, domain.ValidateAccentColor(next.AccentColor))
	}
	if len(fields) > 0 {
		return domain.BrandingValues{}, errs.New(errs.CodeValidationFailed, "").WithFields(fields...)
	}
	return next, nil
}

// appendFields appends the field errors of a validation_failed err; nil adds nothing.
func appendFields(fields []errs.FieldError, err error) []errs.FieldError {
	if e, ok := errs.As(err); ok {
		return append(fields, e.Fields...)
	}
	return fields
}

// brandingLocked is UpdateBranding under the row locks of the branding keys.
func (s *Admin) brandingLocked(ctx context.Context, a actor.Actor, im instance.IfMatch, next domain.BrandingValues) (instance.Branding, error) {
	rows, err := s.d.Settings.LockSettings(ctx, brandingKeys)
	if err != nil {
		return instance.Branding{}, err
	}
	v := s.newView(rows)
	cur, err := v.branding()
	if err != nil {
		return instance.Branding{}, err
	}
	if !matches(im, cur.Version) {
		return instance.Branding{}, errs.VersionConflict(cur)
	}
	changed := domain.BrandingChanges(domain.BrandingValues{ProductName: cur.ProductName, AccentColor: cur.AccentColor}, next)
	if len(changed) == 0 {
		return cur, s.recordBranding(ctx, nil)
	}
	now := s.d.Clock.Now()
	for _, key := range changed {
		val := next.ProductName
		if key == domain.KeyAccentColor {
			val = next.AccentColor
		}
		raw, err := json.Marshal(val)
		if err != nil {
			return instance.Branding{}, errs.Wrap(errs.CodeInternalError, err, "instance: branding value")
		}
		row, err := s.write(ctx, v, SettingRow{Key: key, Value: raw, UpdatedBy: a.ID, UpdatedAt: now})
		if err != nil {
			return instance.Branding{}, err
		}
		v.rows[key] = row
	}
	if err := s.emit(ctx, changed); err != nil {
		return instance.Branding{}, err
	}
	if err := s.recordBranding(ctx, changed); err != nil {
		return instance.Branding{}, err
	}
	return v.branding()
}

// recordBranding writes the allow entry of instance.branding.update with the changed keys, or
// no_change when nothing changed.
func (s *Admin) recordBranding(ctx context.Context, changed []string) error {
	attrs := auditing.Attrs{attrNoChange: auditing.Bool(true)}
	if len(changed) > 0 {
		vs := make([]auditing.Value, 0, len(changed))
		for _, k := range changed {
			vs = append(vs, auditing.Code(k))
		}
		attrs = auditing.Attrs{attrChangedKeys: auditing.List(vs...)}
	}
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionInstanceBrandingUpdate), Decision: auditing.DecisionAllow,
		ResourceKind: string(authorize.ResourceInstance), Attrs: attrs,
	})
}

// ProblemType implements instance.Admin (getProblemType, public): the entry of code in the one
// problem catalogue; an unknown code is not_found.
func (s *Admin) ProblemType(ctx context.Context, code string) (instance.ProblemType, error) {
	if err := s.check(ctx, caller(ctx), authorize.ActionInstanceDiscoveryRead); err != nil {
		return instance.ProblemType{}, err
	}
	i := slices.IndexFunc(s.d.Contract.Problems, func(p instance.ProblemType) bool { return p.Code == code })
	if i < 0 {
		return instance.ProblemType{}, errs.New(errs.CodeNotFound, "")
	}
	return s.d.Contract.Problems[i], nil
}
