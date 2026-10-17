// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// FlagKind is the kind of a feature flag (docs/spec/16-operations.md 6.2, ADR 0064).
type FlagKind string

const (
	// FlagKindRelease switches a new function on step by step; the flag carries RemoveBy, and
	// make flag-check fails once the server VERSION reaches it.
	FlagKindRelease FlagKind = "release"
	// FlagKindOption is a lasting product switch of the owner (configuration.md 7).
	FlagKindOption FlagKind = "option"
)

// FlagDefinition is the registration of one feature flag in code. A flag only switches a function
// on or off: it never encodes a number, a limit, a security boundary or a secret (16 6.2
// "Grenze"); no check can see that intent, so review holds it.
type FlagDefinition struct {
	// Key is the flag key, for example dm.enabled.
	Key string
	// Description is the English text the console shows.
	Description string
	// Module is the owning module, a key under modules: of architecture/modules.yaml.
	Module string
	// RemoveBy is the server version (SemVer without v) by which a release flag is removed; ""
	// for an option.
	RemoveBy string
	// Kind is release or option.
	Kind FlagKind
	// Default is the value of the flag while it has no row in feature_flags.
	Default bool
}

// The registered flag keys (configuration.md 7).
const (
	// FlagDMEnabled switches direct messages between people (V3 checks it).
	FlagDMEnabled = "dm.enabled"
	// FlagVoiceEnabled switches the transcription of voice messages (M3 checks it).
	FlagVoiceEnabled = "voice.enabled"
	// FlagTokensPersonalEnabled switches personal API tokens (createToken of identity, M1-44).
	FlagTokensPersonalEnabled = "tokens.personal_enabled"
)

// registry is the one registry of feature flags, sorted by key. Add a flag here, in
// docs/operations/configuration.md 7 for an option, and check it where its function runs;
// make flag-check holds the rules (internal/instance/README.md "Feature flags").
var registry = []FlagDefinition{
	{Key: FlagDMEnabled, Kind: FlagKindOption, Default: false, Module: "chat", Description: "Direct messages between people"},
	{Key: FlagTokensPersonalEnabled, Kind: FlagKindOption, Default: false, Module: "identity", Description: "Personal API tokens"},
	{Key: FlagVoiceEnabled, Kind: FlagKindOption, Default: false, Module: "files", Description: "Transcription of voice messages"},
}

// RegisteredFlags returns a copy of the registry, sorted by key.
func RegisteredFlags() []FlagDefinition {
	out := slices.Clone(registry)
	slices.SortFunc(out, func(a, b FlagDefinition) int { return strings.Compare(a.Key, b.Key) })
	return out
}

// FlagSubject is the principal a flag is evaluated for.
type FlagSubject struct {
	// PrincipalID is the principal.
	PrincipalID ids.ID
	// Kind is user, agent or app; only a user can be in a rollout.
	Kind string
}

// SubjectOf returns the subject of actor a.
func SubjectOf(a actor.Actor) FlagSubject {
	return FlagSubject{PrincipalID: a.ID, Kind: string(a.Kind)}
}

// EvaluatedFlag is one feature flag evaluated for a caller (schema EvaluatedFlag).
type EvaluatedFlag struct {
	// Key is the flag key.
	Key string
	// Enabled is the value for the caller.
	Enabled bool
}

// Flags is the read port of the feature flags for other modules (02 4.3). The server evaluates,
// never a client: value = global value, inverted when the subject (a person) or one of its valid
// groups is in the rollout; an unknown key is off. Errors are returned, never turned into off, so
// a database fault shows as 5xx instead of silently disabling a feature.
type Flags interface {
	// Enabled evaluates key for s. Errors: the database error.
	//
	// Example:
	//
	//	on, err := flags.Enabled(ctx, instance.SubjectOf(a), instance.FlagTokensPersonalEnabled)
	Enabled(ctx context.Context, s FlagSubject, key string) (bool, error)
	// EvaluateAll evaluates every registered flag for s, sorted by key; orphaned rows never
	// appear. Errors: the database error.
	EvaluateAll(ctx context.Context, s FlagSubject) ([]EvaluatedFlag, error)
}

// FlagGroupSource tells the evaluation which groups a person belongs to. Declared here because
// instance may not import authz (02 4.4); authz implements it in authz/adapters/flagsource.
type FlagGroupSource interface {
	// ValidGroupIDs returns the groups that are not deleted and in which userID has a membership
	// in effect at at (expires_at = at is already invalid, 07 2.5). Errors: the database error.
	ValidGroupIDs(ctx context.Context, userID ids.ID, at time.Time) ([]ids.ID, error)
	// ExistingGroupIDs returns the subset of groupIDs that exist and are not deleted. Errors: the
	// database error.
	ExistingGroupIDs(ctx context.Context, groupIDs []ids.ID) ([]ids.ID, error)
}

// FlagUserSource tells putFeatureFlag which IDs of a rollout are people. Declared here because
// instance may not import identity (02 4.4); identity implements it in
// identity/adapters/flagsource.
type FlagUserSource interface {
	// ExistingUserIDs returns the subset of principalIDs that are principals of kind user with a
	// status other than deleted. Errors: the database error.
	ExistingUserIDs(ctx context.Context, principalIDs []ids.ID) ([]ids.ID, error)
}

// FlagRollout is the rollout of a flag in the API: people and groups for which the flag has the
// opposite of its global value; sorted and without duplicates.
type FlagRollout struct {
	// UserIDs are people.
	UserIDs []ids.ID
	// GroupIDs are groups.
	GroupIDs []ids.ID
}

// FeatureFlag is one flag as listFeatureFlags and putFeatureFlag return it (schema FeatureFlag):
// the registration plus the stored value, or for an orphaned row only the stored value.
type FeatureFlag struct {
	// Key is the flag key.
	Key string
	// Kind, Description, Default, Module and RemoveBy come from the registration; empty for an
	// orphaned row.
	Kind        FlagKind
	Description string
	Default     bool
	Module      string
	RemoveBy    string
	// Orphaned is true for a stored row whose key is not registered; it is off.
	Orphaned bool
	// Enabled is the global value: the stored value or the default.
	Enabled bool
	// Rollout is the stored rollout; empty without a row.
	Rollout FlagRollout
	// Version is the row version, 0 without a row.
	Version int64
	// UpdatedBy is the principal of the last write; zero without a row.
	UpdatedBy ids.ID
	// UpdatedAt is the time of the last write; zero without a row.
	UpdatedAt time.Time
}

// PutFlagCmd is the command of putFeatureFlag: PUT replaces the value and the rollout (an absent
// rollout means empty).
type PutFlagCmd struct {
	// Key is the path parameter key.
	Key string
	// Enabled is the global value.
	Enabled bool
	// Rollout is the new rollout as sent, before normalization.
	Rollout FlagRollout
	// Reason is the reason of the change, written to the audit log.
	Reason string
	// IfMatch is the optional precondition.
	IfMatch IfMatch
}

// FlagAdmin is the incoming port of listFeatureFlags and putFeatureFlag (M1-47). Both actions are
// owner only; the update needs step-up and is audited on allow and deny (05 4.2).
type FlagAdmin interface {
	// ListFlags serves listFeatureFlags (feature_flag.read): every registered flag, then every
	// orphaned row, sorted by key.
	ListFlags(ctx context.Context, a actor.Actor) ([]FeatureFlag, error)
	// PutFlag serves putFeatureFlag (feature_flag.update). Errors: not_found for a key that is not
	// registered, validation_failed for a blank reason or an unknown person or group in the
	// rollout, version_conflict with the current flag on a precondition mismatch.
	PutFlag(ctx context.Context, a actor.Actor, cmd PutFlagCmd) (FeatureFlag, error)
}
