// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// supportedLocales are the languages of the product (docs/spec/01-produkt.md section 9).
var supportedLocales = []string{"de", "en"}

// SupportedLocales returns the languages of the product, the allowed values of locale.default
// (01 section 9); a fresh slice. It lives in the port so that identity can check against the same
// list.
func SupportedLocales() []string { return append([]string(nil), supportedLocales...) }

// Branding is the branding of the instance as other modules and the API read it.
type Branding struct {
	// ProductName is branding.product_name.
	ProductName string
	// AccentColor is branding.accent_color, an accent key of kernel/designkeys.
	AccentColor string
	// LogoFileID is branding.logo_file_id; zero while no logo is set (M3-10 writes it).
	LogoFileID ids.ID
	// Version is the sum of the versions of the three branding keys, 0 for a key without a row
	// (docs/DECISIONS.md M1-46/2); the ETag of updateBranding.
	Version int64
}

// Settings is the read port of the instance settings for other modules (02 4.3).
type Settings interface {
	// Branding returns the current branding. Errors: the database error.
	Branding(ctx context.Context) (Branding, error)
	// LocaleDefault returns locale.default, the language of new people (createUser of identity
	// stores it when the request names none). Errors: the database error.
	LocaleDefault(ctx context.Context) (string, error)
}

// ProviderInfo is one enabled sign-in provider for discovery.
type ProviderInfo struct {
	// ID is the provider id.
	ID ids.ID
	// Kind is oidc, trusted_header or local.
	Kind string
	// Name is the display name.
	Name string
}

// LoginOptions are the sign-in ways discovery reports.
type LoginOptions struct {
	// Providers are the enabled providers, ordered by name.
	Providers []ProviderInfo
	// LocalPasswordEnabled is auth.local_password_enabled of the configuration of identity.
	LocalPasswordEnabled bool
}

// LoginOptionsSource tells discovery which sign-in ways exist. Declared here because instance
// may not import identity (02 4.4); identity implements it (02 4.1 item 4) in
// identity/adapters/loginoptions, and internal/wire passes it to the use cases.
type LoginOptionsSource interface {
	// LoginOptions returns the enabled providers and the password switch. Errors: the database
	// error.
	LoginOptions(ctx context.Context) (LoginOptions, error)
}

// Setting is one instance setting as listInstanceSettings and putInstanceSetting return it.
type Setting struct {
	// Key is the setting key.
	Key string
	// Value is the JSON value.
	Value json.RawMessage
	// Version is the row version; 0 for a key without a row (its default).
	Version int64
	// UpdatedBy is the principal of the last write; zero for a default or a value written at
	// setup.
	UpdatedBy ids.ID
	// UpdatedAt is the time of the last write; zero for a default.
	UpdatedAt time.Time
}

// IfMatch is the parsed If-Match header of a write (06 section 8).
type IfMatch struct {
	// Present is true when the header was sent.
	Present bool
	// Valid is true when it is exactly one strong ETag with a decimal version; a present header
	// that is not valid never matches.
	Valid bool
	// Version is the version of a valid header.
	Version int64
}

// PutSetting is the command of putInstanceSetting.
type PutSetting struct {
	// Key is the path parameter key.
	Key string
	// Value is the member value of the body.
	Value json.RawMessage
	// IfMatch is the optional precondition.
	IfMatch IfMatch
}

// UpdateBranding is the command of updateBranding: PUT replaces both members, an absent accent
// colour means the default.
type UpdateBranding struct {
	// ProductName is the member product_name.
	ProductName string
	// AccentColor is the member accent_color; nil when absent.
	AccentColor *string
	// IfMatch is the required precondition; the chain answers 428 when it is missing.
	IfMatch IfMatch
}

// Limits are the client limits of getInstance (schema InstanceLimits).
type Limits struct {
	MessageMaxContentBytes int64
	MessageMaxAttachments  int
	FileMaxUploadBytes     int64
	ThreadTitleMaxChars    int
	VoiceMaxDuration       time.Duration
}

// Info is the instance view of getInstance (schema InstanceInfo).
type Info struct {
	InstanceID    ids.ID
	Branding      Branding
	LocaleDefault string
	SpecVersion   string
	// Features are the flags evaluated for the caller; empty until M1-47.
	Features []EvaluatedFlag
	Limits   Limits
}

// EvaluatedFlag is one feature flag evaluated for a caller (M1-47 fills the list).
type EvaluatedFlag struct {
	Key     string
	Enabled bool
}

// ClientVersion is one entry of Discovery.min_client_versions.
type ClientVersion struct {
	// Platform is ios, android, macos or windows.
	Platform string
	// MinVersion is the lowest app version that may connect.
	MinVersion string
	// RecommendedVersion is the version below which the app shows an update note; "" for none.
	RecommendedVersion string
}

// Discovery is the public instance metadata (schema Discovery).
type Discovery struct {
	InstanceID      ids.ID
	SetupRequired   bool
	APIVersions     []string
	SpecVersion     string
	Features        []string
	GatewayURL      string
	GatewayVersions []int
	ProductName     string
	AccentColor     string
	// LogoURL is "" while no logo is set.
	LogoURL              string
	Providers            []ProviderInfo
	LocalPasswordEnabled bool
	JWKSURI              string
	WebhookKeysURI       string
	MinClientVersions    []ClientVersion
}

// ProblemType documents one problem code (schema ProblemType).
type ProblemType struct {
	Code        string
	Status      int
	Title       string
	Description string
	// ClientAction is "" when the contract names none.
	ClientAction string
}

// Admin is the incoming port of the six REST operations of M1-46: discovery, the instance view,
// the settings, the branding and the problem types. Each use case authorizes its action of
// docs/spec/05-aktionen.md 4.1 and 4.2 first; owner, step-up and the deny audit come from the
// Authorizer.
type Admin interface {
	// Discovery serves getDiscovery (instance.discovery.read, public, also before setup).
	Discovery(ctx context.Context) (Discovery, error)
	// Info serves getInstance (instance.read, any active principal).
	Info(ctx context.Context, a actor.Actor) (Info, error)
	// ListSettings serves listInstanceSettings (instance.settings.read, owner): every registered
	// key in key order, a default with version 0.
	ListSettings(ctx context.Context, a actor.Actor) ([]Setting, error)
	// PutSetting serves putInstanceSetting (instance.settings.update, owner with step-up).
	// Errors: not_found for an unknown key, validation_failed for a branding key or an invalid
	// value, version_conflict with the current setting on a precondition mismatch.
	PutSetting(ctx context.Context, a actor.Actor, cmd PutSetting) (Setting, error)
	// UpdateBranding serves updateBranding (instance.branding.update, owner with step-up).
	// Errors: validation_failed, version_conflict with the current branding.
	UpdateBranding(ctx context.Context, a actor.Actor, cmd UpdateBranding) (Branding, error)
	// ProblemType serves getProblemType (instance.discovery.read, public). Errors: not_found
	// for an unknown code.
	ProblemType(ctx context.Context, code string) (ProblemType, error)
}
